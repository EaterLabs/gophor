#!/bin/sh

CC=x86_64-linux-musl-gcc CGO_ENABLED=1 go build -trimpath -buildmode 'pie' -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'