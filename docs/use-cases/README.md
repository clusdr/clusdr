# Use cases

Each page here takes one coordination job that services run into, and builds it from clusdr's primitives step by step. They show the moving parts, a timeline of what happens when something fails, working code, and where clusdr's guarantee stops and your code has to take over.

<UseCaseGrid />

## Picking a pattern

| You need | Start with | Primitive |
|---|---|---|
| Exactly one replica doing a job at any time | [Run one active replica](leader-election.md) | Lock, held and renewed |
| A cron that fires on every replica to run once per period | [Run each scheduled job once](scheduled-jobs.md) | Lock per run name, `TryLock` |
| Protection from a holder that paused and came back late | [Fence off a stale writer](fencing.md) | The fencing token on every grant |
| Work divided between hosts that rebalances on its own | [Split work across live hosts](sharding.md) | `Members()` plus Watch |
| A list of workers that are still alive, and a signal when one stops | [Know which workers are alive](worker-liveness.md) | One lease per worker |
| Every host to drop a stale cache entry right after a write | [Broadcast cache invalidation](signals.md) | `Publish` plus Watch on a topic |

The patterns combine. A scheduled job that writes to a database should also be fenced. A sharded service usually wants worker leases too, so a stuck process gives up its shards before the host dies.

## What these pages assume

- A running cluster with a daemon on every host your app runs on. If you do not have one yet, the [tutorial](../guide/) gets you there in five steps.
- Your app talks to the daemon on its own host through an [SDK](../sdk/). The code samples use Go, Python and TypeScript; the calls are the same in Rust and Java.
- Names for locks, leases and topics are 1 to 128 characters of `A–Z a–z 0–9 . _ -`.

clusdr is not a database or a queue. None of these patterns store your data in the cluster: the state you care about stays in your own store, and clusdr decides who may touch it and tells you when that changes. For the full list of what is out of scope, see the [overview](../overview.md#do-not-use-it-when).
