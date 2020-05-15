package main

import (
    "strings"
)

type Worker struct {
    Conn    *BufferedDeadlineConn
    Host    *ConnHost
    Client  *ConnClient
    RootDir string
}

func (worker *Worker) Serve() {
    defer worker.Conn.Close()

    line, err := worker.Conn.ReadLine()
    if err != nil {
        Config.SysLog.Error("", "Error reading from socket port %s: %s\n", worker.Host.Port(), err.Error())
        return
    }

    /* Drop up to first tab */
    received := strings.Split(string(line), Tab)[0]

    /* Handle URL request if presented */
    lenBefore := len(received)
    received = strings.TrimPrefix(received, "URL:")
    switch len(received) {
        case lenBefore-4:
            /* Send an HTML redirect to supplied URL */
            Config.AccLog.Info("("+worker.Client.Ip()+") ", "Redirecting to %s\n", received)
            worker.Conn.Write(generateHtmlRedirect(received))
            return
        default:
            /* Do nothing */
    }

    /* Create GopherUrl object from request string */
    url, gophorErr := parseGopherUrl(received)
    if gophorErr == nil {
        /* Create new request from url object */
        request := NewSanitizedRequest(worker.RootDir, url)

        /* Create new responder from request */
        responder := NewResponder(worker.Conn, worker.Host, worker.Client, request)

        /* Handle request with supplied responder */
        gophorErr = Config.FileSystem.HandleRequest(responder)
        if gophorErr == nil {
            /* Log success to access and return! */
            responder.AccessLogInfo("Served: %s\n", request.Path.Absolute())
            return
        } else {
            /* Log failure to access */
            responder.AccessLogError("Failed to serve: %s\n", request.Path.Absolute())
        }
    }

    /* Log serve failure to error to system */
    Config.SysLog.Error("", gophorErr.Error())

    /* Generate response bytes from error code */
    errResponse := generateGopherErrorResponseFromCode(gophorErr.Code)

    /* If we got response bytes to send? SEND 'EM! */
    if errResponse != nil {
        /* No gods. No masters. We don't care about error checking here */
        worker.Conn.WriteData(errResponse)
    }
}
