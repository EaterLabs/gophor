package main

import (
    "os/exec"
    "syscall"
    "strconv"
    "bytes"
    "time"
    "io"
)

/* Setup initial (i.e. constant) gophermap / command environment variables */
func setupExecEnviron(path string) []string {
    return []string {
        envKeyValue("PATH", path),
    }
}

/* Setup initial (i.e. constant) CGI environment variables */
func setupInitialCgiEnviron(path string) []string {
    return []string{
        /* RFC 3875 standard */
        envKeyValue("GATEWAY_INTERFACE",  "CGI/1.1"),               /* MUST be set to the dialect of CGI being used by the server */
        envKeyValue("SERVER_SOFTWARE",    "gophor/"+GophorVersion), /* MUST be set to name and version of server software serving this request */
        envKeyValue("SERVER_PROTOCOL",    "RFC1436"),               /* MUST be set to name and version of application protocol used for this request */
        envKeyValue("CONTENT_LENGTH",     "0"),                     /* Contains size of message-body attached (always 0 so we set here) */
        envKeyValue("REQUEST_METHOD",     "GET"),                   /* MUST be set to method by which script should process request. Always GET */

        /* Non-standard */
        envKeyValue("PATH",               path),
        envKeyValue("COLUMNS",            strconv.Itoa(Config.PageWidth)),
        envKeyValue("GOPHER_CHARSET",     Config.CharSet),
    }
}

/* Execute a CGI script */
func executeCgi(responder *Responder) *GophorError {
    /* Get initial CgiEnv variables */
    cgiEnv := Config.CgiEnv
    cgiEnv = append(cgiEnv, envKeyValue("SERVER_NAME",     responder.Host.Name())) /* MUST be set to name of server host client is connecting to */
    cgiEnv = append(cgiEnv, envKeyValue("SERVER_PORT",     responder.Host.Port())) /* MUST be set to the server port that client is connecting to */
    cgiEnv = append(cgiEnv, envKeyValue("REMOTE_ADDR",     responder.Client.Ip())) /* Remote client addr, MUST be set */

    /* We store the query string in Parameters[0]. Ensure we git without initial delimiter */
    var queryString string
    if len(responder.Request.Parameters[0]) > 0 {
        queryString = responder.Request.Parameters[0][1:]
    } else {
        queryString = responder.Request.Parameters[0]
    }
    cgiEnv = append(cgiEnv, envKeyValue("QUERY_STRING",    queryString))                     /* URL encoded search or parameter string, MUST be set even if empty */
    cgiEnv = append(cgiEnv, envKeyValue("SCRIPT_NAME",     "/"+responder.Request.Path.Relative())) /* URI path (not URL encoded) which could identify the CGI script (rather than script's output) */
    cgiEnv = append(cgiEnv, envKeyValue("SCRIPT_FILENAME", responder.Request.Path.Absolute()))     /* Basically SCRIPT_NAME absolute path */
    cgiEnv = append(cgiEnv, envKeyValue("SELECTOR",        responder.Request.Path.Selector()))
    cgiEnv = append(cgiEnv, envKeyValue("DOCUMENT_ROOT",   responder.Request.Path.RootDir()))
    cgiEnv = append(cgiEnv, envKeyValue("REQUEST_URI",     "/"+responder.Request.Path.Relative()+responder.Request.Parameters[0]))

    /* Fuck it. For now, we don't support PATH_INFO. It's a piece of shit variable */
//    cgiEnv = append(cgiEnv, envKeyValue("PATH_INFO",       responder.Parameters[0])) /* Sub-resource to be fetched by script, derived from path hierarch portion of URI. NOT URL encoded */
//    cgiEnv = append(cgiEnv, envKeyValue("PATH_TRANSLATED", responder.AbsPath()))     /* Take PATH_INFO, parse as local URI and append root dir */

/* We ignore these due to just CBA and we're not implementing authorization yet */
//    cgiEnv = append(cgiEnv, envKeyValue("AUTH_TYPE",       "")) /* Any method used my server to authenticate user, MUST be set if auth'd */
//    cgiEnv = append(cgiEnv, envKeyValue("CONTENT_TYPE",    "")) /* Only a MUST if HTTP content-type set (so never for gopher) */
//    cgiEnv = append(cgiEnv, envKeyValue("REMOTE_IDENT",    "")) /* Remote client identity information */
//    cgiEnv = append(cgiEnv, envKeyValue("REMOTE_HOST",     "")) /* Remote client domain name */
//    cgiEnv = append(cgiEnv, envKeyValue("REMOTE_USER",     "")) /* Remote user ID, if AUTH_TYPE, MUST be set */

     /* Create nwe SkipPrefixwriter from underlying response writer set to skip up to:
      * \r\n\r\n
      * Then checks if it contains a valid 'content-type:' header, if so it strips these.
      */
     skipPrefixWriter := NewSkipPrefixWriter(
         responder.Writer,
         []byte(DOSLineEnd+DOSLineEnd),
         func(skipBuffer []byte) bool {
             for _, header := range bytes.Split(skipBuffer, []byte(DOSLineEnd)) {
                 header = bytes.ToLower(header)
                 if bytes.Contains(header, []byte("content-type:")) {
                     return false
                 }
             }
             return true
         },
    )

    /* Execute the CGI script using the new SkipPrefixWriter and above environment */
    return execute(skipPrefixWriter, cgiEnv, responder.Request.Path.Absolute(), nil)
}

/* Execute any file (though only allowed are gophermaps) */
func executeFile(responder *Responder) *GophorError {
    return execute(responder.Writer, Config.Env, responder.Request.Path.Absolute(), responder.Request.Parameters)
}

/* Check command is not restricted then execute */
func executeCommand(responder *Responder) *GophorError {
    if isRestrictedCommand(responder.Request.Path.Absolute()) {
        return &GophorError{ RestrictedCommandErr, nil }
    }
    return execute(responder.Writer, Config.Env, responder.Request.Path.Absolute(), responder.Request.Parameters)
}

/* Execute a supplied path with arguments and environment, to writer */
func execute(writer io.Writer, env []string, path string, args []string) *GophorError {
    /* If CGI disbabled, just return error */
    if !Config.CgiEnabled {
        return &GophorError{ CgiDisabledErr, nil }
    }

    /* Setup command */
    var cmd *exec.Cmd
    if args != nil {
        cmd = exec.Command(path, args...)
    } else {
        cmd = exec.Command(path)
    }

    /* Set new proccess group id */
    cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

    /* Setup cmd env */
    cmd.Env = env

    /* Setup out buffer */
    cmd.Stdout = writer

    /* Start executing! */
    err := cmd.Start()
    if err != nil {
        return &GophorError{ CommandStartErr, err }
    }

    /* Setup timer goroutine to kill cmd after x time */
    go func() {
        time.Sleep(Config.MaxExecRunTime)

        if cmd.ProcessState != nil {
            /* We've already finished */
            return
        }

        /* Get process group id */
        pgid, err := syscall.Getpgid(cmd.Process.Pid)
        if err != nil {
            Config.SysLog.Fatal("", "Process unfinished, PGID not found!\n")
        }

        /* Kill process group! */
        err = syscall.Kill(-pgid, syscall.SIGTERM)
        if err != nil {
            Config.SysLog.Fatal("", "Error stopping process group %d: %s\n", pgid, err.Error())
        }
    }()

    /* Wait for command to finish, get exit code */
    err = cmd.Wait()
    exitCode := 0
    if err != nil {
        /* Error, try to get exit code */
        exitError, _ := err.(*exec.ExitError)
        waitStatus   := exitError.Sys().(syscall.WaitStatus)
        exitCode      = waitStatus.ExitStatus()
    } else {
        /* No error! Get exit code direct from command */
        waitStatus := cmd.ProcessState.Sys().(syscall.WaitStatus)
        exitCode = waitStatus.ExitStatus()
    }

    if exitCode != 0 {
        /* If non-zero exit code return error */
        Config.SysLog.Error("", "Error executing: %s\n", path)
        return &GophorError{ CommandExitCodeErr, err }
    } else {
        return nil
    }
}

/* Just neatens creating an environment KEY=VALUE string */
func envKeyValue(key, value string) string {
    return key+"="+value
}
