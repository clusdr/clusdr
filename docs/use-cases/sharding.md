# Split work across live hosts

You have a fixed set of shards (partitions of a topic, tenants, ranges of keys) and want every host to own a fair share, with a dead host's shards moving to the others on their own. Every daemon holds the same member list and streams the same changes, so each host can compute the whole assignment by itself and they all agree, with no coordinator in between.

<UseMeta />

## The idea

1. Read the alive members with `Members()`.
2. Assign each shard to a member with a deterministic function of the shard and the member ids. Every host runs the same function on the same list and gets the same answer.
3. Each host works on the shards assigned to its own id.
4. Watch `member.*`. When the alive set changes, compute again and start or stop the shards that moved.

<UseFigure name="sharding" fig="Fig. A" caption="Twelve shards on three hosts. host-3 died, so its four shards moved to host-1 and host-2 (in orange). Shards that were already on a live host stayed where they were." />

Membership here is the set of **daemons**, so the unit of work is a host, not a process. If you run several workers per host, split that host's shards between them locally.

## Rendezvous hashing

Rendezvous (highest random weight) hashing gives each shard to the member with the highest `hash(shard, member)`. When a member goes, only its own shards move; when one comes back, it takes back about `1/n` of the shards and nothing else changes.

```go
import "hash/fnv"

func owner(shard string, alive []string) string {
	var best string
	var top uint64
	for _, id := range alive {
		h := fnv.New64a()
		h.Write([]byte(shard + "/" + id))
		if w := h.Sum64(); best == "" || w > top {
			best, top = id, w
		}
	}
	return best
}
```

Use any stable hash, but use the same one on every host and in every language you deploy.

## How a host knows its own id

The app does not join the cluster, so it does not have a member id of its own. Give it the id of its host's daemon the same way you configure that daemon: if `node.id` comes from the host name or from a Kubernetes downward API field, pass the same value to the app (for example as `NODE_ID`). Compare it with `Member.ID`.

## The loop

```go
package main

import (
	"context"
	"log"
	"os"
	"sort"

	"github.com/clusdr/clusdr/sdk"
)

var shards = []string{"s0", "s1", "s2", "s3", "s4", "s5", "s6", "s7", "s8", "s9", "s10", "s11"}

func main() {
	self := os.Getenv("NODE_ID") // same value as this host's node.id
	c, err := clusdr.Local()
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()
	ctx := context.Background()

	ch, err := c.Watch(ctx, clusdr.WithEventTypes("member.join", "member.dead", "member.left"))
	if err != nil {
		log.Fatal(err)
	}
	for ev := range ch {
		// the stream opens with a snapshot (seq 0) and then watch.sync:
		// skip the snapshot rows, recompute at the sync and on every change
		if ev.Type == "watch.gap" || (ev.Seq == 0 && ev.Type != "watch.sync") {
			continue
		}
		members, err := c.Members(ctx)
		if err != nil {
			continue
		}
		var alive []string
		for _, m := range members {
			if m.Status == "alive" {
				alive = append(alive, m.ID)
			}
		}
		sort.Strings(alive)
		var mine []string
		for _, s := range shards {
			if owner(s, alive) == self {
				mine = append(mine, s)
			}
		}
		apply(mine) // start shards that are new to this host, stop the rest
	}
}

func apply(mine []string) {}
```

The membership events are a trigger; the list itself comes from `Members()`. That way the assignment is always computed from the whole current list, and a dropped or merged event cannot leave a host with a stale view. `watch.sync` follows every snapshot, so the loop also recomputes once at start-up and once after every reconnect.

## What a failover looks like

<UseTimeline name="rebalance" fig="Seq. A" caption="host-3 loses power. Its presence lease runs out after 3 seconds and every host's Watch delivers member.dead. host-1 and host-2 each recompute and pick up the shards that hash to them now." />

The delay between a crash and `member.dead` is the presence TTL, 3 seconds by default ([presence](../concepts/presence.md)). A raised TTL means slower failover, not fewer false alarms from reboots: a rebooted host with the same `data.dir` comes back as `member.join` with the same id and takes back the same shards.

## When two hosts disagree for a moment

Each host computes from its own daemon's view, and those views change a little apart in time. For a short window two hosts can both think they own a shard, or neither can. Plan for both:

- **Two owners.** If a shard must never be worked on twice, also take a lock named after it (`shard.s7`) before starting, and pass the token to its writes ([fencing](fencing.md)). The hash decides who should own it; the lock makes sure only one does.
- **No owner.** The shard pauses until the next event. With presence at 3 seconds this is short, and it resolves on its own.

A host that is cut off from the majority keeps its old view until it rejoins. Its daemon's `Members()` can be stale on a partitioned follower, so shards that need strict ownership should use the per-shard lock above.

## Failure cases

| What happens | Event | Result |
|---|---|---|
| Host crashes or loses power | `member.dead` after the presence TTL | Its shards move to the rest; it stays listed as `dead` |
| Host reboots with the same disk | `member.join` with the same id | It takes back the same shards |
| Host is removed with `clusdr leave` | `member.left`, gone from the list | Its shards move for good |
| New host joins | `member.join` with a new id | It takes about `1/n` of the shards |
| An app's Watch stream drops | Snapshot and `watch.sync` again on reconnect | The loop recomputes from scratch |

## Related

- [Membership](../concepts/membership.md) and [Presence](../concepts/presence.md) for what alive and dead mean
- [Watch](../concepts/watch.md) for the snapshot, `watch.sync` and reconnect rules
- [Know which workers are alive](worker-liveness.md), for process-level liveness inside a host
