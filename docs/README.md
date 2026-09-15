# Clusdr documentation

Clusdr is a single-binary **cluster-awareness runtime**. Applications talk only to a daemon on the same host. That daemon is the Raft member.

```text
Application  →  local SDK  →  local daemon  →  Clusdr cluster
```

API package: **`clusdr.v1alpha1`**. There is no tagged v1 release. TLS is on unless `CLUSDR_TLS=disabled`.

## Guide

Read in order. Each page leaves a cluster you use on the next one.

1. [Install the binary](guide/install.md)
2. [Start the first member](guide/first-member.md)
3. [Grow the cluster](guide/grow.md)
4. [Watch and publish](guide/watch.md)
5. [Use it from your app](guide/from-your-app.md)
6. [Run on other hosts](guide/other-hosts.md)

[Guide hub](guide/).

## SDKs

Applications talk to the local daemon. Not the operator path.

- [Application SDK](sdk/)
- [Go](sdk/go.md)
- [Python](sdk/python.md)

## Then

| If you need | Go here |
|---|---|
| Is this the right tool? | [Overview](overview.md) |
| Process model, ports, disk | [Architecture](architecture.md) |
| What a word means | [Glossary](glossary.md) |
| Guarantees (Raft, presence, observers, …) | [Concepts](concepts/) |
| A flag or RPC | [Reference](reference/) |
| Join token, TLS, dial, locks | [Errors](reference/errors.md) |

Contributor workflow (build, proto, tests) is in [CONTRIBUTING.md](https://github.com/durguto/clusdr/blob/main/CONTRIBUTING.md), not in this tree.
