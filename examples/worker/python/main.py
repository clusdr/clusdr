"""Hold a named lease the way a shard worker would.

A lease does not wait. If the name is taken, grant fails immediately
(unlike scheduler, which retries try_lock). close() revokes.

    clusdr init && clusdr start --bootstrap
    python3 examples/worker/python/main.py --name shard-7 --owner worker-a
    python3 examples/worker/python/main.py --name shard-7 --owner worker-b
"""

from __future__ import annotations

import argparse
import sys
import time

from clusdr import ClusdrError, local


def main() -> None:
    p = argparse.ArgumentParser(description="Hold a named shard lease")
    p.add_argument("--name", default="shard-7", help="lease name")
    p.add_argument("--owner", default="", help="lease owner; empty generates sdk-<hex>")
    p.add_argument("--ttl", type=float, default=15.0, help="lease TTL in seconds (daemon default if <=0)")
    args = p.parse_args()

    try:
        c = local(holder=args.owner)
    except ClusdrError as e:
        print(f"connect failed: {e}", file=sys.stderr)
        raise SystemExit(1) from e

    try:
        ls = c.lease(args.name, ttl=args.ttl)
        until = ls.deadline.isoformat() if ls.deadline else "?"
        print(
            f"granted name={ls.name} owner={ls.owner} token={ls.token} until={until}",
            file=sys.stderr,
            flush=True,
        )
        print("holding shard (Ctrl-C to stop; close revokes)", file=sys.stderr, flush=True)
        while True:
            time.sleep(3600)
    except KeyboardInterrupt:
        print("stopping", file=sys.stderr)
    except ClusdrError as e:
        print(f"lease failed: {e}", file=sys.stderr)
        raise SystemExit(1) from e
    finally:
        c.close()


if __name__ == "__main__":
    main()
