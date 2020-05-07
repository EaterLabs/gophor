package main

import (
    "io"
    "strings"
)

const (
    /* Socket settings */
    SocketReadBufSize = 1024
    MaxSocketReadChunks = 4
)

type Worker struct {
    Conn *GophorConn
}

func (worker *Worker) Serve() {
    defer func() {
        /* Close-up shop */
        worker.Conn.Conn.Close()
    }()

    var count int
    var err error

    /* Read buffer + final result */
    buf := make([]byte, SocketReadBufSize)
    received := ""

    iter := 0
    endReached := false
    for {
        /* Buffered read from conn */
        count, err = worker.Conn.Read(buf)

        /* Copy buffer into received string, stop at first tap or CrLf */
        for i := 0; i < count; i += 1 {
            if buf[i] == Tab[0] {
                endReached = true
                break
            } else if buf[i] == DOSLineEnd[0] {
                if count > i+1 && buf[i+1] == DOSLineEnd[1] {
                    endReached = true
                    break
                }
            }
            received += string(buf[i])

        }

        /* Handle errors AFTER checking we didn't receive some bytes */
        if err != nil {
            if err == io.EOF {
                /* EOF, break */
                break
            }

            Config.SysLog.Error("", "Error reading from socket on port %s: %s\n", worker.Conn.Host.Port(), err.Error())
            return
        } else if endReached || count < SocketReadBufSize {
            /* Reached the end of what we want, break */
            break
        }

        /* Hit max read chunk size, send error + close connection */
        if iter == MaxSocketReadChunks {
            Config.SysLog.Error("", "Reached max socket read size %d. Closing connection...\n", MaxSocketReadChunks*SocketReadBufSize)
            return
        }

        /* Keep count :) */
        iter += 1
    }

    /* Handle URL request if presented */
    lenBefore := len(received)
    received = strings.TrimPrefix(received, "URL:")
    switch len(received) {
        case lenBefore-4:
            /* Send an HTML redirect to supplied URL */
            Config.AccLog.Info("("+worker.Conn.Client.Ip()+") ", "Redirecting to %s\n", received)
            worker.Conn.Write(generateHtmlRedirect(received))
            return
        default:
            /* Do nothing */
    }

    /* Create new request from dataStr */
    request := NewSanitizedRequest(worker.Conn, received)

    /* Create new responder from request */
    responder := NewResponder(worker.Conn, request)

    /* Handle request */
    gophorErr := Config.FileSystem.HandleRequest(responder)

    /* Handle any error */
    if gophorErr != nil {
        /* Log serve failure to access, error to system */
        Config.SysLog.Error("", gophorErr.Error())

        /* Generate response bytes from error code */
        errResponse := generateGopherErrorResponseFromCode(gophorErr.Code)

        /* If we got response bytes to send? SEND 'EM! */
        if errResponse != nil {
            /* No gods. No masters. We don't care about error checking here */
            responder.WriteFlush(errResponse)
        }

        responder.AccessLogError("Failed to serve: %s\n", request.AbsPath())
    } else {
        /* Log served */
        responder.AccessLogInfo("Served: %s\n", request.AbsPath())
    }
}
