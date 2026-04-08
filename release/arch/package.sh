#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=../common.sh
source "${SCRIPT_DIR}/../common.sh"

usage() {
    cat <<'EOF'
Usage: release/arch/package.sh -v VERSION [OPTIONS]

Build native Arch Linux packages with makepkg from build/wsfs-linux-* artifacts.

Options:
  -v VERSION    Package version, required
  -a ARCHES     Comma-separated build arches: amd64,arm,arm64 or all
  -b DIR        Build artifact directory (default ../build)
  -w DIR        Work directory (default release/work/arch)
  -o DIR        Output directory (default release/output)
  -e FILE       Example config file (default ../doc/server-config-exmaple.toml)
  -l FILE       License file (default ../LICENSE)
EOF
}

version=""
arch_arg="all"
build_dir="${DEFAULT_BUILD_DIR}"
work_dir="${DEFAULT_WORK_DIR}/arch"
output_dir="${DEFAULT_OUTPUT_DIR}"
example_file="${DEFAULT_EXAMPLE_FILE}"
license_file="${DEFAULT_LICENSE_FILE}"

while getopts ":v:a:b:w:o:e:l:h" opt; do
    case "${opt}" in
        v) version="${OPTARG}" ;;
        a) arch_arg="${OPTARG}" ;;
        b) build_dir="${OPTARG}" ;;
        w) work_dir="${OPTARG}" ;;
        o) output_dir="${OPTARG}" ;;
        e) example_file="${OPTARG}" ;;
        l) license_file="${OPTARG}" ;;
        h)
            usage
            exit 0
            ;;
        :)
            release_die "option -${OPTARG} requires an argument"
            ;;
        \?)
            release_die "unknown option: -${OPTARG}"
            ;;
    esac
done
shift "$((OPTIND - 1))"

[[ $# -eq 0 ]] || release_die "unexpected argument(s): $*"
[[ -n "${version}" ]] || release_die "missing required -v VERSION"
[[ "$(id -u)" -ne 0 ]] || release_die "makepkg must not run as root"

release_require_command makepkg
release_require_command install
release_require_command find

release_require_file "${example_file}"
release_require_file "${license_file}"

mkdir -p "${output_dir}"
release_prepare_dir "${work_dir}"

declare -a build_arches=()
if [[ "${arch_arg}" == "all" ]]; then
    host_archlinux_arch="$(release_host_archlinux_arch)"
    case "${host_archlinux_arch}" in
        x86_64) build_arches=(amd64) ;;
        aarch64) build_arches=(arm64) ;;
        armv7h) build_arches=(arm) ;;
        *) release_die "unsupported host architecture: ${host_archlinux_arch}" ;;
    esac
else
    IFS=',' read -r -a build_arches <<< "${arch_arg}"
fi

completion_binary="$(release_find_completion_binary "${build_dir}")"
completion_dir="${work_dir}/completions"
release_generate_completions "${completion_binary}" "${completion_dir}"
host_archlinux_arch="$(release_host_archlinux_arch)"

for build_arch in "${build_arches[@]}"; do
    case "${build_arch}" in
        amd64|arm|arm64)
            ;;
        *)
            release_die "unsupported Arch build arch: ${build_arch}"
            ;;
    esac

    package_arch="$(release_archlinux_arch "${build_arch}")"
    [[ "${package_arch}" == "${host_archlinux_arch}" ]] || release_die \
        "cannot build Arch package for ${build_arch} on host ${host_archlinux_arch}; use a matching runner"

    binary_path="$(release_binary_path "${build_dir}" "${build_arch}")"
    release_require_file "${binary_path}"

    stage_dir="${work_dir}/${build_arch}"
    pkg_dest="${stage_dir}/pkgdest"
    makepkg_home="${stage_dir}/home"

    release_prepare_dir "${stage_dir}"
    mkdir -p "${pkg_dest}" "${makepkg_home}"

    release_install_file "${binary_path}" "${stage_dir}/wsfs" 0755
    release_install_file "${example_file}" "${stage_dir}/exmaple.toml" 0644
    release_install_file "${completion_dir}/bash/wsfs" "${stage_dir}/wsfs.bash" 0644
    release_install_file "${completion_dir}/fish/wsfs.fish" "${stage_dir}/wsfs.fish" 0644
    release_install_file "${completion_dir}/zsh/_wsfs" "${stage_dir}/_wsfs" 0644
    release_install_file "${license_file}" "${stage_dir}/LICENSE" 0644

    cat > "${stage_dir}/PKGBUILD" <<EOF
pkgname=${PACKAGE_NAME}
pkgver=${version}
pkgrel=1
pkgdesc='${PACKAGE_DESCRIPTION}'
arch=('${package_arch}')
url='${PACKAGE_HOMEPAGE}'
license=('${PACKAGE_LICENSE}')
options=('!debug')
depends=()
optdepends=('fuse3: for no-direct mount')
backup=('etc/wsfs/exmaple.toml')
source=('wsfs' 'exmaple.toml' 'wsfs.bash' 'wsfs.fish' '_wsfs' 'LICENSE')
sha256sums=('SKIP' 'SKIP' 'SKIP' 'SKIP' 'SKIP' 'SKIP')

package() {
    install -Dm755 "\${srcdir}/wsfs" "\${pkgdir}/usr/bin/wsfs"
    install -Dm644 "\${srcdir}/exmaple.toml" "\${pkgdir}/etc/wsfs/exmaple.toml"
    install -Dm644 "\${srcdir}/wsfs.bash" "\${pkgdir}/usr/share/bash-completion/completions/wsfs"
    install -Dm644 "\${srcdir}/wsfs.fish" "\${pkgdir}/usr/share/fish/vendor_completions.d/wsfs.fish"
    install -Dm644 "\${srcdir}/_wsfs" "\${pkgdir}/usr/share/zsh/site-functions/_wsfs"
    install -Dm644 "\${srcdir}/LICENSE" "\${pkgdir}/usr/share/licenses/${PACKAGE_NAME}/LICENSE"
}
EOF

    echo "packaging arch/${package_arch} from wsfs-linux-${build_arch}"
    (
        cd "${stage_dir}"
        HOME="${makepkg_home}" \
        PKGDEST="${pkg_dest}" \
        makepkg --force --nodeps --clean --cleanbuild --noconfirm >/dev/null
    )

    built_package="$(find "${pkg_dest}" -maxdepth 1 -type f -name 'wsfs-core-[0-9]*.pkg.tar.zst' ! -name '*-debug-*' | head -n 1)"
    [[ -n "${built_package}" ]] || release_die "makepkg did not produce a package for ${build_arch}"

    output_path="${output_dir}/wsfs-core-package-arch-${build_arch}.pkg.tar.zst"
    install -m 0644 "${built_package}" "${output_path}"
    echo "created package: ${output_path}"
done
