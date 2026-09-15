"""Print cluster and custom events from the local daemon.

    clusdr init && clusdr start --bootstrap
    pip install clusdr
    python3 examples/watch/main.py
"""

from __future__ import annotations

from clusdr import local


def main() -> None:
    with local() as c:
        print("watching (Ctrl-C to stop)", flush=True)
        for ev in c.watch():
            payload = f" {ev.payload!r}" if ev.payload else ""
            print(f"{ev.seq} {ev.type} {ev.source}{payload}", flush=True)


if __name__ == "__main__":
    main()
