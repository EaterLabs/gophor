package main

import (
    "path"
    "strings"
)

type RequestPath struct {
    /* Path structure to allow hosts at
     * different roots while maintaining relative
     * and absolute path names for returned values
     * and filesystem reading
     */

    Root   string
    Rel    string
    Abs    string
    Select string
}

func NewRequestPath(rootDir, relPath string) *RequestPath {
    return &RequestPath{ rootDir, relPath, path.Join(rootDir, strings.TrimSuffix(relPath, "/")), relPath }
}

func (rp *RequestPath) RemapActual(newRel string) {
    rp.Rel = newRel
    rp.Abs = path.Join(rp.Root, strings.TrimSuffix(newRel, "/"))
}

func (rp *RequestPath) RemapVirtual(newSel string) {
    rp.Select = newSel
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
    if rp.Select == "." {
        return "/"
    } else {
        return "/"+rp.Select
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
