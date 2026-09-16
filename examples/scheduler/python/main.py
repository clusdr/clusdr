"""Hold one exclusive cluster lock and “dispatch” work.

Run two copies with different --holder values. Only one replica holds
"scheduler" at a time. close() unlocks.

    clusdr init && clusdr start --bootstrap
    python3 examples/scheduler/python/main.py --holder replica-a
    python3 examples/scheduler/python/main.py --holder replica-b

try_lock returns None when the name is taken (no current-holder object).
"""

from __future__ import annotations

import argparse
import sys
import time
from typing import Any

from clusdr import ClusdrError, local


def main() -> None:
    p = argparse.ArgumentParser(description="Exclusive scheduler lock")
    p.add_argument("--name", default="scheduler", help="lock name (1–128, A–Z a–z 0–9 . _ -)")
    p.add_argument("--holder", default="", help="lock identity; empty generates sdk-<hex>")
    p.add_argument("--work", type=float, default=3.0, help="seconds to hold the lock while dispatching")
    p.add_argument("--wait", type=float, default=1.0, help="pause after a failed try or a completed shift")
    args = p.parse_args()

    try:
        c = local(holder=args.holder)
    except ClusdrError as e:
        print(f"connect failed: {e}", file=sys.stderr)
        raise SystemExit(1) from e

    print(f"scheduler replica lock={args.name} work={args.work}s", file=sys.stderr, flush=True)
    try:
        while True:
            shift(c, args.name, args.work, args.wait)
    except KeyboardInterrupt:
        print("stopping", file=sys.stderr)
    except ClusdrError as e:
        print(f"shift failed: {e}", file=sys.stderr)
        raise SystemExit(1) from e
    finally:
        c.close()


def shift(c: Any, name: str, work: float, wait: float) -> None:
    lk = c.try_lock(name, ttl=15, timeout=5)
    if lk is None:
        print("waiting held_by=another replica", file=sys.stderr, flush=True)
        time.sleep(wait)
        return

    until = lk.deadline.isoformat() if lk.deadline else "?"
    print(
        f"held name={lk.name} holder={lk.holder} token={lk.token} until={until}",
        file=sys.stderr,
        flush=True,
    )
    print(f"dispatch job=rollout fence={lk.token} holder={lk.holder}", flush=True)
    try:
        time.sleep(work)
    except KeyboardInterrupt:
        c.unlock(name)
        raise
    c.unlock(name)
    print(f"released name={name}", file=sys.stderr, flush=True)
    time.sleep(wait)


if __name__ == "__main__":
    main()
