package main

import (
    "strings"
)

/* Formats an info-text footer from string. Add last line as we use the footer to contain last line (regardless if empty) */
func formatGophermapFooter(text string, useSeparator bool) []byte {
    ret := make([]byte, 0)
    if text != "" {
        ret = append(ret, buildInfoLine("")...)
        if useSeparator {
            ret = append(ret, buildInfoLine(buildLineSeparator(Config.PageWidth))...)
        }
        for _, line := range strings.Split(text, "\n") {
            ret = append(ret, buildInfoLine(line)...)
        }
    }
    return append(ret, []byte(LastLine)...)
}

/* Replace standard replacement strings */
func replaceStrings(str string, connHost *ConnHost) []byte {
    /* We only replace the actual host and port values */
    split := strings.Split(str, Tab)
    if len(split) < 4 {
        return []byte(str)
    }

    split[2] = strings.Replace(split[2], ReplaceStrHostname, connHost.Name(), -1)
    split[3] = strings.Replace(split[3], ReplaceStrPort, connHost.Port(), -1)

    /* Return slice */
    b := make([]byte, 0)

    /* Recombine the slices and add the removed tabs */
    splitLen := len(split)
    for i := 0; i < splitLen-1; i += 1 {
        split[i] += Tab
        b = append(b, []byte(split[i])...)
    }
    b = append(b, []byte(split[splitLen-1])...)

    return b
}

/* Replace new-line characters */
func replaceNewLines(str string) string {
    return strings.Replace(str, "\n", "", -1)
}
