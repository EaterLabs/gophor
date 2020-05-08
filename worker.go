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
    defer worker.Conn.Close()

    /* Read buffer + final result */
    buf := make([]byte, SocketReadBufSize)

    received := ""
    receivedCount := 0
    endReached := false

    var count int
    var err error
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
            receivedCount += 1
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
        if receivedCount >= Config.SocketReadMax {
            Config.SysLog.Error("", "Reached max socket read size %d. Closing connection...\n", MaxSocketReadChunks*SocketReadBufSize)
            return
        }
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

    /* Create new request from received */
    request := NewSanitizedRequest(worker.Conn, received)

    /* Create new responder from request */
    responder := NewResponder(worker.Conn, request)

    /* Handle request with supplied responder */
    gophorErr := Config.FileSystem.HandleRequest(responder)

    /* Handle any error */
    if gophorErr != nil {
        /* Log serve failure to error to system */
        Config.SysLog.Error("", gophorErr.Error())

        /* Generate response bytes from error code */
        errResponse := generateGopherErrorResponseFromCode(gophorErr.Code)

        /* If we got response bytes to send? SEND 'EM! */
        if errResponse != nil {
            /* No gods. No masters. We don't care about error checking here */
            responder.WriteFlush(errResponse)
        }

        /* Log failure to access */
        responder.AccessLogError("Failed to serve: %s\n", request.Path.Absolute())
    } else {
        /* Log served to access */
        responder.AccessLogInfo("Served: %s\n", request.Path.Absolute())
    }
}
