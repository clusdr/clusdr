# Tutorial

A Linux laptop goes from an empty `PATH` to a local three-process cluster and one SDK call. Read the pages **in order**. Each page leaves a process or file the next page uses; skipping ahead makes `join` and `Local()` fail for reasons that look like product bugs.

1. [Install the binary](install.md) — `clusdr` on `PATH`
2. [Start the first member](first-member.md) — identity, Raft, one row in `members`
3. [Grow the cluster](grow.md) — a second voter, then an observer
4. [Watch and publish](watch.md) — the stream, then one deploy signal
5. [Use it from your app](from-your-app.md) — `Local()` / `local()`

Finish the checkpoint at the end of each page before you continue. If a command errors, the page names the fix or links to [Errors](../reference/errors.md).

When you need a real NIC, a container, or Kubernetes, leave the tutorial: [Run on other hosts](other-hosts.md). Guarantees: [Concepts](../concepts/). A flag: [Reference](../reference/).
