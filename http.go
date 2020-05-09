package main

import (
    "io"
    "bufio"
    "bytes"
)

type HttpStripWriter struct {
    /* Wrapper to bufio.Writer that reads a predetermined amount into a buffer
     * then parses the buffer for valid HTTP headers and status code, deciding
     * whether to strip these headers or returning with an HTTP status code.
     */

    /* We set underlying write function with a variable, so that each call
     * to .Write() doesn't have to perform a check every time whether we need
     * to keep checking for headers to skip.
     */
    WriteFunc  func([]byte) (int, error)
    Writer     *bufio.Writer

    SkipBuffer []byte
    SkipIndex  int

    Err        *GophorError
}

func NewHttpStripWriter(writer *bufio.Writer) *HttpStripWriter {
    w := &HttpStripWriter{}
    w.Writer = writer
    w.WriteFunc = w.WriteCheckForHeaders
    w.SkipBuffer = make([]byte, Config.SkipPrefixBufSize)
    w.SkipIndex = 0
    return w
}

func (w *HttpStripWriter) Size() int {
    /* Size of the skip buffer */
    return len(w.SkipBuffer)
}

func (w *HttpStripWriter) Available() int {
    /* How much space have we got left in the skip buffer */
    return w.Size() - w.SkipIndex
}

func (w *HttpStripWriter) AddToSkipBuffer(data []byte) int {
    /* Figure out how much data we need to add */
    toAdd := w.Available()
    if len(data) < toAdd {
        toAdd = len(data)
    }

    /* Add the data to the skip buffer! */
    copy(w.SkipBuffer[w.SkipIndex:], data[:toAdd])
    w.SkipIndex += toAdd
    return toAdd
}

func (w *HttpStripWriter) ParseHttpHeaderSection() (bool, bool) {
    /* Check if this is a valid HTTP header section and determine from status if we should continue */
    validHeaderSection, shouldContinue := false, true
    for _, header := range bytes.Split(w.SkipBuffer, []byte(DOSLineEnd)) {
        header = bytes.ToLower(header)

        if bytes.Contains(header, []byte("content-type: ")) {
            /* This whole header section is now _valid_ */
            validHeaderSection = true
        } else if bytes.Contains(header, []byte("status: ")) {
            /* Try parse status code */
            statusStr := string(bytes.Split(bytes.TrimPrefix(header, []byte("status: ")), []byte(" "))[0])

            if statusStr == "200" {
                /* We ignore this */
                continue
            }

            /* Any other values indicate error, we should not continue writing */
            shouldContinue = false

            /* Try parse error code */
            errorCode := CgiStatusUnknownErr
            switch statusStr {
                case "400":
                    errorCode = CgiStatus400Err
                case "401":
                    errorCode = CgiStatus401Err
                case "403":
                    errorCode = CgiStatus403Err
                case "404":
                    errorCode = CgiStatus404Err
                case "408":
                    errorCode = CgiStatus408Err
                case "410":
                    errorCode = CgiStatus410Err
                case "500":
                    errorCode = CgiStatus500Err
                case "501":
                    errorCode = CgiStatus501Err
                case "503":
                    errorCode = CgiStatus503Err
            }

            /* Set struct error */
            w.Err = &GophorError{ errorCode, nil }
        }
    }

    return validHeaderSection, shouldContinue
}

func (w *HttpStripWriter) WriteSkipBuffer() (bool, error) {
    defer func() {
        w.SkipIndex = 0
    }()

    /* First try parse the headers, determine what to do next */
    validHeaders, shouldContinue := w.ParseHttpHeaderSection()

    if validHeaders {
        /* Valid headers, we don't bother writing. Return whether
         * shouldContinue whatever value that may be.
         */
        return shouldContinue, nil
    }

    /* Default is to write skip buffer contents. shouldContinue only
     * means something as long as we have valid headers.
     */
    _, err := w.Writer.Write(w.SkipBuffer[:w.SkipIndex])
    return true, err
}

func (w *HttpStripWriter) FinishUp() *GophorError {
    /* If SkipBuffer still has contents, in case of data written being less
     * than w.Size() --> check this data for HTTP headers to strip, parse
     * any status codes and write this content with underlying writer if
     * necessary.
     */
    if w.SkipIndex > 0 {
        w.WriteSkipBuffer()
    }

    /* Return HttpStripWriter error code if set */
    return w.Err
}


func (w *HttpStripWriter) Write(data []byte) (int, error) {
    /* Write using whatever write function is currently set */
    return w.WriteFunc(data)
}

func (w *HttpStripWriter) WriteRegular(data []byte) (int, error) {
    /* Regular write function */
    return w.Writer.Write(data)
}

func (w *HttpStripWriter) WriteCheckForHeaders(data []byte) (int, error) {
    split := bytes.Split(data, []byte(DOSLineEnd+DOSLineEnd))
    if len(split) == 1 {
        /* Try add these to skip buffer */
        added := w.AddToSkipBuffer(data)

        if added < len(data) {
            defer func() {
                /* Having written skipbuffer after this if clause, set write to regular */
                w.WriteFunc = w.WriteRegular
            }()

            doContinue, err := w.WriteSkipBuffer()
            if !doContinue {
                return len(data), io.EOF
            } else if err != nil {
                return added, err
            }

            /* Write remaining data not added to skip buffer */
            count, err := w.Writer.Write(data[added:])
            if err != nil {
                return added+count, err
            }
        }

        return len(data), nil
    } else {
        defer func() {
            /* No use for skip buffer after this clause, set write to regular */
            w.WriteFunc = w.WriteRegular
            w.SkipIndex = 0
        }()

        /* Try add what we can to skip buffer */
        added := w.AddToSkipBuffer(append(split[0], []byte(DOSLineEnd+DOSLineEnd)...))

        /* Write skip buffer data if necessary, check if we should continue */
        doContinue, err := w.WriteSkipBuffer()
        if !doContinue {
            return len(data), io.EOF
        } else if err != nil {
            return added, err
        }

        /* Write remaining data not added to skip buffer */
        count, err := w.Writer.Write(data[added:])
        if err != nil {
            return added+count, err
        }

        return len(data), nil
    }
}
