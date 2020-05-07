package main

import (
    "io"
    "path"
    "strings"
    "bufio"
)

const (
    SocketWriteBufSize = 4096
)

type RequestPath struct {
    /* Path structure to allow hosts at
     * different roots while maintaining relative
     * and absolute path names for returned values
     * and filesystem reading
     */

    Root string
    Rel  string
    Abs  string
}

func NewRequestPath(rootDir, relPath string) *RequestPath {
    return &RequestPath{ rootDir, relPath, path.Join(rootDir, strings.TrimSuffix(relPath, "/")) }
}

func (rp *RequestPath) RootDir() string {
    return rp.Root
}

func (rp *RequestPath) Relative() string {
    return rp.Rel
}

func (rp *RequestPath) Absolute() string {
    return rp.Abs
}

func (rp *RequestPath) Selector() string {
    if rp.Rel == "." {
        return "/"
    } else {
        return "/"+rp.Rel
    }
}

type Request struct {
    Path       *RequestPath
    Parameters []string
}

func (r *Request) RootDir() string {
    return r.Path.RootDir()
}

func (r *Request) AbsPath() string {
    return r.Path.Absolute()
}

func (r *Request) RelPath() string {
    return r.Path.Relative()
}

func (r *Request) SelectorPath() string {
    return r.Path.Selector()
}

func (r *Request) PathJoinSelector(extPath string) string {
    return path.Join(r.SelectorPath(), extPath)
}

func (r *Request) PathJoinAbs(extPath string) string {
    return path.Join(r.AbsPath(), extPath)
}

func (r *Request) PathJoinRel(extPath string) string {
    return path.Join(r.RelPath(), extPath)
}

func (r *Request) PathHasAbsPrefix(prefix string) bool {
    return strings.HasPrefix(r.AbsPath(), prefix)
}

func (r *Request) PathHasRelPrefix(prefix string) bool {
    return strings.HasPrefix(r.RelPath(), prefix)
}

func (r *Request) PathHasRelSuffix(suffix string) bool {
    return strings.HasSuffix(r.RelPath(), suffix)
}

func (r *Request) PathHasAbsSuffix(suffix string) bool {
    return strings.HasSuffix(r.AbsPath(), suffix)
}

func (r *Request) PathTrimRelSuffix(suffix string) string {
    return strings.TrimSuffix(strings.TrimSuffix(r.RelPath(), suffix), "/")
}

func (r *Request) PathTrimAbsSuffix(suffix string) string {
    return strings.TrimSuffix(strings.TrimSuffix(r.AbsPath(), suffix), "/")
}

func (r *Request) PathJoinRootDir(extPath string) string {
    return path.Join(r.Path.RootDir(), extPath)
}

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
    bufWriter := bufio.NewWriterSize(conn.Conn, SocketWriteBufSize)
    return &Responder{ conn.Host, conn.Client, bufWriter, request }
}

func (r *Responder) AccessLogInfo(format string, args ...interface{}) {
    Config.AccLog.Info("("+r.Client.AddrStr()+") ", format, args...)
}

func (r *Responder) AccessLogError(format string, args ...interface{}) {
    Config.AccLog.Error("("+r.Client.AddrStr()+") ", format, args...)
}

func (r *Responder) Write(data []byte) *GophorError {
    _, err := r.Writer.Write(data)
    if err != nil {
        return &GophorError{ BufferedWriteErr, err }
    }
    return nil
}

func (r *Responder) WriteFlush(data []byte) *GophorError {
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
    if gophorErr != nil {
        return gophorErr
    } else {
        return r.Flush()
    }
}

func (r *Responder) WriteRaw(reader io.Reader) *GophorError {
    _, err := r.Writer.ReadFrom(reader)
    if err != nil {
        return &GophorError{ BufferedWriteReadErr, err }
    }
    return r.Flush()
}

/* Sanitize a request path string */
func sanitizeRelativePath(rootDir, relPath string) string {
    /* Start with a clean :) */
    relPath = path.Clean(relPath)

    if path.IsAbs(relPath) {
        /* Is absolute. Try trimming root and leading '/' */
        relPath = strings.TrimPrefix(strings.TrimPrefix(relPath, rootDir), "/")
    } else {
        /* Is relative. If back dir traversal, give them root */
        if strings.HasPrefix(relPath, "..") {
            relPath = ""
        }
    }

    return relPath
}
