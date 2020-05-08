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

type SkipPrefixWriter struct {
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

    SkipUpTo   []byte
    SkipBuffer []byte
    Available  int

    ShouldWriteSkipped func([]byte) bool
}

func NewSkipPrefixWriter(writer *bufio.Writer, skipUpTo []byte, shouldWriteSkipped func(data []byte) bool) *SkipPrefixWriter {
    w := &SkipPrefixWriter{}
    w.Writer = writer
    w.WriteFunc = w.WriteCheckSkip
    w.SkipUpTo = skipUpTo
    w.SkipBuffer = make([]byte, Config.SkipPrefixBufSize)
    w.ShouldWriteSkipped = shouldWriteSkipped
    w.Available = Config.SkipPrefixBufSize
    return w
}

func (w *SkipPrefixWriter) AddToSkipBuffer(data []byte) int {
    /* Add as much data as we can to the skip buffer */
    if len(data) >= w.Available {
        toAdd := w.Available
        w.SkipBuffer = append(w.SkipBuffer, data[:toAdd]...)
        w.Available = 0
        return toAdd
    } else {
        w.SkipBuffer = append(w.SkipBuffer, data...)
        w.Available -= len(data)
        return len(data)
    }
}

func (w *SkipPrefixWriter) Write(data []byte) (int, error) {
    return w.WriteFunc(data)
}

func (w *SkipPrefixWriter) WriteRegular(data []byte) (int, error) {
    return w.Writer.Write(data)
}

func (w *SkipPrefixWriter) WriteCheckSkip(data []byte) (int, error) {
    split := bytes.Split(data, w.SkipUpTo)
    if len(split) == 1 {
        /* Try add these to skip buffer */
        added := w.AddToSkipBuffer(data)

        if added < len(data) {
            defer func() {
                w.WriteFunc = w.WriteRegular
            }()

            /* Write contents of skip buffer */
            _, err := w.Writer.Write(w.SkipBuffer)
            if err != nil {
                /* We return 0 here as if failed here our count is massively off anyways */
                return 0, err
            }

            /* Write remaining data not added to skip buffer */
            _, err = w.Writer.Write(data[added:])
            if err != nil {
                /* We return 0 here as if failed here our count is massively off anyways */
                return 0, err
            }
        }

        return len(data), nil
    } else {
        defer func() {
            w.WriteFunc = w.WriteRegular
        }()

        /* Try add what we can to skip buffer */
        added := w.AddToSkipBuffer(append(split[0], w.SkipUpTo...))

        /* Check if we should write contents of skip buffer */
        if !w.ShouldWriteSkipped(w.SkipBuffer) {
            /* Skip empty data remaining */
            if added >= len(data)-1 {
                return len(data), nil
            }

            /* Write from index = added */
            count, err := w.Writer.Write(data[added:])
            if err != nil {
                /* Failed, return added+count as write count*/
                return added+count, err
            }
        } else {
            /* We write skip buffer contents */
            count, err := w.Writer.Write(w.SkipBuffer)
            if err != nil {
                /* Failed, assume write up to added */
                return added, err
            }

            /* Now write remaining */
            count, err = w.Writer.Write(data[added:])
            if err != nil {
                /* Failed, return count from added */
                return added+count, err
            }
        }

        return len(data), nil
    }
}
