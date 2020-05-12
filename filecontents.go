package main

import (
    "bytes"
    "bufio"
    "os"
)

type FileContents interface {
    /* Interface that provides an adaptable implementation
     * for holding onto some level of information about the
     * contents of a file.
     */
    Render(*Responder) *GophorError
    Load()             *GophorError
    Clear()
}

type GeneratedFileContents struct {
    /* Super simple, holds onto a slice of bytes */

    Contents []byte
}

func (fc *GeneratedFileContents) Render(responder *Responder) *GophorError {
    return responder.WriteFlush(fc.Contents)
}

func (fc *GeneratedFileContents) Load() *GophorError {
    /* do nothing */
    return nil
}

func (fc *GeneratedFileContents) Clear() {
    /* do nothing */
}

type RegularFileContents struct {
    /* Simple implemention that holds onto a RequestPath
     * and slice containing cache'd content
     */

    Path     *RequestPath
    Contents []byte
}

func (fc *RegularFileContents) Render(responder *Responder) *GophorError {
    return responder.WriteFlush(fc.Contents)
}

func (fc *RegularFileContents) Load() *GophorError {
    /* Load the file into memory */
    var gophorErr *GophorError
    fc.Contents, gophorErr = bufferedRead(fc.Path.Absolute())
    return gophorErr
}

func (fc *RegularFileContents) Clear() {
    fc.Contents = nil
}

type GophermapContents struct {
    /* Holds onto a RequestPath and slice containing individually
     * renderable sections of the gophermap.
     */

    Path     *RequestPath
    Sections []GophermapSection
}

func (gc *GophermapContents) Render(responder *Responder) *GophorError {
    /* Render and send each of the gophermap sections */
    var gophorErr *GophorError
    for _, line := range gc.Sections {
        gophorErr = line.Render(responder)
        if gophorErr != nil {
            Config.SysLog.Error("", "Error executing gophermap contents: %s\n", gophorErr.Error())
        }
    }

    /* End on footer text (including lastline) */
    return responder.WriteFlush(Config.FooterText)
}

func (gc *GophermapContents) Load() *GophorError {
    /* Load the gophermap into memory as gophermap sections */
    var gophorErr *GophorError
    gc.Sections, gophorErr = readGophermap(gc.Path)
    return gophorErr
}

func (gc *GophermapContents) Clear() {
    gc.Sections = nil
}

type GophermapSection interface {
    /* Interface for storing differring types of gophermap
     * sections to render when necessary
     */

    Render(*Responder) *GophorError
}

type GophermapTextSection struct {
    Contents []byte
}

func (s *GophermapTextSection) Render(responder *Responder) *GophorError {
    return responder.Write(replaceStrings(string(s.Contents), responder.Host))
}

type GophermapDirectorySection struct {
    /* Holds onto a directory path, and a list of files
     * to hide from the client when rendering.
     */

    Path    *RequestPath
    Hidden  map[string]bool
}

func (g *GophermapDirectorySection) Render(responder *Responder) *GophorError {
    /* Create new responder from supplied and using stored path */
    return listDir(responder.CloneWithRequest(&Request{ g.Path, nil }), g.Hidden)
}

type GophermapFileSection struct {
    /* Holds onto a file path to be read and rendered when requested */
    Path *RequestPath
}

func (g *GophermapFileSection) Render(responder *Responder) *GophorError {
    fileContents, gophorErr := readIntoGophermap(g.Path.Absolute())
    if gophorErr != nil {
        return gophorErr
    }
    return responder.Write(fileContents)
}

type GophermapSubmapSection struct {
    /* Holds onto a gophermap path to be read and rendered when requested */
    Path *RequestPath
}

func (g *GophermapSubmapSection) Render(responder *Responder) *GophorError {
    /* Load the gophermap into memory as gophermap sections */
    sections, gophorErr := readGophermap(g.Path)
    if gophorErr != nil {
        return gophorErr
    }

    /* Render and send each of the gophermap sections */
    for _, line := range sections {
        gophorErr = line.Render(responder)
        if gophorErr != nil {
            Config.SysLog.Error("", "Error executing gophermap contents: %s\n", gophorErr.Error())
        }
    }

    return nil
}

type GophermapExecCgiSection struct {
    /* Holds onto a request with CGI script path and supplied parameters */
    Request *Request
}

func (g *GophermapExecCgiSection) Render(responder *Responder) *GophorError {
    /* Create new filesystem request from mixture of stored + supplied */
    return executeCgi(responder.CloneWithRequest(g.Request))
}

type GophermapExecFileSection struct {
    /* Holds onto a request with executable file path and supplied arguments */
    Request *Request
}

func (g *GophermapExecFileSection) Render(responder *Responder) *GophorError {
    /* Create new responder from supplied and using stored path */
    return executeFile(responder.CloneWithRequest(g.Request))
}

/* Read and parse a gophermap into separately cacheable and renderable GophermapSection */
func readGophermap(path *RequestPath) ([]GophermapSection, *GophorError) {
    /* Create return slice */
    sections := make([]GophermapSection, 0)

    /* Create hidden files map now in case dir listing requested */
    hidden := map[string]bool{
        path.Relative(): true, /* Ignore current gophermap */
        CgiBinDirStr:    true, /* Ignore cgi-bin if found */
    }

    /* Keep track of whether we've already come across a title line (only 1 allowed!) */
    titleAlready := false

    /* Perform buffered scan with our supplied splitter and iterators */
    gophorErr := bufferedScan(path.Absolute(),
        func(scanner *bufio.Scanner) bool {
            line := scanner.Text()

            /* Parse the line item type and handle */
            lineType := parseLineType(line)
            switch lineType {
                case TypeInfoNotStated:
                    /* Append TypeInfo to the beginning of line */
                    sections = append(sections, &GophermapTextSection{ buildInfoLine(line) })

                case TypeTitle:
                    /* Reformat title line to send as info line with appropriate selector */
                    if !titleAlready {
                        sections = append(sections, &GophermapTextSection{ buildLine(TypeInfo, line[1:], "TITLE", NullHost, NullPort) })
                        titleAlready = true
                    }

                case TypeComment:
                    /* We ignore this line */
                    break

                case TypeHiddenFile:
                    /* Add to hidden files map */
                    hidden[path.JoinRel(line[1:])] = true

                case TypeSubGophermap:
                    /* Parse new RequestPath and parameters */
                    subPath, parameters := parseLineRequestString(path, line[1:])
                    if subPath.Relative() == "" || subPath.Relative() == path.Relative() {
                        /* Either path parsing failed, or we've been supplied same gophermap, and recursion is
                         * recursion is recursion is bad kids!
                         */
                        break
                    }

                    /* Perform file stat */
                    stat, err := os.Stat(subPath.Absolute())
                    if (err != nil) || (stat.Mode() & os.ModeDir != 0) {
                        /* File read error or is directory */
                        break
                    }

                    /* Check if we've been supplied subgophermap or regular file */
                    if subPath.HasAbsSuffix(GophermapFileStr) {
                        /* If executable, store as GophermapExecFileSection, else GophermapSubmapSection */
                        if stat.Mode().Perm() & 0100 != 0 {
                            sections = append(sections, &GophermapExecFileSection { &Request{ subPath, parameters } })
                        } else {
                            sections = append(sections, &GophermapSubmapSection{ subPath })
                        }
                    } else {
                        /* If stored in cgi-bin store as GophermapExecCgiSection, else GophermapFileSection */
                        if subPath.HasRelPrefix(CgiBinDirStr) {
                            sections = append(sections, &GophermapExecCgiSection{ &Request{ subPath, parameters} })
                        } else {
                            sections = append(sections, &GophermapFileSection{ subPath })
                        }
                    }

                case TypeEnd:
                    /* Lastline, break out at end of loop. GophermapContents.Render() will
                     * append a LastLine string so we don't have to worry about that here.
                     */
                    return false

                case TypeEndBeginList:
                    /* Append GophermapDirectorySection object then break, as with TypeEnd. */
                    dirRequest := NewRequestPath(path.RootDir(), path.TrimRelSuffix(GophermapFileStr))
                    sections = append(sections, &GophermapDirectorySection{ dirRequest, hidden })
                    return false

                default:
                    /* Default is appending to sections slice as GopherMapTextSection */
                    sections = append(sections, &GophermapTextSection{ []byte(line+DOSLineEnd) })
            }

            return true
        },
    )

    /* Check the bufferedScan didn't exit with error */
    if gophorErr != nil {
        return nil, gophorErr
    }

    return sections, nil
}

/* Read a text file into a gophermap as text sections */
func readIntoGophermap(path string) ([]byte, *GophorError) {
    /* Create return slice */
    fileContents := make([]byte, 0)

    /* Perform buffered scan with our supplied iterator */
    gophorErr := bufferedScan(path,
        func(scanner *bufio.Scanner) bool {
            line := scanner.Text()

            if line == "" {
                fileContents = append(fileContents, buildInfoLine("")...)
                return true
            }

            /* Replace the newline characters */
            line = replaceNewLines(line)

            /* Iterate through line string, reflowing to new line
             * until all lines < PageWidth
             */
            for len(line) > 0 {
                length := minWidth(len(line))
                fileContents = append(fileContents, buildInfoLine(line[:length])...)
                line = line[length:]
            }

            return true
        },
    )

    /* Check the bufferedScan didn't exit with error */
    if gophorErr != nil {
        return nil, gophorErr
    }

    /* Check final output ends on a newline */
    if !bytes.HasSuffix(fileContents, []byte(DOSLineEnd)) {
        fileContents = append(fileContents, []byte(DOSLineEnd)...)
    }

    return fileContents, nil
}

/* Return minimum width out of PageWidth and W */
func minWidth(w int) int {
    if w <= Config.PageWidth {
        return w
    } else {
        return Config.PageWidth
    }
}
