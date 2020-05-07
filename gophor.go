package main

import (
    "os"
    "strconv"
    "syscall"
    "os/signal"
    "flag"
    "time"
)

const (
    GophorVersion = "0.7-beta-PR1"
)

var (
    Config *ServerConfig
)

func main() {
    /* Setup the entire server, getting slice of listeners in return */
    listeners := setupServer()

    /* Handle signals so we can _actually_ shutdowm */
    signals := make(chan os.Signal)
    signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)

    /* Start accepting connections on any supplied listeners */
    for _, l := range listeners {
        go func() {
            Config.SysLog.Info("", "Listening on: gopher://%s:%s\n", l.Host.Name(), l.Host.RealPort())

            for {
                connWrapper, err := l.Accept()
                if err != nil {
                    Config.SysLog.Error("", "Error accepting connection: %s\n", err.Error())
                    continue
                }

                /* Run this in it's own goroutine so we can go straight back to accepting */
                go func() {
                    worker := &Worker{ connWrapper }
                    worker.Serve()
                }()
            }
        }()
    }

    /* When OS signal received, we close-up */
    sig := <-signals
    Config.SysLog.Info("", "Signal received: %v. Shutting down...\n", sig)
    os.Exit(0)
}

func setupServer() []*GophorListener {
    /* First we setup all the flags and parse them... */

    /* Base server settings */
    serverRoot         := flag.String("root", "/var/gopher", "Change server root directory.")
    serverBindAddr     := flag.String("bind-addr", "127.0.0.1", "Change server socket bind address")
    serverPort         := flag.Int("port", 70, "Change server bind port.")

    serverFwdPort      := flag.Int("fwd-port", 0, "Change port used in '$port' replacement strings (useful if you're port forwarding).")
    serverHostname     := flag.String("hostname", "127.0.0.1", "Change server hostname (FQDN).")

    /* Logging settings */
    systemLogPath      := flag.String("system-log", "", "Change server system log file (blank outputs to stderr).")
    accessLogPath      := flag.String("access-log", "", "Change server access log file (blank outputs to stderr).")
    logOutput          := flag.String("log-output", "stderr", "Change server log file handling (disable|stderr|file)")
    logOpts            := flag.String("log-opts", "timestamp,ip", "Comma-separated list of log options (timestamp|ip)")

    /* Cache settings */
    fileMonitorFreq    := flag.Duration("file-monitor-freq", time.Second*60, "Change file monitor frequency.")
    cacheSize          := flag.Int("cache-size", 50, "Change file cache size, measured in file count.")
    cacheFileSizeMax   := flag.Float64("cache-file-max", 0.5, "Change maximum file size to be cached (in megabytes).")
    cacheDisabled      := flag.Bool("disable-cache", false, "Disable file caching.")

    /* Content settings */
    pageWidth          := flag.Int("page-width", 80, "Change page width used when formatting output.")
//    charSet            := flag.String("charset", "", "Change default output charset.")

    footerText         := flag.String("footer", " Gophor, a Gopher server in Go.", "Change gophermap footer text (Unix new-line separated lines).")
    footerSeparator    := flag.Bool("no-footer-separator", false, "Disable footer line separator.")

    /* Regex */
    restrictedFiles    := flag.String("restrict-files", "", "New-line separated list of regex statements restricting accessible files.")
    restrictedCommands := flag.String("restrict-commands", "", "New-line separated list of regex statements restricting accessible commands.")

    /* User supplied caps.txt information */
    serverDescription  := flag.String("description", "Gophor, a Gopher server in Go.", "Change server description in generated caps.txt.")
    serverAdmin        := flag.String("admin-email", "", "Change admin email in generated caps.txt.")
    serverGeoloc       := flag.String("geoloc", "", "Change server gelocation string in generated caps.txt.")

    /* Exec settings */
    disableCgi         := flag.Bool("disable-cgi", false, "Disable CGI and all executable support.")
    safeExecPath       := flag.String("safe-path", "/usr/bin:/bin", "Set safe PATH variable to be used when executing CGI scripts, gophermaps and inline shell commands.")
    maxExecRunTime     := flag.Duration("max-exec-time", time.Second*3, "Change max executable CGI, gophermap and inline shell command runtime.")

    /* Buffer sizes */
    socketWriteBuf     := flag.Int("socket-write-buf", 4096, "Change socket write buffer size (bytes).")
    socketReadBuf      := flag.Int("socket-read-buf", 256, "Change socket read buffer size (bytes).")
    socketReadMax      := flag.Int("socket-read-max", 8, "Change socket read count max (integer multiplier socket-read-buf-max)")
    skipPrefixBuf      := flag.Int("cgi-header-max", 4096, "Change max CGI read count to look for and strip HTTP headers before sending raw (bytes).")
    fileReadBuf        := flag.Int("file-read-buf", 4096, "Change file read buffer size (bytes).")

    /* Version string */
    version            := flag.Bool("version", false, "Print version information.")

    /* Parse parse parse!! */
    flag.Parse()
    if *version {
        printVersionExit()
    }

    /* Setup the server configuration instance and enter as much as we can right now */
    Config = new(ServerConfig)

    /* Set misc content settings */
    Config.PageWidth = *pageWidth

    /* Setup various buffer sizes */
    Config.SocketWriteBufSize = *socketWriteBuf
    Config.SocketReadBufSize  = *socketReadBuf
    Config.SocketReadMax      = *socketReadBuf * *socketReadMax
    Config.FileReadBufSize    = *fileReadBuf

    /* Have to be set AFTER page width variable set */
    Config.FooterText = formatGophermapFooter(*footerText, !*footerSeparator)

    /* Setup Gophor logging system */
    Config.SysLog, Config.AccLog = setupLoggers(*logOutput, *logOpts, *systemLogPath, *accessLogPath)

    /* Set CGI support status */
    if *disableCgi {
        Config.SysLog.Info("", "CGI support disabled")
        Config.CgiEnabled = false


    } else {
        /* Enable CGI */
        Config.SysLog.Info("", "CGI support enabled")
        Config.CgiEnabled = true

        /* Set safe executable path and setup environments */
        Config.SysLog.Info("", "Setting safe executable path: %s\n", *safeExecPath)
        Config.Env = setupExecEnviron(*safeExecPath)
        Config.CgiEnv = setupInitialCgiEnviron(*safeExecPath)

        /* Set executable watchdog */
        Config.SysLog.Info("", "Maximum executable time: %s\n", *maxExecRunTime)
        Config.MaxExecRunTime = *maxExecRunTime

        /* Specific to CGI buffer */
        Config.SysLog.Info("", "Maximum CGI HTTP header read-ahead: %d bytes\n", *skipPrefixBuf)
        Config.SkipPrefixBufSize = *skipPrefixBuf
    }

    /* If running as root, get ready to drop privileges */
    if syscall.Getuid() == 0 || syscall.Getgid() == 0 {
        Config.SysLog.Fatal("", "Gophor does not support running as root!\n")
    }

    /* Enter server dir */
    enterServerDir(*serverRoot)
    Config.SysLog.Info("", "Entered server directory: %s\n", *serverRoot)

    /* Setup listeners */
    listeners := make([]*GophorListener, 0)

    /* If requested, setup unencrypted listener */
    if *serverPort != 0 {
        /* If no forward port set, just use regular */
        if *serverFwdPort == 0 {
            *serverFwdPort = *serverPort
        }

        l, err := BeginGophorListen(*serverBindAddr, *serverHostname, strconv.Itoa(*serverPort), strconv.Itoa(*serverFwdPort), *serverRoot)
        if err != nil {
            Config.SysLog.Fatal("", "Error setting up (unencrypted) listener: %s\n", err.Error())
        }
        listeners = append(listeners, l)
    } else {
        Config.SysLog.Fatal("", "No valid port to listen on\n")
    }

    /* Compile regex statements */
    Config.CmdParseLineRegex  = compileCmdParseRegex()
    Config.RestrictedFiles    = compileUserRestrictedFilesRegex(*restrictedFiles)
    Config.RestrictedCommands = compileUserRestrictedCommandsRegex(*restrictedCommands)

    /* Setup file cache */
    Config.FileSystem = new(FileSystem)

    /* Check if cache requested disabled */
    if !*cacheDisabled {
        /* Init file cache */
        Config.FileSystem.Init(*cacheSize, *cacheFileSizeMax)

        /* Before file monitor or any kind of new goroutines started,
         * check if we need to cache generated policy files
         */
        cachePolicyFiles(*serverRoot, *serverDescription, *serverAdmin, *serverGeoloc)

        /* Start file cache freshness checker */
        go startFileMonitor(*fileMonitorFreq)
        Config.SysLog.Info("", "File caching enabled with: maxcount=%d maxsize=%.3fMB checkfreq=%s\n", *cacheSize, *cacheFileSizeMax, *fileMonitorFreq)
    } else {
        /* File caching disabled, init with zero max size so nothing gets cached */
        Config.FileSystem.Init(2, 0)
        Config.SysLog.Info("", "File caching disabled\n")

        /* Safe to cache policy files now */
        cachePolicyFiles(*serverRoot, *serverDescription, *serverAdmin, *serverGeoloc)
    }

    /* Return the created listeners slice :) */
    return listeners
}

func enterServerDir(path string) {
    err := syscall.Chdir(path)
    if err != nil {
        Config.SysLog.Fatal("", "Error changing dir to server root %s: %s\n", path, err.Error())
    }
}
