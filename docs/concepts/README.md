# Concepts

clusdr’s guarantees are here: who is a member, what a crash does, what Raft stores, what Watch delivers. Flags and RPCs live under [Reference](../reference/). The same words appear in the [glossary](../glossary.md); if a tutorial restates a definition in one sentence so you can stay on that page, that is on purpose.

This is not a first-run walkthrough. If you have not started a daemon yet, begin at [Get started](../guide/) and come back when a term needs a precise meaning.

| Page | Question it answers |
|---|---|
| [Daemon and application](daemon.md) | Who is the cluster member? |
| [Membership](membership.md) | Who is in the cluster? |
| [Leadership](leadership.md) | Who commits writes? |
| [Observers](observers.md) | How do I add a replica that does not vote? |
| [Watch](watch.md) | How do I see what changed? |
| [Custom events](events.md) | What is `publish`? |
| [Locks](locks.md) | What is an exclusive name? |
| [Leases](leases.md) | What is a named TTL grant? |
| [Presence](presence.md) | Crash vs reboot vs `join`? |
| [Security](security.md) | How do join tokens and mTLS work? |
| [Consistency](consistency.md) | What is on Raft, what is not? |
| [Kubernetes](kubernetes.md) | Why a node is a Linux host, not etcd-for-kube |

Start from [Architecture](../architecture.md) if you have not read it.
