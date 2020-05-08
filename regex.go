package main

import (
    "regexp"
    "strings"
    "log"
)

func compileCmdParseRegex() *regexp.Regexp {
    return regexp.MustCompile(` `)
}

/* Compile a user supplied new line separated list of regex statements */
func compileUserRestrictedRegex(restrictions string) []*regexp.Regexp {
    /* Return slice */
    restricted := make([]*regexp.Regexp, 0)

    /* Split the user supplied regex statements by new line */
    for _, expr := range strings.Split(restrictions, "\n") {
        /* Empty expression, skip */
        if len(expr) == 0 {
            continue
        }

        /* Try compile regex then append */
        regex, err := regexp.Compile(expr)
        if err != nil {
            log.Fatalf("Failed compiling user supplied regex: %s\n", expr)
        }
        restricted = append(restricted, regex)
    }

    return restricted
}

/* Iterate through restricted file expressions, check if file _is_ restricted */
func isRestrictedFile(name string) bool {
    for _, regex := range Config.RestrictedFiles {
        if regex.MatchString(name) {
            return true
        }
    }
    return false
}

/* Iterate through restricted command expressions, check if command _is_ restricted */
func isRestrictedCommand(name string) bool {
    for _, regex := range Config.RestrictedCommands {
        if regex.MatchString(name) {
            return true
        }
    }
    return false
}
