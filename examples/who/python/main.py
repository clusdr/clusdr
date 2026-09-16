"""Print cluster membership, then follow Watch until Ctrl-C.

The process is an application. It does not vote. It talks only to the
daemon on this host (CLUSDR_GRPC_ADDR or 127.0.0.1:7947).

    clusdr init && clusdr start --bootstrap
    python3 examples/who/python/main.py
    python3 examples/who/python/main.py --once
"""

from __future__ import annotations

import argparse
import sys
from typing import Any

from clusdr import ClusdrError, local


def main() -> None:
    p = argparse.ArgumentParser(description="Print clusdr members, then watch")
    p.add_argument("--once", action="store_true", help="print members and the leader, then exit")
    args = p.parse_args()

    try:
        c = local()
    except ClusdrError as e:
        print(f"connect failed: {e}", file=sys.stderr)
        raise SystemExit(1) from e

    try:
        print_members(c)
        if args.once:
            return
        print("watching cluster events (Ctrl-C to stop)", file=sys.stderr)
        for ev in c.watch():
            print(format_event(ev), flush=True)
    except KeyboardInterrupt:
        print("stopping", file=sys.stderr)
    finally:
        c.close()


def print_members(c: Any) -> None:
    members = c.members()
    print(f"{'ID':<24} {'ADDRESS':<22} {'STATUS':<8} {'ROLE':<10} LEADER")
    for m in members:
        role = m.role or "voter"
        print(f"{m.id:<24} {m.address:<22} {m.status:<8} {role:<10} {m.leader}")
    leader = c.leader()
    print(f"leader {leader.id} at {leader.address}")


def format_event(ev: Any) -> str:
    if ev.type in {"member.join", "member.left", "leader.changed"}:
        kind = "cluster"
    elif ev.type.startswith("custom."):
        kind = "gossip"
    elif ev.type in {"watch.sync", "watch.gap"}:
        kind = "watch"
    else:
        kind = "bus"
    payload = ""
    if ev.payload:
        try:
            payload = " " + ev.payload.decode("utf-8")
        except UnicodeDecodeError:
            payload = f" {ev.payload!r}"
    return f"{kind} seq={ev.seq} {ev.type} src={ev.source}{payload}"


if __name__ == "__main__":
    main()
