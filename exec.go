package main

import (
    "io"
    "os/exec"
    "syscall"
    "strconv"
    "time"
)

/* Setup initial (i.e. constant) CGI environment variables */
func setupInitialCgiEnviron(path, charset string) []string {
    return []string{
        /* RFC 3875 standard */
        envKeyValue("GATEWAY_INTERFACE",  "CGI/1.1"),               /* MUST be set to the dialect of CGI being used by the server */
        envKeyValue("SERVER_SOFTWARE",    "gophor/"+GophorVersion), /* MUST be set to name and version of server software serving this request */
        envKeyValue("SERVER_PROTOCOL",    "gopher"),                /* MUST be set to name and version of application protocol used for this request */
        envKeyValue("CONTENT_LENGTH",     "0"),                     /* Contains size of message-body attached (always 0 so we set here) */
        envKeyValue("REQUEST_METHOD",     "GET"),                   /* MUST be set to method by which script should process request. Always GET */

        /* Non-standard */
        envKeyValue("PATH",               path),
        envKeyValue("COLUMNS",            strconv.Itoa(Config.PageWidth)),
        envKeyValue("GOPHER_CHARSET",     charset),
    }
}

/* Generate CGI environment */
func generateCgiEnvironment(responder *Responder) []string {
    /* Get initial CgiEnv variables */
    env := Config.CgiEnv

    env = append(env, envKeyValue("SERVER_NAME",     responder.Host.Name())) /* MUST be set to name of server host client is connecting to */
    env = append(env, envKeyValue("SERVER_PORT",     responder.Host.Port())) /* MUST be set to the server port that client is connecting to */
    env = append(env, envKeyValue("REMOTE_ADDR",     responder.Client.Ip())) /* Remote client addr, MUST be set */
    env = append(env, envKeyValue("QUERY_STRING",    responder.Request.Parameters))          /* URL encoded search or parameter string, MUST be set even if empty */
    env = append(env, envKeyValue("SCRIPT_NAME",     "/"+responder.Request.Path.Relative())) /* URI path (not URL encoded) which could identify the CGI script (rather than script's output) */
    env = append(env, envKeyValue("SCRIPT_FILENAME", responder.Request.Path.Absolute()))     /* Basically SCRIPT_NAME absolute path */
    env = append(env, envKeyValue("SELECTOR",        responder.Request.Path.Selector()))
    env = append(env, envKeyValue("DOCUMENT_ROOT",   responder.Request.Path.RootDir()))
    env = append(env, envKeyValue("REQUEST_URI",     "/"+responder.Request.Path.Relative()+responder.Request.Parameters))

    return env
}

/* Execute a CGI script (pointer to correct function) */
var executeCgi func(*Responder) *GophorError

/* Execute CGI script and serve as-is */
func executeCgiNoHttp(responder *Responder) *GophorError {
    return execute(responder.Conn, generateCgiEnvironment(responder), responder.Request.Path.Absolute())
}

/* Execute CGI script and strip HTTP headers */
func executeCgiStripHttp(responder *Responder) *GophorError {
    /* HTTP header stripping writer that also parses HTTP status codes */
    httpStripWriter := NewHttpStripWriter(responder.Conn)

    /* Execute the CGI script using the new httpStripWriter */
    gophorErr := execute(httpStripWriter, generateCgiEnvironment(responder), responder.Request.Path.Absolute())

    /* httpStripWriter's error takes priority as it might have parsed the status code */
    cgiStatusErr := httpStripWriter.FinishUp()
    if cgiStatusErr != nil {
        return cgiStatusErr
    } else {
        return gophorErr
    }
}

/* Execute a supplied path with arguments and environment, to writer */
func execute(writer io.Writer, env []string, path string) *GophorError {
    /* If CGI disbabled, just return error */
    if !Config.CgiEnabled {
        return &GophorError{ CgiDisabledErr, nil }
    }

    /* Setup command */
    cmd := exec.Command(path)

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
        exitError, ok := err.(*exec.ExitError)
        if ok {
            waitStatus := exitError.Sys().(syscall.WaitStatus)
            exitCode = waitStatus.ExitStatus()
        } else {
            exitCode = 1
        }
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
