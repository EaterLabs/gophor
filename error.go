package main

import (
    "fmt"
)

/* Simple error code type defs */
type ErrorCode int
type ErrorResponseCode int
const (
    /* Filesystem */
    PathEnumerationErr    ErrorCode = iota
    IllegalPathErr        ErrorCode = iota
    FileStatErr           ErrorCode = iota
    FileOpenErr           ErrorCode = iota
    FileReadErr           ErrorCode = iota
    FileTypeErr           ErrorCode = iota
    DirListErr            ErrorCode = iota

    /* Sockets */
    BufferedWriteErr      ErrorCode = iota
    BufferedWriteReadErr  ErrorCode = iota
    BufferedWriteFlushErr ErrorCode = iota
    
    /* Parsing */
    InvalidRequestErr     ErrorCode = iota
    EmptyItemTypeErr      ErrorCode = iota
    InvalidGophermapErr   ErrorCode = iota

    /* Executing */
    CommandStartErr       ErrorCode = iota
    CommandExitCodeErr    ErrorCode = iota
    CgiOutputErr          ErrorCode = iota
    CgiDisabledErr        ErrorCode = iota
    RestrictedCommandErr  ErrorCode = iota

    /* Wrapping CGI http status codes */
    CgiStatus400Err       ErrorCode = iota
    CgiStatus401Err       ErrorCode = iota
    CgiStatus403Err       ErrorCode = iota
    CgiStatus404Err       ErrorCode = iota
    CgiStatus408Err       ErrorCode = iota
    CgiStatus410Err       ErrorCode = iota
    CgiStatus500Err       ErrorCode = iota
    CgiStatus501Err       ErrorCode = iota
    CgiStatus503Err       ErrorCode = iota
    CgiStatusUnknownErr   ErrorCode = iota

    /* Error Response Codes */
    ErrorResponse200 ErrorResponseCode = iota
    ErrorResponse400 ErrorResponseCode = iota
    ErrorResponse401 ErrorResponseCode = iota
    ErrorResponse403 ErrorResponseCode = iota
    ErrorResponse404 ErrorResponseCode = iota
    ErrorResponse408 ErrorResponseCode = iota
    ErrorResponse410 ErrorResponseCode = iota
    ErrorResponse500 ErrorResponseCode = iota
    ErrorResponse501 ErrorResponseCode = iota
    ErrorResponse503 ErrorResponseCode = iota
    NoResponse       ErrorResponseCode = iota
)

/* Simple GophorError data structure to wrap another error */
type GophorError struct {
    Code ErrorCode
    Err  error
}

/* Convert error code to string */
func (e *GophorError) Error() string {
    var str string
    switch e.Code {
        case PathEnumerationErr:
            str = "path enumeration fail"
        case IllegalPathErr:
            str = "illegal path requested"
        case FileStatErr:
            str = "file stat fail"
        case FileOpenErr:
            str = "file open fail"
        case FileReadErr:
            str = "file read fail"
        case FileTypeErr:
            str = "invalid file type"
        case DirListErr:
            str = "directory read fail"

        case BufferedWriteErr:
            str = "buffered write error"
        case BufferedWriteReadErr:
            str = "buffered write readFrom error"
        case BufferedWriteFlushErr:
            str = "buffered write flush error"

        case InvalidRequestErr:
            str = "invalid request data"
        case InvalidGophermapErr:
            str = "invalid gophermap"

        case CommandStartErr:
            str = "command start fail"
        case CgiOutputErr:
            str = "cgi output format error"
        case CommandExitCodeErr:
            str = "command exit code non-zero"
        case CgiDisabledErr:
            str = "ignoring /cgi-bin request, CGI disabled"
        case RestrictedCommandErr:
            str = "command use restricted"

        case CgiStatus400Err:
            str = "CGI script error status 400"
        case CgiStatus401Err:
            str = "CGI script error status 401"
        case CgiStatus403Err:
            str = "CGI script error status 403"
        case CgiStatus404Err:
            str = "CGI script error status 404"
        case CgiStatus408Err:
            str = "CGI script error status 408"
        case CgiStatus410Err:
            str = "CGI script error status 410"
        case CgiStatus500Err:
            str = "CGI script error status 500"
        case CgiStatus501Err:
            str = "CGI script error status 501"
        case CgiStatus503Err:
            str = "CGI script error status 503"
        case CgiStatusUnknownErr:
            str = "CGI script error unknown status code"

        default:
            str = "Unknown"
    }

    if e.Err != nil {
        return fmt.Sprintf("%s (%s)", str, e.Err.Error())
    } else {
        return fmt.Sprintf("%s", str)
    }
}

/* Convert a gophor error code to appropriate error response code */
func gophorErrorToResponseCode(code ErrorCode) ErrorResponseCode {
    switch code {
        case PathEnumerationErr:
            return ErrorResponse400
        case IllegalPathErr:
            return ErrorResponse403
        case FileStatErr:
            return ErrorResponse404
        case FileOpenErr:
            return ErrorResponse404
        case FileReadErr:
            return ErrorResponse404
        case FileTypeErr:
            /* If wrong file type, just assume file not there */
            return ErrorResponse404
        case DirListErr:
            return ErrorResponse404

        /* These are errors _while_ sending, no point trying to send error  */
        case BufferedWriteErr:
            return NoResponse
        case BufferedWriteReadErr:
            return NoResponse
        case BufferedWriteFlushErr:
            return NoResponse

        case InvalidRequestErr:
            return ErrorResponse400
        case InvalidGophermapErr:
            return ErrorResponse500

        case CommandStartErr:
            return ErrorResponse500
        case CommandExitCodeErr:
            return ErrorResponse500
        case CgiOutputErr:
            return ErrorResponse500
        case CgiDisabledErr:
            return ErrorResponse404
        case RestrictedCommandErr:
            return ErrorResponse500

        case CgiStatus400Err:
            return ErrorResponse400
        case CgiStatus401Err:
            return ErrorResponse401
        case CgiStatus403Err:
            return ErrorResponse403
        case CgiStatus404Err:
            return ErrorResponse404
        case CgiStatus408Err:
            return ErrorResponse408
        case CgiStatus410Err:
            return ErrorResponse410
        case CgiStatus500Err:
            return ErrorResponse500
        case CgiStatus501Err:
            return ErrorResponse501
        case CgiStatus503Err:
            return ErrorResponse503
        case CgiStatusUnknownErr:
            return ErrorResponse500

        default:
            return ErrorResponse503
    }
}

/* Generates gopher protocol compatible error response from our code */
func generateGopherErrorResponseFromCode(code ErrorCode) []byte {
    responseCode := gophorErrorToResponseCode(code)
    if responseCode == NoResponse {
        return nil
    }
    return generateGopherErrorResponse(responseCode)
}

/* Generates gopher protocol compatible error response for response code */
func generateGopherErrorResponse(code ErrorResponseCode) []byte {
    return buildErrorLine(code.String())
}

/* Error response code to string */
func (e ErrorResponseCode) String() string {
    switch e {
        case ErrorResponse200:
            /* Should not have reached here */
            Config.SysLog.Fatal("", "Passed error response 200 to error handler, SHOULD NOT HAVE DONE THIS\n")
            return ""
        case ErrorResponse400:
            return "400 Bad Request"
        case ErrorResponse401:
            return "401 Unauthorised"
        case ErrorResponse403:
            return "403 Forbidden"
        case ErrorResponse404:
            return "404 Not Found"
        case ErrorResponse408:
            return "408 Request Time-out"
        case ErrorResponse410:
            return "410 Gone"
        case ErrorResponse500:
            return "500 Internal Server Error"
        case ErrorResponse501:
            return "501 Not Implemented"
        case ErrorResponse503:
            return "503 Service Unavailable"
        default:
            /* Should not have reached here */
            Config.SysLog.Fatal("", "Unhandled ErrorResponseCode type\n")
            return ""
    }
}
