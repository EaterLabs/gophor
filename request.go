package main

import (
    "path"
    "strings"
)

type RequestPath struct {
    /* Path structure to allow hosts at
     * different roots while maintaining relative
     * and absolute path names for filesystem reading
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

func (rp *RequestPath) JoinRel(extPath string) string {
    return path.Join(rp.Relative(), extPath)
}

func (rp *RequestPath) JoinAbs(extPath string) string {
    return path.Join(rp.Absolute(), extPath)
}

func (rp *RequestPath) JoinSelector(extPath string) string {
    return path.Join(rp.Selector(), extPath)
}

func (rp *RequestPath) HasAbsPrefix(prefix string) bool {
    return strings.HasPrefix(rp.Absolute(), prefix)
}

func (rp *RequestPath) HasRelPrefix(prefix string) bool {
    return strings.HasPrefix(rp.Relative(), prefix)
}

func (rp *RequestPath) HasRelSuffix(suffix string) bool {
    return strings.HasSuffix(rp.Relative(), suffix)
}

func (rp *RequestPath) HasAbsSuffix(suffix string) bool {
    return strings.HasSuffix(rp.Absolute(), suffix)
}

func (rp *RequestPath) TrimRelSuffix(suffix string) string {
    return strings.TrimSuffix(strings.TrimSuffix(rp.Relative(), suffix), "/")
}

func (rp *RequestPath) TrimAbsSuffix(suffix string) string {
    return strings.TrimSuffix(strings.TrimSuffix(rp.Absolute(), suffix), "/")
}

func (rp *RequestPath) JoinRootDir(extPath string) string {
    return path.Join(rp.RootDir(), extPath)
}

type Request struct {
    /* Holds onto a request path to the filesystem and
     * a string slice of parsed parameters (usually nil
     * or length 1)
     */

    Path       *RequestPath
    Parameters []string
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
