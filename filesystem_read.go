package main

import (
    "os"
    "bytes"
    "io"
    "sort"
    "bufio"
)

const (
    FileReadBufSize = 1024
)

/* Perform simple buffered read on a file at path */
func bufferedRead(path string) ([]byte, *GophorError) {
    /* Open file */
    fd, err := os.Open(path)
    if err != nil {
        return nil, &GophorError{ FileOpenErr, err }
    }
    defer fd.Close()

    /* Setup buffers */
    var count int
    contents := make([]byte, 0)
    buf := make([]byte, FileReadBufSize)

    /* Setup reader */
    reader := bufio.NewReader(fd)

    /* Read through buffer until error or null bytes! */
    for {
        count, err = reader.Read(buf)
        if err != nil {
            if err == io.EOF {
                break
            }

            return nil, &GophorError{ FileReadErr, err }
        }

        contents = append(contents, buf[:count]...)

        if count < FileReadBufSize {
            break
        }
    }

    return contents, nil
}

/* Perform buffered read on file at path, then scan through with supplied iterator func */
func bufferedScan(path string, scanIterator func(*bufio.Scanner) bool) *GophorError {
    /* First, read raw file contents */
    contents, gophorErr := bufferedRead(path)
    if gophorErr != nil {
        return gophorErr
    }

    /* Create reader and scanner from this */
    reader := bytes.NewReader(contents)
    scanner := bufio.NewScanner(reader)

    /* If contains DOS line-endings, split by DOS! Else, split by Unix */
    if bytes.Contains(contents, []byte(DOSLineEnd)) {
        scanner.Split(dosLineEndSplitter)
    } else {
        scanner.Split(unixLineEndSplitter)
    }

    /* Scan through file contents using supplied iterator */
    for scanner.Scan() && scanIterator(scanner) {}

    /* Check scanner finished cleanly */
    if scanner.Err() != nil {
        return &GophorError{ FileReadErr, scanner.Err() }
    }

    return nil
}

func dosLineEndSplitter(data []byte, atEOF bool) (advance int, token []byte, err error) {
    if atEOF && len(data) == 0  {
        /* At EOF, no more data */
        return 0, nil, nil
    }

    if i := bytes.Index(data, []byte("\r\n")); i >= 0 {
        /* We have a full new-line terminate line */
        return i+2, data[:i], nil
    }

    /* Request more data */
    return 0, nil, nil
}

func unixLineEndSplitter(data []byte, atEOF bool) (advance int, token []byte, err error) {
    if atEOF && len(data) == 0  {
        /* At EOF, no more data */
        return 0, nil, nil
    }

    if i := bytes.Index(data, []byte("\n")); i >= 0 {
        /* We have a full new-line terminate line */
        return i+1, data[:i], nil
    }

    /* Request more data */
    return 0, nil, nil
}

/* List the files in a directory, hiding those requested */
func listDir(responder *Responder, hidden map[string]bool, includeTitleFooter bool) *GophorError {
    /* Open directory file descriptor */
    fd, err := os.Open(responder.Request.AbsPath())
    if err != nil {
        Config.SysLog.Error("", "failed to open %s: %s\n", responder.Request.AbsPath(), err.Error())
        return &GophorError{ FileOpenErr, err }
    }

    /* Read files in directory */
    files, err := fd.Readdir(-1)
    if err != nil {
        Config.SysLog.Error("", "failed to enumerate dir %s: %s\n", responder.Request.AbsPath(), err.Error())
        return &GophorError{ DirListErr, err }
    }
    
    /* Sort the files by name */
    sort.Sort(byName(files))

    /* Create directory content slice, ready */
    dirContents := make([]byte, 0)

    /* First add a title + a space */
    if includeTitleFooter {
        dirContents = append(dirContents, buildLine(TypeInfo, "[ "+responder.Host.Name()+responder.Request.SelectorPath()+" ]", "TITLE", NullHost, NullPort)...)
        dirContents = append(dirContents, buildInfoLine("")...)

        /* Add a 'back' entry. GoLang Readdir() seems to miss this */
        dirContents = append(dirContents, buildLine(TypeDirectory, "..", responder.Request.PathJoinSelector(".."), responder.Host.Name(), responder.Host.Port())...)
    }

    /* Walk through files :D */
    var reqPath *RequestPath
    for _, file := range files {
        reqPath = NewRequestPath(responder.Request.RootDir(), file.Name())

        /* If hidden file, or restricted file, continue! */
        if isHiddenFile(hidden, reqPath.Relative()) || isRestrictedFile(reqPath.Relative()) {
            continue
        }

        /* If requires remap, do so! */
        Config.FileSystem.ReverseRemapRequestPath(reqPath)

        /* Handle file, directory or ignore others */
        switch {
            case file.Mode() & os.ModeDir != 0:
                /* Directory -- create directory listing */
                dirContents = append(dirContents, buildLine(TypeDirectory, file.Name(), reqPath.Selector(), responder.Host.Name(), responder.Host.Port())...)

            case file.Mode() & os.ModeType == 0:
                /* Regular file -- find item type and creating listing */
                itemPath := reqPath.Selector()
                itemType := getItemType(itemPath)
                dirContents = append(dirContents, buildLine(itemType, file.Name(), reqPath.Selector(), responder.Host.Name(), responder.Host.Port())...)

            default:
                /* Ignore */
        }
    }

    if includeTitleFooter {
        dirContents = append(dirContents, Config.FooterText...)
    }

    /* Append the footer (including lastline), write and flush! */
    return responder.WriteFlush(dirContents)
}

func isHiddenFile(hiddenMap map[string]bool, fileName string) bool {
    _, ok := hiddenMap[fileName]
    return ok
}

/* Took a leaf out of go-gopher's book here. */
type byName []os.FileInfo
func (s byName) Len() int           { return len(s) }
func (s byName) Less(i, j int) bool { return s[i].Name() < s[j].Name() }
func (s byName) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
