# Run each scheduled job once

Every replica of your service carries the same cron table, so the schedule survives any one of them going down. That also means the 03:00 job fires three times. Give each run its own lock name and let the replicas race for it: one claims the run, the others see it taken and skip.

<UseMeta />

## The idea

Name the lock after the job **and** the period it covers: `nightly.20261016`, `invoices.2026-10`, `digest.20261016T09`. Every replica computes the same name from the same tick, so they all compete for one name per run. `TryLock` makes one attempt and never waits, which is what you want here: a replica that loses has nothing to wait for.

<UseFigure name="scheduled-jobs" fig="Fig. A" caption="Three replicas fire the same tick and compute the same run name. Replica a claims nightly.20261016; b and c find it held and skip." />

A lock is mutual exclusion **while it is held**, not a record that something happened. Once the winner unlocks, the name is free again, so a replica whose clock runs a few seconds late could claim the same run after it finished. Two things close that gap:

1. Write a row for the run in the store the job already writes to, inside the job's own transaction.
2. After claiming the lock, read that row first. If the run is already recorded, unlock and skip.

<UseTimeline name="cron" fig="Seq. A" caption="Replica a wins the 03:00 run and writes a run row when it is done. Replica c's clock is late; it claims the free name at +14s, reads the row, sees the run is done, and skips." />

## Code

::: code-group

```go [Go]
package main

import (
	"context"
	"log"
	"time"

	"github.com/clusdr/clusdr/sdk"
)

// onTick runs on every replica when the cron fires.
func onTick(c clusdr.Cluster, now time.Time) {
	run := "nightly." + now.UTC().Format("20060102")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	lk, ok, err := c.TryLock(ctx, run, 15*time.Minute)
	if err != nil {
		log.Print(err) // daemon down, observer, bad name: not "taken"
		return
	}
	if !ok {
		log.Printf("%s claimed by %s, skipping", run, lk.Holder)
		return
	}
	defer c.Unlock(context.Background(), run)

	if done(run) { // the run row, read from your own store
		return
	}
	runJob(run, lk.Token) // writes its results and the run row in one transaction
}

func done(run string) bool               { return false }
func runJob(run string, token uint64) {}
```

```python [Python]
from datetime import datetime, timezone

from clusdr import ClusdrError


def on_tick(c, now: datetime) -> None:
    run = "nightly." + now.astimezone(timezone.utc).strftime("%Y%m%d")
    try:
        lk = c.try_lock(run, ttl=15 * 60, timeout=5)
    except ClusdrError as e:
        print(e)  # daemon down, observer, bad name: not "taken"
        return
    if lk is None:
        print(run, "claimed elsewhere, skipping")
        return
    try:
        if done(run):  # the run row, read from your own store
            return
        run_job(run, lk.token)  # writes results and the run row together
    finally:
        c.unlock(run)
```

```ts [TypeScript]
import type { Cluster } from "clusdr";

export async function onTick(c: Cluster, now: Date): Promise<void> {
  const run = "nightly." + now.toISOString().slice(0, 10).replaceAll("-", "");
  const lk = await c.tryLock(run, 15 * 60, 5);
  if (lk === null) {
    console.log(run, "claimed elsewhere, skipping");
    return;
  }
  try {
    if (await done(run)) return; // the run row, read from your own store
    await runJob(run, lk.token); // writes results and the run row together
  } finally {
    await c.unlock(run);
  }
}
```

:::

## Choosing the TTL

The lock is renewed in the background while the job runs, so the TTL is not a limit on how long the job may take. It is how long the run stays claimed after the replica running it **dies**. Pick it with that in mind:

| TTL | After a crash mid-run | Good for |
|---|---|---|
| Short (15s to 1m) | Another replica can claim the run at its next tick, soon after | Jobs that are safe to restart from the top |
| Long (as long as the job) | Nobody retries the run until the TTL is over | Jobs where a second partial run costs more than a late one |

Ticks only happen at the scheduled time, so nobody tries the run again on their own after a crash. If a crashed run should be retried, have the cron fire a few times in the period (for example every 5 minutes from 03:00 to 04:00) and rely on the run row to make the extra ticks cheap.

## Failure cases

| What happens | Result | What to do |
|---|---|---|
| Two ticks at the same moment | One `TryLock` succeeds, the others get `ok == false` | Nothing |
| A late clock ticks after the run finished | It claims the free name, reads the run row, skips | Keep the run row check |
| Winner dies mid-run | The name stays claimed until the TTL; then it is free | Retry ticks in the period, and make the job restartable |
| Winner pauses mid-run past the TTL | A retry tick can claim the run while the first is still alive | [Fence](fencing.md) the job's writes with `lk.Token` |
| `TryLock` returns an error | Not "taken": the daemon is down, or this host is an observer | Log it; the other replicas still claim the run |

## Checklist

- [ ] The run name contains the period, in UTC, so every replica computes the same one.
- [ ] Losers use `TryLock` and skip; nobody waits in `Lock()` for a run.
- [ ] The job records the run in its own store, and the winner checks that record before starting.
- [ ] Writes carry `lk.Token` if a paused winner could overlap a retry.
- [ ] Lock names stay under the 4096-lock table limit. Unlocked names are removed, so this only matters for runs held at the same time.

## Related

- [Run one active replica](leader-election.md), when the work is continuous instead of periodic
- [Fence off a stale writer](fencing.md)
- [Locks](../concepts/locks.md) and the [scheduler example](https://github.com/clusdr/clusdr/tree/main/examples/scheduler) in five languages
