package main

import (
    "os"
    "sync"
    "time"
)

const (
    /* Help converting file size stat to supplied size in megabytes */
    BytesInMegaByte = 1048576.0

    /* Filename constants */
    CgiBinDirStr     = "cgi-bin"
    GophermapFileStr = "gophermap"
)

type FileSystem struct {
    /* Holds and helps manage our file cache, as well as managing
     * access and responses to requests submitted a worker instance.
     */

    CacheMap     *FixedMap
    CacheMutex   sync.RWMutex
    CacheFileMax int64
    Remap        map[string]string
    ReverseRemap map[string]string
}

func (fs *FileSystem) Init(size int, fileSizeMax float64) {
    fs.CacheMap     = NewFixedMap(size)
    fs.CacheMutex   = sync.RWMutex{}
    fs.CacheFileMax = int64(BytesInMegaByte * fileSizeMax)
    /* {,Reverse}Remap map is setup in `gophor.go`, no need to here */
}

func (fs *FileSystem) RemapRequestPath(requestPath *RequestPath) {
    realPath, ok := fs.Remap[requestPath.Relative()]
    if ok {
        requestPath.RemapActual(realPath)
    }
}

func (fs *FileSystem)ReverseRemapRequestPath(requestPath *RequestPath) {
    virtualPath, ok := fs.ReverseRemap[requestPath.Relative()]
    if ok {
        requestPath.RemapVirtual(virtualPath)
    }
}

func (fs *FileSystem) HandleRequest(responder *Responder) *GophorError {
    /* Check if restricted file */
    if isRestrictedFile(responder.Request.Path.Relative()) {
        return &GophorError{ IllegalPathErr, nil }
    }

    /* Remap RequestPath if necessary */
    fs.RemapRequestPath(responder.Request.Path)

    /* Get filesystem stat, check it exists! */
    stat, err := os.Stat(responder.Request.Path.Absolute())
    if err != nil {
        /* Check file isn't in cache before throwing in the towel */
        fs.CacheMutex.RLock()
        file := fs.CacheMap.Get(responder.Request.Path.Absolute())
        if file == nil {
            fs.CacheMutex.RUnlock()
            return &GophorError{ FileStatErr, err }
        }

        /* It's there! Get contents, unlock and return */
        file.Mutex.RLock()
        gophorErr := file.WriteContents(responder)
        file.Mutex.RUnlock()

        fs.CacheMutex.RUnlock()
        return gophorErr
    }

    switch {
        /* Directory */
        case stat.Mode() & os.ModeDir != 0:
            /* Ignore anything under cgi-bin directory */
            if responder.Request.Path.HasRelPrefix(CgiBinDirStr) {
                return &GophorError{ IllegalPathErr, nil }
            }

            /* Check Gophermap exists */
            gophermapPath := NewRequestPath(responder.Request.Path.RootDir(), responder.Request.Path.JoinRel(GophermapFileStr))
            stat, err = os.Stat(gophermapPath.Absolute())

            if err == nil {
                /* Gophermap exists! If executable try return executed contents, else serve as regular gophermap. */
                gophermapRequest := &Request{ gophermapPath, responder.Request.Parameters }
                responder.Request = gophermapRequest

                if stat.Mode().Perm() & 0100 != 0 {
                    return responder.SafeFlush(executeFile(responder))
                } else {
                    return fs.FetchFile(responder)
                }
            } else {
                /* No gophermap, serve directory listing */
                return listDirAsGophermap(responder, map[string]bool{ gophermapPath.Relative(): true, CgiBinDirStr: true })
            }

        /* Regular file */
        case stat.Mode() & os.ModeType == 0:
            /* If cgi-bin, try return executed contents. Else, fetch regular file */
            if responder.Request.Path.HasRelPrefix(CgiBinDirStr) {
                return responder.SafeFlush(executeCgi(responder))
            } else {
                return fs.FetchFile(responder)
            }

        /* Unsupported type */
        default:
            return &GophorError{ FileTypeErr, nil }
    }
}

func (fs *FileSystem) FetchFile(responder *Responder) *GophorError {
    /* Get cache map read lock then check if file in cache map */
    fs.CacheMutex.RLock()
    file := fs.CacheMap.Get(responder.Request.Path.Absolute())

    if file != nil {
        /* File in cache -- before doing anything get file read lock */
        file.Mutex.RLock()

        /* Check file is marked as fresh */
        if !file.Fresh {
            /* File not fresh! Swap file read for write-lock */
            file.Mutex.RUnlock()
            file.Mutex.Lock()

            /* Reload file contents from disk */
            gophorErr := file.CacheContents()
            if gophorErr != nil {
                /* Error loading contents, unlock all mutex then return error */
                file.Mutex.Unlock()
                fs.CacheMutex.RUnlock()
                return gophorErr
            }

            /* Updated! Swap back file write for read lock */
            file.Mutex.Unlock()
            file.Mutex.RLock()
        }
    } else {
        /* Open file here, to check it exists, ready for file stat
         * and in case file is too big we pass it as a raw response
         */
        fd, err := os.Open(responder.Request.Path.Absolute())
        if err != nil {
            /* Error stat'ing file, unlock read mutex then return error */
            fs.CacheMutex.RUnlock()
            return &GophorError{ FileOpenErr, err }
        }

        /* We need a doctor, stat! */
        stat, err := fd.Stat()
        if err != nil {
            /* Error stat'ing file, unlock read mutext then return */
            fs.CacheMutex.RUnlock()
            return &GophorError{ FileStatErr, err }
        }

        /* Compare file size (in MB) to CacheFileSizeMax. If larger, just send file raw */
        if stat.Size() > fs.CacheFileMax {
            /* Unlock the read mutex, we don't need it where we're going... returning, we're returning. */
            fs.CacheMutex.RUnlock()
            return responder.WriteRaw(fd)
        }

        /* Create new file contents */
        var contents FileContents
        if responder.Request.Path.HasRelSuffix(GophermapFileStr) {
            contents = &GophermapContents{ responder.Request.Path, nil }
        } else {
            contents = &RegularFileContents{ responder.Request.Path, nil }
        }

        /* Create new file wrapper around contents */
        file = &File{ contents, sync.RWMutex{}, true, time.Now().UnixNano() }

        /* File isn't in cache yet so no need to get file lock mutex */
        gophorErr := file.CacheContents()
        if gophorErr != nil {
            /* Error loading contents, unlock read mutex then return error */
            fs.CacheMutex.RUnlock()
            return gophorErr
        }

        /* File not in cache -- Swap cache map read for write lock. */
        fs.CacheMutex.RUnlock()
        fs.CacheMutex.Lock()

        /* Put file in the FixedMap */
        fs.CacheMap.Put(responder.Request.Path.Absolute(), file)

        /* Before unlocking cache mutex, lock file read for upcoming call to .Contents() */
        file.Mutex.RLock()

        /* Swap cache lock back to read */
        fs.CacheMutex.Unlock()
        fs.CacheMutex.RLock()
    }

    /* Write file contents via responder */
    gophorErr := file.WriteContents(responder)
    file.Mutex.RUnlock()

    /* Finally we can unlock the cache map read lock, we are done :) */
    fs.CacheMutex.RUnlock()

    return gophorErr
}

type File struct {
    /* Wraps around the cached contents of a file
     * helping with management.
     */

    Content     FileContents
    Mutex       sync.RWMutex
    Fresh       bool
    LastRefresh int64
}

func (f *File) WriteContents(responder *Responder) *GophorError {
    return f.Content.Render(responder)
}

func (f *File) CacheContents() *GophorError {
    /* Clear current file contents */
    f.Content.Clear()

    /* Reload the file */
    gophorErr := f.Content.Load()
    if gophorErr != nil {
        return gophorErr
    }

    /* Update lastRefresh, set fresh, unset deletion (not likely set) */
    f.LastRefresh = time.Now().UnixNano()
    f.Fresh = true
    return nil
}

/* Start the file monitor! */
func startFileMonitor(sleepTime time.Duration) {
    go func() {
        for {
            /* Sleep so we don't take up all the precious CPU time :) */
            time.Sleep(sleepTime)

            /* Check global file cache freshness */
            checkCacheFreshness()
        }

        /* We shouldn't have reached here */
        Config.SysLog.Fatal("", "FileCache monitor escaped run loop!\n")
    }()
}

/* Check file cache for freshness, deleting files not-on disk */
func checkCacheFreshness() {
    /* Before anything, get cache write lock (in case we have to delete) */
    Config.FileSystem.CacheMutex.Lock()

    /* Iterate through paths in cache map to query file last modified times */
    for path := range Config.FileSystem.CacheMap.Map {
        /* Get file pointer, no need for lock as we have write lock */
        file := Config.FileSystem.CacheMap.Get(path)

        /* If this is a generated file, we skip */
        if isGeneratedType(file) {
            continue
        }

        /* Check file still exists on disk, delete and continue if not */
        stat, err := os.Stat(path)
        if err != nil {
            Config.SysLog.Error("", "Failed to stat file in cache: %s\n", path)
            Config.FileSystem.CacheMap.Remove(path)
            continue
        }

        /* Get file's last modified time */
        timeModified := stat.ModTime().UnixNano()

        /* If the file is marked as fresh, but file on disk is newer, mark as unfresh */
        if file.Fresh && file.LastRefresh < timeModified {
            file.Fresh = false
        }
    }

    /* Done! We can release cache read lock */
    Config.FileSystem.CacheMutex.Unlock()
}

/* Just a helper function to neaten-up checking if file contents is of generated type */
func isGeneratedType(file *File) bool {
    switch file.Content.(type) {
       case *GeneratedFileContents:
           return true
       default:
           return false 
    }
}
