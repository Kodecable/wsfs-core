#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=../common.sh
source "${SCRIPT_DIR}/../common.sh"

usage() {
    cat <<'EOF'
Usage: release/debian/package.sh -v VERSION [OPTIONS]

Build native Debian packages from build/wsfs-linux-* artifacts.

Options:
  -v VERSION    Package version, required
  -a ARCHES     Comma-separated build arches: 386,amd64,arm,arm64 or all
  -b DIR        Build artifact directory (default ../build)
  -w DIR        Work directory (default release/work/debian)
  -o DIR        Output directory (default release/output)
  -e FILE       Example config file (default ../doc/server-config-exmaple.toml)
  -l FILE       License file (default ../LICENSE)
EOF
}

version=""
arch_arg="all"
build_dir="${DEFAULT_BUILD_DIR}"
work_dir="${DEFAULT_WORK_DIR}/debian"
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

release_require_command dpkg-deb
release_require_command install
release_require_command du

release_require_file "${example_file}"
release_require_file "${license_file}"

mkdir -p "${output_dir}"
release_prepare_dir "${work_dir}"

declare -a build_arches=()
if [[ "${arch_arg}" == "all" ]]; then
    build_arches=(386 amd64 arm arm64)
else
    IFS=',' read -r -a build_arches <<< "${arch_arg}"
fi

completion_binary="$(release_find_completion_binary "${build_dir}")"
completion_dir="${work_dir}/completions"
release_generate_completions "${completion_binary}" "${completion_dir}"

for build_arch in "${build_arches[@]}"; do
    case "${build_arch}" in
        386|amd64|arm|arm64)
            ;;
        amd64v3)
            release_die "amd64v3 is not packaged for Debian: native Debian metadata has no distinct architecture for it"
            ;;
        *)
            release_die "unsupported Debian build arch: ${build_arch}"
            ;;
    esac

    binary_path="$(release_binary_path "${build_dir}" "${build_arch}")"
    release_require_file "${binary_path}"

    deb_arch="$(release_debian_arch "${build_arch}")"
    stage_dir="${work_dir}/${deb_arch}"
    pkg_root="${stage_dir}/${PACKAGE_NAME}"
    control_dir="${pkg_root}/DEBIAN"

    release_prepare_dir "${stage_dir}"

    release_install_file "${binary_path}" "${pkg_root}/usr/bin/wsfs" 0755
    release_install_file "${example_file}" "${pkg_root}/etc/wsfs/exmaple.toml" 0644
    release_install_file "${completion_dir}/bash/wsfs" "${pkg_root}/usr/share/bash-completion/completions/wsfs" 0644

    mkdir -p "${control_dir}"
    installed_size="$(du -sk "${pkg_root}" | awk '{print $1}')"

    cat > "${control_dir}/control" <<EOF
Package: ${PACKAGE_NAME}
Version: ${version}-1
Section: net
Priority: optional
Architecture: ${deb_arch}
Maintainer: ${PACKAGE_MAINTAINER}
Installed-Size: ${installed_size}
Suggests: ca-certificates, fuse | fuse3
Homepage: ${PACKAGE_HOMEPAGE}
Description: ${PACKAGE_DESCRIPTION}
EOF

    cat > "${control_dir}/conffiles" <<'EOF'
/etc/wsfs/exmaple.toml
EOF

    chmod 0644 "${control_dir}/control" "${control_dir}/conffiles"

    output_path="${output_dir}/wsfs-core-package-debian-${deb_arch}.deb"
    echo "packaging debian/${deb_arch} from wsfs-linux-${build_arch}"
    dpkg-deb --build --root-owner-group "${pkg_root}" "${output_path}" >/dev/null
    echo "created package: ${output_path}"
done
