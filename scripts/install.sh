#!/bin/sh
# https://clusdr.io/install.sh
# Detects OS/arch, downloads the matching release archive, installs clusdr to BINDIR.
set -eu

REPO="odurgut/clusdr"
PROJECT="clusdr"
DEFAULT_ORIGIN="https://clusdr.io/download"
GITHUB_LATEST="https://github.com/${REPO}/releases/latest/download"

usage() {
	echo "Install ${PROJECT} (Linux amd64/arm64)." >&2
	echo "Usage: curl -fsSL https://clusdr.io/install.sh | sh" >&2
	echo "Env: BINDIR (default /usr/local/bin), PREFIX, CLUSDR_VERSION, CLUSDR_DOWNLOAD_ORIGIN" >&2
	exit 2
}

die() {
	echo "install.sh: $*" >&2
	exit 1
}

need() {
	command -v "$1" >/dev/null 2>&1 || die "need $1"
}

os_arch() {
	os=$(uname -s | tr '[:upper:]' '[:lower:]')
	arch=$(uname -m)
	case "$os" in
	linux) ;;
	darwin)
		die "macOS is not a supported host. Clusdr ships Linux amd64/arm64. Use ghcr.io/odurgut/clusdr, or build from source."
		;;
	mingw* | msys* | cygwin* | windows*)
		die "Windows is not a supported host. Clusdr ships Linux amd64/arm64."
		;;
	*)
		die "unsupported OS $(uname -s). Need Linux amd64 or arm64."
		;;
	esac
	case "$arch" in
	x86_64 | amd64) arch=amd64 ;;
	aarch64 | arm64) arch=arm64 ;;
	*)
		die "unsupported architecture $(uname -m). Need amd64 or arm64."
		;;
	esac
	echo "${os} ${arch}"
}

resolve_version() {
	if [ -n "${CLUSDR_VERSION:-}" ]; then
		echo "${CLUSDR_VERSION#v}"
		return
	fi
	tmp=$(mktemp)
	if download "${origin}/latest.json" "$tmp" 2>/dev/null; then
		ver=$(sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$tmp" | head -n 1)
		rm -f "$tmp"
		[ -n "$ver" ] && echo "${ver#v}" && return
	else
		rm -f "$tmp"
	fi
	need curl
	ver=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" |
		tr ',' '\n' |
		sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' |
		head -n 1)
	[ -n "$ver" ] || die "could not resolve latest version"
	echo "${ver#v}"
}

# download URL dest — tries origin, then GitHub latest if origin is the domain.
download() {
	url=$1
	dest=$2
	if curl -fsSL "$url" -o "$dest"; then
		return 0
	fi
	if [ "${origin}" = "${DEFAULT_ORIGIN}" ] && [ -n "${github_fallback:-}" ]; then
		file=$(basename "$url")
		curl -fsSL "${github_fallback}/${file}" -o "$dest"
		return
	fi
	return 1
}

checksum_of() {
	file=$1
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$file" | awk '{print $1}'
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$file" | awk '{print $1}'
	else
		die "need sha256sum or shasum"
	fi
}

[ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ] && usage

need curl
need tar
need mktemp
need uname

# shellcheck disable=SC2046
set -- $(os_arch)
os=$1
arch=$2

if [ -n "${BINDIR:-}" ]; then
	bindir=$BINDIR
elif [ -n "${PREFIX:-}" ]; then
	bindir="${PREFIX}/bin"
else
	bindir=/usr/local/bin
fi

pinned=
[ -n "${CLUSDR_VERSION:-}" ] && pinned=1

if [ -n "${CLUSDR_DOWNLOAD_ORIGIN:-}" ]; then
	origin=${CLUSDR_DOWNLOAD_ORIGIN}
	github_fallback=
elif [ -n "$pinned" ]; then
	origin="https://github.com/${REPO}/releases/download/v${CLUSDR_VERSION#v}"
	github_fallback=
else
	origin=$DEFAULT_ORIGIN
	github_fallback=$GITHUB_LATEST
fi

version=$(resolve_version)
archive="${PROJECT}_${version}_${os}_${arch}.tar.gz"
sums=checksums.txt

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

download "${origin}/${archive}" "${work}/${archive}" || die "failed to download ${archive} from ${origin}"
download "${origin}/${sums}" "${work}/${sums}" || die "failed to download ${sums} from ${origin}"

want=$(awk -v f="$archive" '$2 == f || $2 == "*"f || $2 ~ "/"f"$" { print $1; exit }' "${work}/${sums}")
[ -n "$want" ] || die "no checksum for ${archive} in ${sums}"
got=$(checksum_of "${work}/${archive}")
[ "$got" = "$want" ] || die "checksum mismatch for ${archive}: got ${got} want ${want}"

tar -xzf "${work}/${archive}" -C "$work"
[ -x "${work}/${PROJECT}" ] || die "archive did not contain ${PROJECT}"

mkdir -p "$bindir"
if [ -w "$bindir" ]; then
	cp "${work}/${PROJECT}" "${bindir}/${PROJECT}"
	chmod 755 "${bindir}/${PROJECT}"
else
	need sudo
	sudo cp "${work}/${PROJECT}" "${bindir}/${PROJECT}"
	sudo chmod 755 "${bindir}/${PROJECT}"
fi

echo "installed ${bindir}/${PROJECT} (${version})"
"${bindir}/${PROJECT}" version
