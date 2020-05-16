#!/bin/sh

set -e

PROJECT='gophor'
VERSION="$(cat 'gophor.go' | grep -E '^\s*GophorVersion' | sed -e 's|\s*GophorVersion = \"||' -e 's|\"\s*$||')"
GOVERSION="$(go version | sed -e 's|^go version go||' -e 's|\s.*$||')"
LOGFILE='build.log'
OUTDIR="build-${VERSION}"

silent() {
    "$@" >> "$LOGFILE" 2>&1
}

build_for() {
    local archname="$1" toolchain="$2" os="$3" arch="$4"
    shift 4
    if [ "$arch" = 'arm' ]; then
        local armversion="$1"
        shift 1
    fi

    echo "Building for ${os} ${archname}..."
    local filename="${OUTDIR}/${PROJECT}_${os}_${archname}"
    CGO_ENABLED=1 CC="$toolchain-gcc" GOOS="$os" GOARCH="$arch" GOARM="$armversion" silent go build -trimpath -o "$filename" "$@"
    if [ "$?" -ne 0 ]; then
        echo "Failed!"
        return 1
    fi

    echo "Attempting to compress ${filename}..."
    cp "$filename" "${filename}.topack"

    if (silent upx --best "${filename}.topack") && (silent upx -t "${filename}.topack"); then
        mv "${filename}.topack" "$filename"
    else
        rm "${filename}.topack"
    fi

    echo ""
}

echo "PLEASE BE WARNED THIS SCRIPT IS WRITTEN FOR A VOID LINUX (MUSL) BUILD ENVIRONMENT"
echo "YOUR CC TOOLCHAIN LOCATIONS MAY DIFFER"
echo "IF THE SCRIPT FAILS, CHECK THE OUTPUT OF: ${LOGFILE}"
echo ""

# Clean logfile
rm -f "$LOGFILE"

# Clean and recreate directory
rm -rf "$OUTDIR"
mkdir -p "$OUTDIR"

# Build time :)

# Linux
build_for '386'      'i686-linux-musl'         'linux' '386'     -buildmode 'pie'     -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

build_for 'amd64'    'x86_64-linux-musl'       'linux' 'amd64'   -buildmode 'pie'     -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

build_for 'armv5'    'arm-linux-musleabi'      'linux' 'arm' '5' -buildmode 'pie'     -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

build_for 'armv5hf'  'arm-linux-musleabihf'    'linux' 'arm' '5' -buildmode 'pie'     -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

build_for 'armv6'    'arm-linux-musleabi'      'linux' 'arm' '6' -buildmode 'pie'     -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

build_for 'armv6hf'  'arm-linux-musleabihf'    'linux' 'arm' '6' -buildmode 'pie'     -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

build_for 'armv7lhf' 'armv7l-linux-musleabihf' 'linux' 'arm' '7' -buildmode 'pie'     -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

build_for 'arm64'    'aarch64-linux-musl'      'linux' 'arm64'   -buildmode 'pie'     -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

build_for 'mips'     'mips-linux-musl'         'linux' 'mips'    -buildmode 'default' -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

build_for 'mipshf'   'mips-linux-muslhf'       'linux' 'mips'    -buildmode 'default' -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

build_for 'mipsle'   'mipsel-linux-musl'       'linux' 'mipsle'  -buildmode 'default' -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

build_for 'mipslehf' 'mipsel-linux-muslhf'     'linux' 'mipsle'  -buildmode 'default' -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

build_for 'ppc64le'  'powerpc64le-linux-musl'  'linux' 'ppc64le' -buildmode 'pie'     -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

# Netbsd
build_for '386'      'i686-linux-musl'         'netbsd'  '386'     -buildmode 'default' -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

build_for 'amd64'    'x86_64-linux-musl'       'netbsd'  'amd64'   -buildmode 'default' -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

build_for 'armv5'    'arm-linux-musleabi'      'netbsd'  'arm' '5' -buildmode 'default' -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

build_for 'armv5hf'  'arm-linux-musleabihf'    'netbsd'  'arm' '5' -buildmode 'default' -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

build_for 'armv6'    'arm-linux-musleabi'      'netbsd'  'arm' '6' -buildmode 'default' -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

build_for 'armv6hf'  'arm-linux-musleabihf'    'netbsd'  'arm' '6' -buildmode 'default' -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

build_for 'armv7lhf' 'armv7l-linux-musleabihf' 'netbsd'  'arm' '7' -buildmode 'default' -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

build_for 'arm64'    'aarch64-linux-musl'      'netbsd'  'arm64'   -buildmode 'default' -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

# Openbsd
build_for '386'      'i686-linux-musl'         'openbsd'  '386'     -buildmode 'default' -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

build_for 'amd64'    'x86_64-linux-musl'       'openbsd'  'amd64'   -buildmode 'default' -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

build_for 'armv5'    'arm-linux-musleabi'      'openbsd'  'arm' '5' -buildmode 'default' -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

build_for 'armv5hf'  'arm-linux-musleabihf'    'openbsd'  'arm' '5' -buildmode 'default' -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

build_for 'armv6'    'arm-linux-musleabi'      'openbsd'  'arm' '6' -buildmode 'default' -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

build_for 'armv6hf'  'arm-linux-musleabihf'    'openbsd'  'arm' '6' -buildmode 'default' -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

build_for 'armv7lhf' 'armv7l-linux-musleabihf' 'openbsd'  'arm' '7' -buildmode 'default' -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

build_for 'arm64'    'aarch64-linux-musl'      'openbsd'  'arm64'   -buildmode 'default' -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

# Freebsd
build_for '386'      'i686-linux-musl'         'freebsd'  '386'     -buildmode 'default' -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

build_for 'amd64'    'x86_64-linux-musl'       'freebsd'  'amd64'   -buildmode 'default' -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

build_for 'armv5'    'arm-linux-musleabi'      'freebsd'  'arm' '5' -buildmode 'default' -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

build_for 'armv5hf'  'arm-linux-musleabihf'    'freebsd'  'arm' '5' -buildmode 'default' -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

build_for 'armv6'    'arm-linux-musleabi'      'freebsd'  'arm' '6' -buildmode 'default' -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

build_for 'armv6hf'  'arm-linux-musleabihf'    'freebsd'  'arm' '6' -buildmode 'default' -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

build_for 'armv7lhf' 'armv7l-linux-musleabihf' 'freebsd'  'arm' '7' -buildmode 'default' -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

build_for 'arm64'    'aarch64-linux-musl'      'freebsd'  'arm64'   -buildmode 'default' -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

# Dragonfly
build_for 'amd64'    'x86_64-linux-musl'       'dragonfly'  'amd64'   -buildmode 'default' -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

# Macos
build_for '386'      'i686-linux-musl'         'darwin'  '386'     -buildmode 'default' -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'

build_for 'amd64'    'x86_64-linux-musl'       'darwin'  'amd64'   -buildmode 'default' -a -tags 'netgo' -ldflags '-s -w -extldflags "-static"'
