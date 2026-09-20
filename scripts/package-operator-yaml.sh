#!/usr/bin/env bash
# Build clusdr-crds.yaml, clusdr-operator.yaml, and clusdr-operator-bundle.yaml
# for a release tag. Usage: scripts/package-operator-yaml.sh <version> [outdir]
# Does not include a sample ClusdrCluster. Not a Helm chart.
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
ver=${1:?usage: package-operator-yaml.sh <version> [outdir]}
out=${2:-"$ROOT/dist"}

mkdir -p "$out"

cp "$ROOT/config/crd/clusdr.io_clusdrclusters.yaml" "$out/clusdr-crds.yaml"
cp "$out/clusdr-crds.yaml" "$out/clusdr-crds-${ver}.yaml"

emit() {
	first=1
	for f in "$@"; do
		if [[ $first -eq 1 ]]; then
			first=0
		else
			printf '\n---\n'
		fi
		sed -e '1{/^---[[:space:]]*$/d;}' -e '${/^---[[:space:]]*$/d;}' "$f"
		printf '\n'
	done
}

{
	echo "# clusdr-operator ${ver}. Apply after clusdr-crds.yaml."
	echo "# Pin/fallback: https://clusdr.io/download/clusdr-operator.yaml"
	emit \
		"$ROOT/config/operator/namespace.yaml" \
		"$ROOT/config/operator/rbac.yaml" \
		"$ROOT/config/operator/deployment.yaml"
} | sed "s|image: durguto/clusdr-operator:.*|image: durguto/clusdr-operator:${ver}|" >"$out/clusdr-operator.yaml"

cp "$out/clusdr-operator.yaml" "$out/clusdr-operator-${ver}.yaml"

{
	echo "# clusdr-operator-bundle ${ver}. CRDs, then RBAC + Deployment."
	echo "# https://clusdr.io/download/clusdr-operator-bundle.yaml"
	emit \
		"$ROOT/config/crd/clusdr.io_clusdrclusters.yaml" \
		"$ROOT/config/operator/namespace.yaml" \
		"$ROOT/config/operator/rbac.yaml" \
		"$ROOT/config/operator/deployment.yaml"
} | sed "s|image: durguto/clusdr-operator:.*|image: durguto/clusdr-operator:${ver}|" >"$out/clusdr-operator-bundle.yaml"

cp "$out/clusdr-operator-bundle.yaml" "$out/clusdr-operator-bundle-${ver}.yaml"

if ! grep -q "image: durguto/clusdr-operator:${ver}" "$out/clusdr-operator.yaml"; then
	echo "package-operator-yaml.sh: image tag ${ver} missing" >&2
	exit 1
fi
if ! grep -q "image: durguto/clusdr-operator:${ver}" "$out/clusdr-operator-bundle.yaml"; then
	echo "package-operator-yaml.sh: bundle image tag ${ver} missing" >&2
	exit 1
fi
if ! grep -q '^kind: CustomResourceDefinition$' "$out/clusdr-operator-bundle.yaml"; then
	echo "package-operator-yaml.sh: bundle must start with the CRD" >&2
	exit 1
fi
if grep -q '^kind: CustomResourceDefinition$' "$out/clusdr-operator.yaml"; then
	echo "package-operator-yaml.sh: operator.yaml is the pin without CRDs" >&2
	exit 1
fi
if grep -q '^kind: ClusdrCluster$' "$out/clusdr-operator.yaml" "$out/clusdr-operator-bundle.yaml"; then
	echo "package-operator-yaml.sh: sample ClusdrCluster must not be in the YAML" >&2
	exit 1
fi

crd_line=$(grep -n '^kind: CustomResourceDefinition$' "$out/clusdr-operator-bundle.yaml" | head -1 | cut -d: -f1)
dep_line=$(grep -n '^kind: Deployment$' "$out/clusdr-operator-bundle.yaml" | head -1 | cut -d: -f1)
if [[ -z "${crd_line}" || -z "${dep_line}" || "${crd_line}" -ge "${dep_line}" ]]; then
	echo "package-operator-yaml.sh: CRDs must come before the Deployment" >&2
	exit 1
fi

echo "$out/clusdr-crds.yaml"
echo "$out/clusdr-operator.yaml"
echo "$out/clusdr-operator-bundle.yaml"
