package main

import (
    "io"
    "bufio"
    "bytes"
)

type Responder struct {
    Host    *ConnHost
    Client  *ConnClient
    Writer  *bufio.Writer
    Request *Request
}

func NewSanitizedRequest(conn *GophorConn, requestStr string) *Request {
    relPath, paramaters := parseRequestString(requestStr)
    relPath = sanitizeRelativePath(conn.RootDir(), relPath)
    return &Request{ NewRequestPath(conn.RootDir(), relPath), paramaters }
}

func NewResponder(conn *GophorConn, request *Request) *Responder {
    bufWriter := bufio.NewWriterSize(conn.Conn, Config.SocketWriteBufSize)
    return &Responder{ conn.Host, conn.Client, bufWriter, request }
}

func (r *Responder) AccessLogInfo(format string, args ...interface{}) {
    Config.AccLog.Info("("+r.Client.AddrStr()+") ", format, args...)
}

func (r *Responder) AccessLogError(format string, args ...interface{}) {
    Config.AccLog.Error("("+r.Client.AddrStr()+") ", format, args...)
}

func (r *Responder) Write(data []byte) *GophorError {
    /* Try write all supplied data */
    _, err := r.Writer.Write(data)
    if err != nil {
        return &GophorError{ BufferedWriteErr, err }
    }
    return nil
}

func (r *Responder) WriteFlush(data []byte) *GophorError {
    /* Try write all supplied data, followed by flush */
    _, err := r.Writer.Write(data)
    if err != nil {
        return &GophorError{ BufferedWriteErr, err }
    }
    return r.Flush()
}

func (r *Responder) Flush() *GophorError {
    err := r.Writer.Flush()
    if err != nil {
        return &GophorError{ BufferedWriteFlushErr, err }
    }
    return nil
}

func (r *Responder) SafeFlush(gophorErr *GophorError) *GophorError {
    /* Flush only if supplied error is nil */
    if gophorErr != nil {
        return gophorErr
    } else {
        return r.Flush()
    }
}

func (r *Responder) WriteRaw(reader io.Reader) *GophorError {
    /* Write directly from reader to bufio writer */
    _, err := r.Writer.ReadFrom(reader)
    if err != nil {
        return &GophorError{ BufferedWriteReadErr, err }
    }
    return r.Flush()
}

func (r *Responder) CloneWithRequest(request *Request) *Responder {
    /* Create new copy of Responder only with request differring */
    return &Responder{
        r.Host,
        r.Client,
        r.Writer,
        request,
    }
}

type HttpStripWriter struct {
    /* Wrapper to bufio writer that allows read up to
     * some predefined prefix into a buffer, then continuing
     * write to expected writer destination either after prefix
     * reached, or skip buffer filled (whichever comes first).
     */

    Writer     *bufio.Writer

    /* This allows us to specify the write function so that after
     * having performed the skip we can modify the write function used
     * and not have to use an if-case EVERY SINGLE TIME.
     */
    WriteFunc  func([]byte) (int, error)

    SkipBuffer []byte
    SkipIndex  int
}

func NewHttpStripWriter(writer *bufio.Writer) *HttpStripWriter {
    w := &HttpStripWriter{}
    w.Writer = writer
    w.WriteFunc = w.WriteCheckForHeaders
    w.SkipBuffer = make([]byte, Config.SkipPrefixBufSize)
    w.SkipIndex = 0
    return w
}

func (w *HttpStripWriter) Size() int {
    return len(w.SkipBuffer)
}

func (w *HttpStripWriter) Available() int {
    return w.Size() - w.SkipIndex
}

func (w *HttpStripWriter) AddToSkipBuffer(data []byte) int {
    toAdd := w.Available()
    if len(data) < toAdd {
        toAdd = len(data)
    }

    copy(w.SkipBuffer[w.SkipIndex:], data[:toAdd])
    w.SkipIndex += toAdd
    return toAdd
}


func (w *HttpStripWriter) ParseHttpHeaderSection() (bool, ErrorResponseCode) {
    /* Check if this is a valid HTTP header section and check status code */
    validHeaderSection := false
    statusCode := ErrorResponse200
    for _, header := range bytes.Split(w.SkipBuffer, []byte(DOSLineEnd)) {
        header = bytes.ToLower(header)

        if bytes.Contains(header, []byte("content-type: ")) {
            /* This whole header section is now _valid_ */
            validHeaderSection = true
        } else if bytes.Contains(header, []byte("status: ")) {
            /* Try parse status code */
            statusStr := bytes.Split(bytes.TrimPrefix(header, []byte("status: ")), []byte(" "))[0]
            switch string(statusStr) {
                case "200":
                    statusCode = ErrorResponse200
                case "400":
                    statusCode = ErrorResponse400
                case "401":
                    statusCode = ErrorResponse401
                case "403":
                    statusCode = ErrorResponse403
                case "404":
                    statusCode = ErrorResponse404
                case "408":
                    statusCode = ErrorResponse408
                case "410":
                    statusCode = ErrorResponse410
                case "500":
                    statusCode = ErrorResponse500
                case "501":
                    statusCode = ErrorResponse501
                case "503":
                    statusCode = ErrorResponse503
                default:
                    statusCode = ErrorResponse500
            }
        }
    }
    return validHeaderSection, statusCode
}

func (w *HttpStripWriter) WriteSkipBuffer() (bool, error) {
    defer func() {
        w.SkipIndex = 0
    }()

    /* First try parse the headers, determine what to do next */
    validHeaderSection, statusCode := w.ParseHttpHeaderSection()

    if validHeaderSection {
        /* Contains valid HTTP headers, if anything other than 200 statusCode we write error and return nil */
        if statusCode != ErrorResponse200 {
            /* Non-200 status code. Try send error response bytes and return with false (don't continue) */
            _, err := w.Writer.Write(generateGopherErrorResponse(statusCode))
            return false, err
        } else {
            /* Status code all good, we just return a true (do continue) */
            return true, nil
        }
    }

    /* Default is just write skip buffer contents */
    _, err := w.Writer.Write(w.SkipBuffer[:w.SkipIndex])
    return true, err
}

func (w *HttpStripWriter) FlushSkipBuffer() error {
    /* If SkipBuffer non-nil and has contents, make sure this is written!
     * This happens if caller to Write has supplied content length < w.Size().
     * This MUST be called.
     */

    if w.SkipIndex > 0 {
        _, err := w.WriteSkipBuffer()
        return err
    }

    return nil
}


func (w *HttpStripWriter) Write(data []byte) (int, error) {
    /* Write using whatever write function is currently set */
    return w.WriteFunc(data)
}

func (w *HttpStripWriter) WriteRegular(data []byte) (int, error) {
    /* Regular write function */
    return w.Writer.Write(data)
}

func (w *HttpStripWriter) WriteCheckForHeaders(data []byte) (int, error) {
    split := bytes.Split(data, []byte(DOSLineEnd+DOSLineEnd))
    if len(split) == 1 {
        /* Try add these to skip buffer */
        added := w.AddToSkipBuffer(data)

        if added < len(data) {
            defer func() {
                /* Having written skipbuffer after this if clause, set write to regular */
                w.WriteFunc = w.WriteRegular
            }()

            doContinue, err := w.WriteSkipBuffer()
            if !doContinue {
                /* If we shouldn't continue, return 'added' and unexpect EOF error */
                return added, io.ErrUnexpectedEOF
            } else if err != nil {
                /* If err not nil, return that we wrote up to 'added' and err */
                return added, err
            }

            /* Write remaining data not added to skip buffer */
            count, err := w.Writer.Write(data[added:])
            if err != nil {
                /* We return added+count */
                return added+count, err
            }
        }

        return len(data), nil
    } else {
        defer func() {
            /* No use for skip buffer after this clause, set write to regular */
            w.WriteFunc = w.WriteRegular
            w.SkipIndex = 0
        }()

        /* Try add what we can to skip buffer */
        added := w.AddToSkipBuffer(append(split[0], []byte(DOSLineEnd+DOSLineEnd)...))

        doContinue, err := w.WriteSkipBuffer()
        if !doContinue {
            /* If we shouldn't continue, return 'added' and unexpect EOF error */
            return added, io.ErrUnexpectedEOF
        } else if err != nil {
            /* If err not nil, return that we wrote up to 'added' and err */
            return added, err
        }

        /* Write remaining data not added to skip buffer */
        count, err := w.Writer.Write(data[added:])
        if err != nil {
            /* We return added+count */
            return added+count, err
        }

        return len(data), nil
    }
}
