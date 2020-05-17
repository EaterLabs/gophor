package main

import (
    "time"
    "regexp"
)

/* ServerConfig:
 * Holds onto global server configuration details
 * and any data objects we want to keep in memory
 * (e.g. loggers, restricted files regular expressions
 * and file cache)
 */
type ServerConfig struct {
    /* Executable Settings */
    CgiDir              string
    CgiEnv              []string
    CgiEnabled          bool
    MaxExecRunTime      time.Duration

    /* Content settings */
    FooterText          []byte
    PageWidth           int

    /* Logging */
    SysLog              LoggerInterface
    AccLog              LoggerInterface

    /* Filesystem access */
    FileSystem          *FileSystem

    /* Buffer sizes */
    SocketWriteBufSize  int
    SocketReadBufSize   int
    SocketReadMax       int
    SkipPrefixBufSize   int
    FileReadBufSize     int

    /* Socket deadlines */
    SocketReadDeadline  time.Duration
    SocketWriteDeadline time.Duration

    /* Precompiled regular expressions */
    RgxGophermap        *regexp.Regexp
    RgxCgiBin           *regexp.Regexp
}
