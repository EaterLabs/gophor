package main

import (
    "strings"
    "net/url"
)

/* Parse a request string into a path and parameters string */
func parseGopherUrl(request string) (*GopherUrl, *GophorError) {
    if strings.Contains(request, "#") ||     // we don't support fragments
       strings.HasPrefix(request, "GET ") {  // we don't support HTTP requests
        return nil, &GophorError{ InvalidRequestErr, nil }
    }

    /* Check if string contains any ASCII control byte */
    for i := 0; i < len(request); i += 1 {
        if request[i] < ' ' || request[i] == 0x7f {
            return nil, &GophorError{ InvalidRequestErr, nil }
        }
    }

    /* Split into 2 substrings by '?'. Url path and query */
    split := strings.SplitN(request, "?", 2)

    /* Unescape path */
    path, err := url.PathUnescape(split[0])
    if err != nil {
        return nil, &GophorError{ InvalidRequestErr, nil }
    }

    /* Return GopherUrl based on this split request */
    if len(split) == 1 {
        return &GopherUrl{ path, "" }, nil
    } else {
        return &GopherUrl{ path, split[1] }, nil
    }
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

/* Parses a line in a gophermap into a new request object. TODO: improve this */
func parseLineRequestString(requestPath *RequestPath, lineStr string) (*Request, *GophorError) {
    if strings.HasPrefix(lineStr, "/") {
        /* Assume is absolute (well, seeing server root as '/') */
        if withinCgiBin(lineStr[1:]) {
            /* CGI script, parse request path and parameters */
            url, gophorErr := parseGopherUrl(lineStr[1:])
            if gophorErr != nil {
                return nil, gophorErr
            } else {
                return &Request{ NewRequestPath(requestPath.RootDir(), url.Path), url.Parameters }, nil
            }
        } else {
            /* Regular file, no more parsing */
            return &Request{ NewRequestPath(requestPath.RootDir(), lineStr[1:]), "" }, nil
        }
    } else {
        /* Assume relative to current directory */
        if withinCgiBin(lineStr) && requestPath.Relative() == "" {
            /* If begins with cgi-bin and is at root dir, parse as cgi-bin */
            url, gophorErr := parseGopherUrl(lineStr)
            if gophorErr != nil {
                return nil, gophorErr
            } else {
                return &Request{ NewRequestPath(requestPath.RootDir(), url.Path), url.Parameters }, nil
            }
        } else {
            /* Regular file, no more parsing */
            return &Request{ NewRequestPath(requestPath.RootDir(), requestPath.JoinCurDir(lineStr)), "" }, nil
        }
    }
}
