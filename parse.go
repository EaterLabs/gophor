package main

import (
    "strings"
)

/* Parse a request string into a path and parameters string */
func parseRequestString(request string) (string, []string) {
    /* Read up to first '?' and then put rest into single slice string array */
    i := 0
    for i < len(request) {
        if request[i] == '?' {
            break
        }
        i += 1
    }

    /* Use strings.TrimPrefix() as it returns empty string for zero length string */
    return request[:i], []string{ request[i:] }
}

/* Parse line type from contents */
func parseLineType(line string) ItemType {
    lineLen := len(line)

    if lineLen == 0 {
        return TypeInfoNotStated
    } else if lineLen == 1 {
        /* The only accepted types for a length 1 line */
        switch ItemType(line[0]) {
            case TypeEnd:
                return TypeEnd
            case TypeEndBeginList:
                return TypeEndBeginList
            case TypeComment:
                return TypeComment
            case TypeInfo:
                return TypeInfo
            case TypeTitle:
                return TypeTitle
            default:
                return TypeUnknown
        }
    } else if !strings.Contains(line, string(Tab)) {
        /* The only accepted types for a line with no tabs */
        switch ItemType(line[0]) {
            case TypeComment:
                return TypeComment
            case TypeTitle:
                return TypeTitle
            case TypeInfo:
                return TypeInfo
            case TypeHiddenFile:
                return TypeHiddenFile
            case TypeSubGophermap:
                return TypeSubGophermap
            default:
                return TypeInfoNotStated
        }
    }

    return ItemType(line[0])
}

/* Parses a line in a gophermap into a filesystem request path and a string slice of arguments */
func parseLineRequestString(requestPath *RequestPath, lineStr string) (*RequestPath, []string) {
    if strings.HasPrefix(lineStr, "/") {
        /* Assume is absolute (well, seeing server root as '/') */
        if strings.HasPrefix(lineStr[1:], CgiBinDirStr) {
            /* CGI script, parse request path and parameters */
            relPath, parameters := parseRequestString(lineStr)
            return NewRequestPath(requestPath.RootDir(), relPath), parameters
        } else {
            /* Regular file, no more parsing */
            return NewRequestPath(requestPath.RootDir(), lineStr), []string{}
        }
    } else {
        /* Assume relative to current directory */
        if strings.HasPrefix(lineStr, CgiBinDirStr) && requestPath.Relative() == "" {
            /* If begins with cgi-bin and is at root dir, parse as cgi-bin */
            relPath, parameters := parseRequestString(lineStr)
            return NewRequestPath(requestPath.RootDir(), relPath), parameters
        } else {
            /* Regular file, no more parsing */
            return NewRequestPath(requestPath.RootDir(), requestPath.JoinCurDir(lineStr)), []string{}
        }
    }
}

/* Split a string according to a rune, that supports delimiting with '\' */
func splitStringByRune(str string, r rune) []string {
    ret := make([]string, 0)
    buf := ""
    delim := false
    for _, c := range str {
        switch c {
            case r:
                if !delim {
                    ret = append(ret, buf)
                    buf = ""
                } else {
                    buf += string(c)
                    delim = false
                }

            case '\\':
                if !delim {
                    delim = true
                } else {
                    buf += string(c)
                    delim = false
                }

            default:
                if !delim {
                    buf += string(c)
                } else {
                    buf += "\\"+string(c)
                    delim = false
                }
        }
    }

    if len(buf) > 0 || len(ret) == 0 {
        ret = append(ret, buf)
    }

    return ret
}
