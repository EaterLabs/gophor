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
    _, err := r.Writer.Write(data)
    if err != nil {
        return &GophorError{ BufferedWriteErr, err }
    }
    return nil
}

func (r *Responder) WriteFlush(data []byte) *GophorError {
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
    if gophorErr != nil {
        return gophorErr
    } else {
        return r.Flush()
    }
}

func (r *Responder) WriteRaw(reader io.Reader) *GophorError {
    _, err := r.Writer.ReadFrom(reader)
    if err != nil {
        return &GophorError{ BufferedWriteReadErr, err }
    }
    return r.Flush()
}

type SkipPrefixWriter struct {
    Writer     *bufio.Writer

    SkipUpTo   []byte
    SkipBuffer []byte
    HasSkipped bool

    ShouldContinue func([]byte) bool
}

func NewSkipPrefixWriter(writer *bufio.Writer, skipUpTo []byte, shouldContinue func(data []byte) bool) *SkipPrefixWriter {
    return &SkipPrefixWriter{ writer, skipUpTo, make([]byte, Config.SkipPrefixBufSize), false, shouldContinue }
}

func (w *SkipPrefixWriter) AddToSkipBuffer(data []byte) int {
    i := 0

    withinBounds := len(w.SkipBuffer) < cap(w.SkipBuffer)
    for i < len(data) {
        if !withinBounds {
            w.HasSkipped = true
            break
        }

        w.SkipBuffer = append(w.SkipBuffer, data[i])
        withinBounds = len(w.SkipBuffer) < cap(w.SkipBuffer)
        i += 1
    }

    return i
}

func (w *SkipPrefixWriter) Write(data []byte) (int, error) {
    if !w.HasSkipped {
        split := bytes.Split(data, w.SkipUpTo)
        if len(split) == 1 {
            /* Try add these to skip buffer */
            added := w.AddToSkipBuffer(data)

            if added != len(data) {
                /* We've hit the skip buffer max. Write skip buffer */
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

                /* Clear the skip buffer */
                w.SkipBuffer = nil

                return len(data), nil
            }

            return len(data), nil
        } else {
            /* Set us as skipped! */
            w.HasSkipped = true

            /* Get length of the byte slice to save */
            savedLen := len(split[0])+len(w.SkipUpTo)

            /* We only want up to first SkipUpTo, so take first element */
            w.SkipBuffer = append(w.SkipBuffer, split[0]...)

            /* Check if we should continue */
            if !w.ShouldContinue(w.SkipBuffer) {
                return savedLen, io.ErrUnexpectedEOF
            }

            /* Create return slice from remaining data */
            ret := bytes.Join(split[1:], w.SkipUpTo)
            count, err := w.Writer.Write(ret)
            return count+savedLen, err
        }
    } else {
        return w.Writer.Write(data)
    }
}
