# Run one active replica

Run three copies of a service for availability, but let only one of them do the work that must not happen twice: consuming a feed, driving a reconciler, sending the daily report. The others stand by and one of them takes over within seconds of the active replica going away.

<UseMeta />

## The idea

Every replica asks the daemon on its own host for the same lock, `jobs.active`. The cluster agrees on a single holder through Raft, so it does not matter which host each replica runs on or which daemon it asks. The replica that gets the lock is active. The others block in `Lock()` until the name is free.

<UseFigure name="leader-election" fig="Fig. A" caption="Each replica calls the daemon on its own host. The daemons agree on one holder for jobs.active; the active replica's lock floats above it, the standbys wait in Lock()." />

This is application leadership, not the cluster's Raft leader. `Leader()` tells you which **daemon** leads Raft, which says nothing about where your service runs. Use a lock named after the job you want done once.

## What happens when the active replica dies

The SDK renews a held lock in the background about every TTL/3, so a 15 second lock is renewed about every 5 seconds. When the process exits cleanly, `Close()` releases the lock at once and a standby takes over immediately. When the process is killed or the host loses power, nothing releases it: the renewals stop, the TTL runs out, the leader expires the grant and emits `lock.expired`, and the next waiting `Lock()` call returns.

<UseTimeline name="failover" fig="Seq. A" caption="Replica a is killed while active. No renewals arrive, so the grant expires after at most one TTL, and replica b, which has been waiting in Lock(), is granted the name with a higher token." />

So the worst-case failover time is about one TTL. A shorter TTL fails over faster and renews more often; 15 seconds is the default and a good start. The minimum is 1ms and the maximum 24 hours ([limits](../reference/limits.md)).

## The loop

The active replica works in small units and checks before each one that its lock has not run out. If the deadline is close, it stops and goes back to waiting. A replica that lost its lock must not keep working on the assumption that it still holds it.

::: code-group

```go [Go]
package main

import (
	"context"
	"log"
	"time"

	"github.com/clusdr/clusdr/sdk"
)

const name = "jobs.active"

func main() {
	for {
		standby()
	}
}

// standby waits for the lock, works while it holds it, then lets go.
func standby() {
	c, err := clusdr.Local()
	if err != nil {
		log.Print(err) // daemon not up yet
		time.Sleep(2 * time.Second)
		return
	}
	defer c.Close() // releases the lock if we still hold it

	// Lock blocks until granted or the context ends; loop and ask again.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	lk, err := c.Lock(ctx, name, 15*time.Second)
	cancel()
	if err != nil {
		return // still held by someone else
	}
	log.Printf("active, token %d", lk.Token)

	for time.Until(lk.Deadline()) > 5*time.Second {
		doOneUnit(lk.Token) // pass the token to anything you write
	}
	log.Print("lock about to lapse, standing down")
}

func doOneUnit(token uint64) { time.Sleep(time.Second) }
```

```python [Python]
import time
from datetime import datetime, timedelta, timezone

from clusdr import ClusdrError, local

NAME = "jobs.active"


def standby() -> None:
    try:
        c = local()
    except ClusdrError as e:
        print(e)  # daemon not up yet
        time.sleep(2)
        return
    try:
        try:
            lk = c.lock(NAME, ttl=15, timeout=30)  # waits up to 30s
        except ClusdrError:
            return  # still held by someone else
        print("active, token", lk.token)
        margin = timedelta(seconds=5)
        while lk.deadline and lk.deadline - datetime.now(timezone.utc) > margin:
            do_one_unit(lk.token)  # pass the token to anything you write
        print("lock about to lapse, standing down")
    finally:
        c.close()  # releases the lock if we still hold it


def do_one_unit(token: int) -> None:
    time.sleep(1)


while True:
    standby()
```

```ts [TypeScript]
import { ClusdrError, local } from "clusdr";

const NAME = "jobs.active";

async function standby(): Promise<void> {
  const c = await local();
  try {
    let lk;
    try {
      lk = await c.lock(NAME, 15, 30); // waits up to 30s
    } catch (err) {
      if (err instanceof ClusdrError) return; // still held by someone else
      throw err;
    }
    console.log("active, token", lk.token);
    while (lk.deadline && lk.deadline.getTime() - Date.now() > 5_000) {
      await doOneUnit(lk.token); // pass the token to anything you write
    }
    console.log("lock about to lapse, standing down");
  } finally {
    await c.close(); // releases the lock if we still hold it
  }
}

async function doOneUnit(token: number): Promise<void> {
  await new Promise((r) => setTimeout(r, 1_000));
}

for (;;) await standby();
```

:::

Three details in that loop matter:

- **A fresh client per term.** Each pass connects, competes, and closes. After a lost lock the client may still have the old grant cached; closing it and connecting again is the clean way back into the line.
- **The deadline check.** `Deadline()` moves forward each time a renewal succeeds. If renewals stop reaching the cluster (a partition, an overloaded daemon), it stops moving, and the loop stands down before the cluster hands the name to someone else.
- **Small units of work.** The check only runs between units. A unit that can take longer than the margin needs its writes [fenced](fencing.md), because the lock may lapse in the middle of it.

## Failure cases

| What happens | What clusdr does | What your replica should do |
|---|---|---|
| Active process exits cleanly | `Close()` unlocks; a waiting `Lock()` returns at once | Nothing |
| Active process is killed, host loses power | Renewals stop; after the TTL, `lock.expired`, next waiter is granted | Nothing; the standby takes over |
| Active process freezes (GC, swap, SIGSTOP) | Same as killed, from the cluster's point of view | On wake, see the deadline has passed and stand down; [fence](fencing.md) writes so a late one is refused |
| Active host is cut off from the majority | Renewals fail on its side; the majority expires the grant and grants it elsewhere | The deadline stops moving; the check stands the replica down |
| No Raft leader for a while | `Lock()` and renewals return `Unavailable` and the SDK retries | Keep waiting; the deadline check covers the active side |
| Replica runs on an [observer](../concepts/observers.md) | Lock calls fail with `FailedPrecondition` | Run lock takers next to a voter, or `clusdr promote` that host |

## Watching who is active

Other services can follow the handover without competing for the lock. `lock.expired` carries the lock name in `source` and the old holder and token in its payload:

```go
ch, _ := c.Watch(ctx, clusdr.WithEventTypes("lock.expired"))
for ev := range ch {
	if ev.Source == "jobs.active" {
		log.Printf("active replica lapsed: %s", ev.Payload) // {"holder":"…","token":7}
	}
}
```

A clean `Unlock` or `Close` does not emit `lock.expired`; it is only for grants that ran out. From the CLI, `clusdr locks` shows the current holder and token at any time.

## Checklist

- [ ] One lock name per job, named after the job: `jobs.active`, `billing.reconciler`.
- [ ] Give each replica a readable holder with `WithHolder` (Go) or `holder=` so `clusdr locks` shows which replica is active.
- [ ] Work in units shorter than the deadline margin, and check the deadline between them.
- [ ] Pass `lk.Token` with every write the active replica makes, and [fence](fencing.md) them.
- [ ] Run lock takers on hosts whose daemon is a voter.

## Related

- [Locks](../concepts/locks.md) for the full API, TTL rules and table limits
- [Fence off a stale writer](fencing.md), the other half of this pattern
- [Run each scheduled job once](scheduled-jobs.md), when work is periodic rather than continuous
