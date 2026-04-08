#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

readonly RELEASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly REPO_ROOT="$(cd "${RELEASE_DIR}/.." && pwd)"
readonly DEFAULT_BUILD_DIR="${REPO_ROOT}/build"
readonly DEFAULT_WORK_DIR="${RELEASE_DIR}/work"
readonly DEFAULT_OUTPUT_DIR="${RELEASE_DIR}/output"
readonly DEFAULT_EXAMPLE_FILE="${REPO_ROOT}/doc/server-config-exmaple.toml"
readonly DEFAULT_LICENSE_FILE="${REPO_ROOT}/LICENSE"
readonly PACKAGE_NAME="wsfs-core"
readonly PACKAGE_DESCRIPTION="Mount or serve Websocket Filesystem"
readonly PACKAGE_HOMEPAGE="https://github.com/Kodecable/wsfs-core"
readonly PACKAGE_MAINTAINER="Kodecable <Kodecable@outlook.com>"
readonly PACKAGE_LICENSE="MIT"

release_die() {
    echo "error: $*" >&2
    exit 1
}

release_require_command() {
    local cmd="$1"
    command -v "${cmd}" >/dev/null 2>&1 || release_die "missing required command: ${cmd}"
}

release_require_file() {
    local path="$1"
    [[ -f "${path}" ]] || release_die "missing required file: ${path}"
}

release_prepare_dir() {
    local path="$1"
    rm -rf "${path}"
    mkdir -p "${path}"
}

release_binary_path() {
    local build_dir="$1"
    local arch="$2"
    echo "${build_dir}/wsfs-linux-${arch}"
}

release_debian_arch() {
    case "$1" in
        386) echo "i386" ;;
        amd64) echo "amd64" ;;
        arm) echo "armhf" ;;
        arm64) echo "arm64" ;;
        *)
            release_die "unsupported Debian package architecture: $1"
            ;;
    esac
}

release_archlinux_arch() {
    case "$1" in
        amd64) echo "x86_64" ;;
        arm) echo "armv7h" ;;
        arm64) echo "aarch64" ;;
        *)
            release_die "unsupported Arch package architecture: $1"
            ;;
    esac
}

release_host_archlinux_arch() {
    case "$(uname -m)" in
        x86_64) echo "x86_64" ;;
        aarch64) echo "aarch64" ;;
        armv7l|armv7h) echo "armv7h" ;;
        *)
            release_die "unsupported host architecture for makepkg: $(uname -m)"
            ;;
    esac
}

release_completion_candidates() {
    case "$(uname -m)" in
        x86_64)
            printf '%s\n' amd64 amd64v3 386
            ;;
        i386|i486|i586|i686)
            printf '%s\n' 386
            ;;
        aarch64)
            printf '%s\n' arm64
            ;;
        armv7l|armv7h)
            printf '%s\n' arm
            ;;
        *)
            release_die "unsupported host architecture for completion generation: $(uname -m)"
            ;;
    esac
}

release_find_completion_binary() {
    local build_dir="$1"
    local arch
    while IFS= read -r arch; do
        local binary
        binary="$(release_binary_path "${build_dir}" "${arch}")"
        if [[ -f "${binary}" ]]; then
            chmod +x "${binary}"
        fi
        if [[ -x "${binary}" ]] && "${binary}" version >/dev/null 2>&1; then
            echo "${binary}"
            return 0
        fi
    done < <(release_completion_candidates)

    release_die "no runnable Linux build artifact found for completion generation in ${build_dir}"
}

release_generate_completions() {
    local binary="$1"
    local out_dir="$2"

    mkdir -p "${out_dir}/bash" "${out_dir}/fish" "${out_dir}/zsh"
    "${binary}" completion bash > "${out_dir}/bash/wsfs"
    "${binary}" completion fish > "${out_dir}/fish/wsfs.fish"
    "${binary}" completion zsh > "${out_dir}/zsh/_wsfs"
}

release_install_file() {
    local src="$1"
    local dst="$2"
    local mode="$3"
    install -D -m "${mode}" "${src}" "${dst}"
}
