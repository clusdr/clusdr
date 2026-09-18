#!/usr/bin/env bash
# test_watch.sh — Watch RPC end-to-end: two nodes, then a join on the stream.
#
# Starts two nodes (A + B), opens a grpcurl watch stream on A, then
# joins a third node C and verifies that the stream delivers a member.join event.
#
# Prerequisites: bin/clusdr (arm64), grpcurl
# Usage: bash scripts/test_watch.sh
set -euo pipefail

BIN="$(cd "$(dirname "$0")/.." && pwd)/bin/clusdr"
WDIR=$(mktemp -d)
trap 'echo "cleaning up..."; pkill -P $$ 2>/dev/null || true; rm -rf "$WDIR"' EXIT

log() { echo ">> $*"; }

# ── Helper: init + start a node ────────────────────────────────────────────
start_node() {
  local name="$1" grpc="$2" raft="$3" sock="$4" bootstrap="${5:-}"
  local cfg="$WDIR/$name.yaml"
  local datadir="$WDIR/$name"
  mkdir -p "$datadir"

  CLUSDR_GRPC_ADDR="$grpc" \
  CLUSDR_RAFT_ADDR="$raft" \
  CLUSDR_DATA_DIR="$datadir" \
  CLUSDR_CONTROL_SOCKET="$sock" \
    "$BIN" init --config "$cfg" >/dev/null 2>&1

  CLUSDR_GRPC_ADDR="$grpc" \
  CLUSDR_RAFT_ADDR="$raft" \
  CLUSDR_DATA_DIR="$datadir" \
  CLUSDR_CONTROL_SOCKET="$sock" \
    "$BIN" start --config "$cfg" ${bootstrap:+--bootstrap} \
      >> "$WDIR/$name.log" 2>&1 &
  echo $!
}

# ── Start cluster ───────────────────────────────────────────────────────────
log "Starting node A (seed)"
A_PID=$(start_node A 127.0.0.1:7945 127.0.0.1:7946 "$WDIR/a.sock" bootstrap)
sleep 2.5

log "Starting node B"
B_PID=$(start_node B 127.0.0.1:7947 127.0.0.1:7948 "$WDIR/b.sock")
sleep 0.5

log "Joining B → A"
CLUSDR_CONTROL_SOCKET="$WDIR/b.sock" "$BIN" join --config "$WDIR/B.yaml" 127.0.0.1:7945 \
  || { echo "FAIL: join B failed"; exit 1; }
sleep 1.5

log "Members on A:"
CLUSDR_CONTROL_SOCKET="$WDIR/a.sock" "$BIN" members --config "$WDIR/A.yaml" 2>&1

# ── Watch stream ────────────────────────────────────────────────────────────
log "Opening watch stream on A (background)..."
WATCH_OUT="$WDIR/watch.out"
grpcurl -plaintext -d '{}' 127.0.0.1:7945 \
  clusdr.v1alpha1.WatchService/Watch > "$WATCH_OUT" 2>&1 &
GRPCURL_PID=$!
sleep 0.5  # let snapshot events arrive

# ── Join node C and check event ─────────────────────────────────────────────
log "Starting node C"
C_PID=$(start_node C 127.0.0.1:7949 127.0.0.1:7950 "$WDIR/c.sock")
sleep 0.5

log "Joining C → A"
CLUSDR_CONTROL_SOCKET="$WDIR/c.sock" "$BIN" join --config "$WDIR/C.yaml" 127.0.0.1:7945 \
  || { echo "FAIL: join C failed"; exit 1; }
sleep 1

# ── Verify ──────────────────────────────────────────────────────────────────
kill $GRPCURL_PID 2>/dev/null || true

log "Watch output:"
cat "$WATCH_OUT"
echo

if grep -q '"member.join"' "$WATCH_OUT" || grep -q 'member.join' "$WATCH_OUT"; then
  echo "✅ PASS: member.join event received on watch stream"
else
  echo "❌ FAIL: no member.join event in watch stream"
  echo "--- node A log ---"
  cat "$WDIR/A.log" 2>/dev/null | tail -20
  exit 1
fi
