package main

import (
    "regexp"
    "strings"
    "log"
)

const (
    FileRemapSeparatorStr = " -> "
)

type FileRemap struct {
    Regex      *regexp.Regexp
    Template   string
}

/* Pre-compile gophermap file string regex */
func compileGophermapCheckRegex() *regexp.Regexp {
    return regexp.MustCompile(`^(|.+/|.+\.)gophermap$`)
}

/* Pre-compile cgi-bin path string regex */
func compileCgiBinCheckRegex() *regexp.Regexp {
    return regexp.MustCompile(`^`+Config.CgiDir+`(|/.*)$`)
}

/* Compile a user supplied new line separated list of regex statements */
func compileUserRestrictedRegex(restrictions string) []*regexp.Regexp {
    /* Return slice */
    restrictedRegex := make([]*regexp.Regexp, 0)

    /* Split the user supplied regex statements by new line */
    for _, expr := range strings.Split(restrictions, "\n") {
        /* Empty expression, skip */
        if len(expr) == 0 {
            continue
        }

        /* Try compile regex */
        regex, err := regexp.Compile(expr)
        if err != nil {
            log.Fatalf("Failed compiling user supplied regex: %s\n", expr)
        }

        /* Append restricted */
        restrictedRegex = append(restrictedRegex, regex)
        Config.SysLog.Info("", "Compiled restricted: %s\n", expr)
    }

    return restrictedRegex
}

/* Compile a user supplied new line separated list of file remap regex statements */
func compileUserRemapRegex(remaps string) []*FileRemap {
    /* Return slice */
    fileRemaps := make([]*FileRemap, 0)

    /* Split the user supplied regex statements by new line */
    for _, expr := range strings.Split(remaps, "\n") {
        /* Empty expression, skip */
        if len(expr) == 0 {
            continue
        }

        /* Split into alias and remap string (MUST BE LENGTH 2) */
        split := strings.Split(expr, FileRemapSeparatorStr)
        if len(split) != 2 {
            continue
        }

        /* Try compile regex */
        regex, err := regexp.Compile("(?m)"+strings.TrimPrefix(split[0], "/")+"$")
        if err != nil {
            log.Fatalf("Failed compiling user supplied regex: %s\n", expr)
        }

        /* Append file remapper */
        fileRemaps = append(fileRemaps, &FileRemap{ regex, split[1] })
        Config.SysLog.Info("", "Compiled remap: %s\n", expr)
    }

    return fileRemaps
}

/* Check if file path is gophermap */
func isGophermap(path string) bool {
    return Config.RgxGophermap.MatchString(path)
}

/* Check if file path within cgi-bin */
func withinCgiBin(absPath string) bool {
    return Config.RgxCgiBin.MatchString(absPath)
}
