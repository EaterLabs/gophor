package main

import (
    "io"
)

type Responder struct {
    Conn        *BufferedDeadlineConn
    Host        *ConnHost
    Client      *ConnClient
    Request     *Request
}

func NewResponder(conn *BufferedDeadlineConn, host *ConnHost, client *ConnClient, request *Request) *Responder {
    return &Responder{ conn, host, client, request }
}

func (r *Responder) AccessLogInfo(format string, args ...interface{}) {
    Config.AccLog.Info("("+r.Client.Ip()+") ", format, args...)
}

func (r *Responder) AccessLogError(format string, args ...interface{}) {
    Config.AccLog.Error("("+r.Client.Ip()+") ", format, args...)
}

func (r *Responder) Write(b []byte) (int, error) {
    return r.Conn.Write(b)
}

func (r *Responder) WriteData(data []byte) *GophorError {
    err := r.Conn.WriteData(data)
    if err != nil {
        return &GophorError{ SocketWriteErr, err }
    }
    return nil
}

func (r *Responder) WriteRaw(reader io.Reader) *GophorError {
    err := r.Conn.WriteRaw(reader)
    if err != nil {
        return &GophorError{ SocketWriteRawErr, err }
    }
    return nil
}

func (r *Responder) CloneWithRequest(request *Request) *Responder {
    /* Create new copy of Responder only with request differring */
    return &Responder{
        r.Conn,
        r.Host,
        r.Client,
        request,
    }
}
