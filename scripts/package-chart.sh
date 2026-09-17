#!/usr/bin/env bash
# Package charts/clusdr. Usage: scripts/package-chart.sh <version> [outdir]
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
ver=${1:?usage: package-chart.sh <version> [outdir]}
out=${2:-"$ROOT/dist"}

if ! command -v helm >/dev/null 2>&1; then
	echo "package-chart.sh: need helm" >&2
	exit 1
fi

mkdir -p "$out"
helm package "$ROOT/charts/clusdr" --version "$ver" --app-version "$ver" -d "$out" >&2
tgz="$out/clusdr-${ver}.tgz"
test -f "$tgz"
echo "$tgz"
