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
    Contents []byte /* Generated file contents as byte slice */
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
    Request  *Request     /* Stored filesystem request */
    Contents []byte       /* File contents as byte slice */
}

func (fc *RegularFileContents) Render(responder *Responder) *GophorError {
    return responder.WriteFlush(fc.Contents)
}

func (fc *RegularFileContents) Load() *GophorError {
    /* Load the file into memory */
    var gophorErr *GophorError
    fc.Contents, gophorErr = bufferedRead(fc.Request.AbsPath())
    return gophorErr
}

func (fc *RegularFileContents) Clear() {
    fc.Contents = nil
}

type GophermapContents struct {
    Request  *Request /* Stored filesystem request */
    Sections []GophermapSection /* Slice to hold differing gophermap sections */
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
    gc.Sections, gophorErr = readGophermap(gc.Request)
    return gophorErr
}

func (gc *GophermapContents) Clear() {
    gc.Sections = nil
}

type GophermapSection interface {
    /* Interface for storing differring types of gophermap
     * sections and render when necessary
     */

    Render(*Responder) *GophorError
}

type GophermapTextSection struct {
    Contents []byte /* Text contents */
}

func (s *GophermapTextSection) Render(responder *Responder) *GophorError {
    return responder.Write(replaceStrings(string(s.Contents), responder.Host))
}

type GophermapDirectorySection struct {
    Request *Request    /* Stored filesystem request */
    Hidden  map[string]bool /* Hidden files map parsed from gophermap */
}

func (g *GophermapDirectorySection) Render(responder *Responder) *GophorError {
    /* Create new filesystem request from mixture of stored + supplied */
    return listDir(
        &Responder{
            responder.Host,
            responder.Client,
            responder.Writer,
            g.Request,
        },
        g.Hidden,
    )
}

type GophermapFileSection struct {
    Request *Request
}

func (g *GophermapFileSection) Render(responder *Responder) *GophorError {
    fileContents, gophorErr := readIntoGophermap(g.Request.AbsPath())
    if gophorErr != nil {
        return gophorErr
    }
    return responder.Write(fileContents)
}

type GophermapSubmapSection struct {
    Request *Request
}

func (g *GophermapSubmapSection) Render(responder *Responder) *GophorError {
    /* Load the gophermap into memory as gophermap sections */
    sections, gophorErr := readGophermap(g.Request)
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
    Request *Request /* Stored file system request */
}

func (g *GophermapExecCgiSection) Render(responder *Responder) *GophorError {
    /* Create new filesystem request from mixture of stored + supplied */
    return executeCgi(&Responder{
        responder.Host,
        responder.Client,
        responder.Writer,
        g.Request,
    })
}

type GophermapExecFileSection struct {
    Request *Request /* Stored file system request */
}

func (g *GophermapExecFileSection) Render(responder *Responder) *GophorError {
    return executeFile(&Responder{
        responder.Host,
        responder.Client,
        responder.Writer,
        g.Request,
    })
}

type GophermapExecCommandSection struct {
    Request *Request
}

func (g *GophermapExecCommandSection) Render(responder *Responder) *GophorError {
    return executeCommand(&Responder{
        responder.Host,
        responder.Client,
        responder.Writer,
        g.Request,
    })
}

func readGophermap(request *Request) ([]GophermapSection, *GophorError) {
    /* Create return slice */
    sections := make([]GophermapSection, 0)

    /* _Create_ hidden files map now in case dir listing requested */
    hidden := make(map[string]bool)

    /* Keep track of whether we've already come across a title line (only 1 allowed!) */
    titleAlready := false

    /* Reference directory listing now in case requested */
    var dirListing *GophermapDirectorySection

    /* Perform buffered scan with our supplied splitter and iterators */
    gophorErr := bufferedScan(request.AbsPath(),
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
                    hidden[line[1:]] = true

                case TypeSubGophermap:
                    /* Parse new requestPath and parameters (this automatically sanitizes requestPath) */
                    subRelPath, subParameters := parseLineRequestString(request.Path, line[1:])
                    subRequest := &Request{ subRelPath, subParameters }

                    if !subRequest.PathHasAbsPrefix("/") {
                        if Config.CgiEnabled {
                            /* Special case here where command must be in path, return GophermapExecCommand */
                            Config.SysLog.Info("", "Inserting command output")
                            sections = append(sections, &GophermapExecCommandSection{ subRequest })
                        } else {
                            break
                        }
                    } else if subRequest.RelPath() == "" {
                        /* path cleaning failed */
                        break
                    } else if subRequest.RelPath() == request.RelPath() {
                        /* Same as current gophermap. Recursion bad! */
                        break
                    }

                    /* Perform file stat */
                    stat, err := os.Stat(subRequest.AbsPath())
                    if (err != nil) || (stat.Mode() & os.ModeDir != 0) {
                        /* File read error or is directory */
                        break
                    }

                    /* Check if we've been supplied subgophermap or regular file */
                    if subRequest.PathHasAbsSuffix(GophermapFileStr) {
                        /* If executable, store as GophermapExecutable, else readGophermap() */
                        if Config.CgiEnabled && stat.Mode().Perm() & 0100 != 0 {
                            Config.SysLog.Info("", "Inserting executable subgophermap")
                            sections = append(sections, &GophermapExecFileSection { subRequest })
                        } else {
                            /* Treat as any other gophermap! */
                            Config.SysLog.Info("", "Inserting regular subgophermap")
                            sections = append(sections, &GophermapSubmapSection{ subRequest })
                        }
                    } else {
                        /* If stored in cgi-bin store as GophermapExecutable, else read into GophermapText */
                        if Config.CgiEnabled && subRequest.PathHasRelPrefix(CgiBinDirStr) {
                            Config.SysLog.Info("", "Inserting CGI script output")
                            sections = append(sections, &GophermapExecCgiSection{ subRequest })
                        } else {
                            Config.SysLog.Info("", "Inserting regular file")
                            sections = append(sections, &GophermapFileSection{ subRequest })
                        }
                    }

                case TypeEnd:
                    /* Lastline, break out at end of loop. Interface method Contents()
                     * will append a last line at the end so we don't have to worry about
                     * that here, only stopping the loop.
                     */
                    return false

                case TypeEndBeginList:
                    /* Create GophermapDirListing object then break out at end of loop */
                    dirRequest := &Request{ NewRequestPath(request.RootDir(), request.PathTrimRelSuffix(GophermapFileStr)), request.Parameters }
                    dirListing = &GophermapDirectorySection{ dirRequest, hidden }
                    return false

                default:
                    /* Just append to sections slice as gophermap text */
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

func readIntoGophermap(path string) ([]byte, *GophorError) {
    /* Create return slice */
    fileContents := make([]byte, 0)

    /* Perform buffered scan with our supplied splitter and iterators */
    gophorErr := bufferedScan(path,
        func(scanner *bufio.Scanner) bool {
            line := scanner.Text()

            if line == "" {
                fileContents = append(fileContents, buildInfoLine("")...)
                return true
            }

            /* Replace the newline character */
            line = replaceNewLines(line)

            /* Iterate through returned str, reflowing to new line
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

func minWidth(w int) int {
    if w <= Config.PageWidth {
        return w
    } else {
        return Config.PageWidth
    }
}
