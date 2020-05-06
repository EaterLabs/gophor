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
    ConnWrapper *GophorConnWrapper
}

func (worker *Worker) Serve() {
    defer func() {
        /* Close-up shop */
        worker.ConnWrapper.Conn.Close()
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
        count, err = worker.ConnWrapper.Conn.Read(buf)

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

            Config.SysLog.Error("", "Error reading from socket on port %s: %s\n", worker.ConnWrapper.Host.Port(), err.Error())
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
            Config.AccLog.Info("("+worker.ConnWrapper.Client.Ip()+") ", "Redirecting to %s\n", received)
            worker.ConnWrapper.Conn.Write(generateHtmlRedirect(received))
            return
        default:
            /* Do nothing */
    }

    /* Create new request from dataStr */
    request := NewSanitizedRequest(worker.ConnWrapper, received)

    /* Handle request */
    gophorErr := Config.FileSystem.HandleRequest(request)

    /* Handle any error */
    if gophorErr != nil {
        /* Log serve failure to access, error to system */
        Config.SysLog.Error("", gophorErr.Error())

        /* Generate response bytes from error code */
        response := generateGopherErrorResponseFromCode(gophorErr.Code)

        /* If we got response bytes to send? SEND 'EM! */
        if response != nil {
            /* No gods. No masters. We don't care about error checking here */
            request.WriteFlush(response)
        }

        request.AccessLogError("Failed to serve: %s\n", request.AbsPath())
    } else {
        /* Log served */
        request.AccessLogInfo("Served: %s\n", request.AbsPath())
    }
}
