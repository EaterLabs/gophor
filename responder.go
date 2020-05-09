package main

import (
    "io"
    "bufio"
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
