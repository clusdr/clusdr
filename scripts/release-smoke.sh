#!/usr/bin/env bash
# Start one bootstrap daemon, wait for Health, list members, then stop.
# Used by CI and by the Release workflow before publishing.
set -euo pipefail

bin="${1:-./bin/clusdr}"
if [[ ! -x "$bin" ]]; then
  echo "release-smoke: missing binary $bin" >&2
  exit 1
fi

dir=$(mktemp -d)
port=$((18000 + RANDOM % 1000))
pid=""
cleanup() {
  if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
  fi
  rm -rf "$dir"
}
trap cleanup EXIT

export HOME="$dir"
export CLUSDR_DATA_DIR="$dir/data"
export CLUSDR_TLS=disabled
export CLUSDR_GRPC_ADDR="127.0.0.1:${port}"
export CLUSDR_RAFT_ADDR="127.0.0.1:$((port + 1))"
export CLUSDR_NODE_ADDR="127.0.0.1:${port}"
export CLUSDR_CONTROL_SOCKET="$dir/clusdr.sock"
cfg="$dir/clusdr.yaml"

"$bin" init --config "$cfg" >/dev/null
"$bin" start --config "$cfg" --bootstrap >/dev/null &
pid=$!

ok=0
for _ in $(seq 1 50); do
  if "$bin" health --config "$cfg" >/dev/null 2>&1; then
    ok=1
    break
  fi
  if ! kill -0 "$pid" 2>/dev/null; then
    echo "release-smoke: daemon exited before health" >&2
    exit 1
  fi
  sleep 0.2
done
if [[ "$ok" -ne 1 ]]; then
  echo "release-smoke: health did not become ready" >&2
  exit 1
fi

"$bin" health --config "$cfg"
out=$("$bin" members --config "$cfg")
echo "$out"
if ! grep -q alive <<<"$out"; then
  echo "release-smoke: members did not list an alive node" >&2
  exit 1
fi
