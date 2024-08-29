#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail
BuildName="wsfs"
BuildTime=`date -u +'%Y%m%d%H%M'`
BuildMode="debug"
BuildVersion="unknown"

cd "$(dirname $0)"
BasePath="$(pwd)"

help() {
    echo "Usage: ./build.sh [OPTIONS]"
    echo "Build project"
    echo ""
    echo "   -c         Clean and exit"
    echo "   -o OS      Target OS"
    echo "              'all', 'windows', 'linux' (default 'all')"
    echo "   -a ARCH    Target Arch"
    echo "              'all', 'arm', 'arm64', '386', 'amd64' (default 'all')"
    echo "   -m MODE    'release' or 'debug'"
    echo "   -v VERSION Short commit ID (if available) or 'unknown' when not set"
    echo ""
    echo "Configed BuildName: '$BuildName'"
}

if command -v "git" >/dev/null 2>&1 && [ -d ".git" ]; then
    BuildVersion="commit.$(git rev-parse --short --verify HEAD 2>/dev/null)" || BuildVersion="unknown"
    if [ "$BuildVersion" != "unknown" ] && [ ! -z "$(git status --porcelain=v1)" ]; then
       BuildVersion="$BuildVersion+"
    fi
fi

clean(){
    if [ -d "$BasePath/build" ]; then
        rm -rf "$BasePath/build"
    fi
}

gen_buildinfo(){
    cd "$BasePath/src/buildinfo"
    if [ -e gen.go~ ]; then
        rm -f gen.go
    else
        mv gen.go gen.go~
    fi
    echo "// Code generated by build.sh. DO NOT EDIT." >> gen.go
    echo "package buildinfo" >> gen.go
    echo "" >> gen.go
    echo "const (" >> gen.go
    echo "	Version string = \"$BuildVersion\"" >> gen.go
    echo "	Mode    string = \"$BuildMode\"" >> gen.go
    echo "	Time    string = \"$BuildTime\"" >> gen.go
    echo ")" >> gen.go
    echo "" >> gen.go
}

gen_minifiedwebui() {
    cd "$BasePath/src/internal/server/webui/"
    if [ -e resources~ ]; then
        rm -rf resources
    else
        mv resources resources~
    fi
    mkdir resources resources/js resources/css
    cp -r resources~/img resources/img
    cd resources~/js
    for file in ./*; do
        minify -o ../../resources/js/${file} ${file} > /dev/null
    done
    cd ../..
    cd resources~/css
    for file in ./*; do
        minify -o ../../resources/css/${file} ${file} > /dev/null
    done
}

clean_minifiedwebui(){
    cd "$BasePath/src/internal/server/webui/"
    rm -r resources
    mv resources~ resources
}

clean_buildinfo(){
    cd "$BasePath/src/buildinfo"
    rm gen.go
    mv gen.go~ gen.go
}

do_hooks() {
    gen_buildinfo
    if [ "$BuildMode" == "release" ]; then
        gen_minifiedwebui
    fi
}

clean_hooks(){
    clean_buildinfo
    if [ "$BuildMode" == "release" ]; then
        clean_minifiedwebui
    fi
}

build(){
    cd "$BasePath/src"
    EXENAME="../build/$BuildName-$1-$2"
    if [ "$1" = "windows" ]; then
        EXENAME="$EXENAME.exe"
    fi
    if [ "$BuildMode" == "release" ]; then
        CGO_ENABLED=0 GOOS=$1 GOARCH=$2 go build -trimpath -ldflags "-s -w" -o "$EXENAME"
    else
        CGO_ENABLED=0 GOOS=$1 GOARCH=$2 go build -o "$EXENAME"
    fi
    echo "build $1 $2 done"
}

buildAllArch(){
    case $1 in
        linux)
            build linux 386
            build linux amd64
            build linux arm
            build linux arm64
            ;;
        windows)
            build windows 386
            build windows amd64
            ;;
        *)
            echo "You need to specify a specific arch for this os"
            ;;
    esac
}

OS="all"
ARCH="all"

while getopts "o:a:m:v:hc" o; do
    case "${o}" in
        o)OS=${OPTARG};;
        a)ARCH=${OPTARG};;
        m)BuildMode=${OPTARG};;
        v)BuildVersion=${OPTARG};;
        h)
            help
            exit 0
            ;;
        c)
            clean
            exit 0
            ;;
    esac
done

do_hooks
trap clean_hooks EXIT

if [ "$ARCH" == "all" ]; then
    if [ "$OS" == "all" ]; then
        buildAllArch windows
        buildAllArch linux
    else
        buildAllArch "$OS"
    fi
else
    if [ "$OS" == "all" ]; then
        build windows "$ARCH"
        build linux "$ARCH"
    else
        build "$OS" "$ARCH"
    fi
fi
