# Get started

Clusdr is a cluster-awareness runtime: a daemon on each host is the Raft member, and your application talks only to the daemon on the same machine. These docs teach that model, then let you look up a flag without rereading the story.

Two doors: learn in order, or look up a flag.

| You are | Open |
|---|---|
| New, and you want a working cluster on this machine | [Tutorial](guide/) |
| Looking up a flag, RPC, error, or SDK type | [Reference](reference/) (or search) |

The tutorial is five pages, in order: install → first member → grow → watch → first SDK call. Starting in Reference skips the processes those pages leave running, so commands look like they failed.

## After that

| Goal | Page |
|---|---|
| Real NICs, the Docker image, or which health check to trust | [Run on other hosts](guide/other-hosts.md) |
| The same host model on a Kubernetes node | [Run on Kubernetes](guide/kubernetes.md) |
| Why it is built this way, and the trade-offs | [Overview](overview.md) · [Architecture](architecture.md) · [Concepts](concepts/) |

Contributor build, proto, and tests live in [CONTRIBUTING.md](https://github.com/clusdr/clusdr/blob/main/CONTRIBUTING.md), not in this tree — that file is the source-build path, not how operators install a cluster.
