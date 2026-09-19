"""Watch the local daemon: membership snapshot, a worker lease, then the bus.

The process is an application. It does not vote. It talks only to the
daemon on this host (CLUSDR_GRPC_ADDR or 127.0.0.1:7947).

Python locates PEMs in CLUSDR_DATA_DIR or ~/.clusdr unless TLS is off.

    clusdr init && clusdr start --bootstrap
    pip install clusdr
    python3 examples/watch/python/main.py --name edge-1

In another terminal:

    clusdr publish ping '{"from":"cli"}'
    python3 examples/watch/python/main.py --name edge-2

custom.* is gossip (not Raft, not replayed). member.dead is crash (still listed).
member.left is clusdr leave (gone).
close() revokes the worker lease.
"""

from __future__ import annotations

import argparse
import os
import sys
from typing import Any

from clusdr import ClusdrError, local


def main() -> None:
    p = argparse.ArgumentParser(description="Watch clusdr from an application process")
    p.add_argument(
        "--name",
        default=f"pid-{os.getpid()}",
        help="lease name suffix (worker.<name>); two processes must not share it",
    )
    p.add_argument(
        "--once",
        action="store_true",
        help="print members and exit (no lease, no watch)",
    )
    args = p.parse_args()

    try:
        c = local()
    except ClusdrError as e:
        print(f"connect failed: {e}", file=sys.stderr)
        print(
            "need a running daemon and TLS certs in ~/.clusdr "
            "(or CLUSDR_TLS=disabled on daemon and client)",
            file=sys.stderr,
        )
        raise SystemExit(1) from e

    lease_name = f"worker.{args.name}"
    try:
        print_members(c)
        if args.once:
            return
        ls = c.lease(lease_name, ttl=15)
        until = ls.deadline.isoformat() if ls.deadline else "?"
        print(
            f"lease {ls.name} owner={ls.owner} token={ls.token} until {until}",
            flush=True,
        )
        c.publish(
            "hello",
            {"worker": args.name, "pid": os.getpid(), "lease": lease_name},
        )
        print("watching (Ctrl-C to stop); try: clusdr publish ping '{\"from\":\"cli\"}'", flush=True)
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
    if ev.type in {"member.join", "leader.changed"}:
        kind = "cluster"
    elif ev.type == "member.dead":
        kind = "dead"
    elif ev.type == "member.left":
        kind = "left"
    elif ev.type.startswith("custom."):
        kind = "gossip"
    elif ev.type in {"watch.sync", "watch.gap"}:
        kind = "watch"
    else:
        kind = "bus"
    ts = ev.timestamp.strftime("%H:%M:%S") if ev.timestamp else ""
    payload = ""
    if ev.payload:
        try:
            payload = " " + ev.payload.decode("utf-8")
        except UnicodeDecodeError:
            payload = f" {ev.payload!r}"
    return f"{kind} {ts} seq={ev.seq} {ev.type} src={ev.source}{payload}"


if __name__ == "__main__":
    main()
