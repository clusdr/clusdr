"""Agents signalling each other over the local daemon — no broker.

Publish is 1-hop gossip, not Raft. It is not a queue. A subscriber
that connects later will not see old custom events.

    clusdr init && clusdr start --bootstrap
    pip install clusdr
    python3 examples/agent/python/main.py --mode listen
    python3 examples/agent/python/main.py --mode emit --from mapper
"""

from __future__ import annotations

import argparse
import os
import sys
import threading

from clusdr import ClusdrError, local

TOPIC = "agent.task"


def main() -> None:
    p = argparse.ArgumentParser(description="Publish or listen for agent.task on the clusdr bus")
    p.add_argument("--mode", choices=("listen", "emit", "both"), default="both")
    p.add_argument("--from", dest="source", default=f"pid-{os.getpid()}", help="payload from= field when emitting")
    p.add_argument("--every", type=float, default=2.0, help="emit interval in seconds")
    args = p.parse_args()

    try:
        c = local()
    except ClusdrError as e:
        print(f"connect failed: {e}", file=sys.stderr)
        raise SystemExit(1) from e

    stop = threading.Event()
    emitter: threading.Thread | None = None
    try:
        if args.mode in {"emit", "both"}:
            emitter = threading.Thread(target=emit_loop, args=(c, args.source, args.every, stop), daemon=True)
            emitter.start()
            print(f"emitting custom.{TOPIC} every {args.every}s from={args.source}", flush=True)
        if args.mode in {"listen", "both"}:
            print(f"listening for custom.{TOPIC} (Ctrl-C to stop)", flush=True)
            for ev in c.watch(topics=[TOPIC]):
                body = ev.payload.decode("utf-8", errors="replace") if ev.payload else ""
                ts = ev.timestamp.strftime("%H:%M:%S") if ev.timestamp else ""
                print(f"{ts} seq={ev.seq} {ev.type} src={ev.source} {body}", flush=True)
        else:
            print("emitting (Ctrl-C to stop)", flush=True)
            while not stop.wait(0.25):
                pass
    except KeyboardInterrupt:
        print("stopping", file=sys.stderr)
    finally:
        stop.set()
        c.close()


def emit_loop(c: object, source: str, every: float, stop: threading.Event) -> None:
    n = 0
    while not stop.wait(every):
        n += 1
        try:
            c.publish(TOPIC, {"from": source, "n": n, "pid": os.getpid()})  # type: ignore[attr-defined]
        except ClusdrError as e:
            print(f"publish failed: {e}", file=sys.stderr)
            return
        print(f"sent n={n} from={source}", flush=True)


if __name__ == "__main__":
    main()
