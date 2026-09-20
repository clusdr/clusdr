# `clusdr version`

`clusdr version` prints build version, commit, and time. Use it to confirm which binary is on `PATH` before you form a cluster or file a bug. It works without a daemon.

A release binary prints the tag. A source build prints `dev` unless ldflags set `Version`. If you mix a `dev` binary with release peers, you are off the [compatibility](../compatibility.md) train.

## Synopsis

```bash
clusdr version
```

Example shape: `clusdr 0.2.1 (commit: …, built: …)` or `clusdr dev (commit: …, built: …)`.

## See also

- [CLI index](./)
- [Compatibility](../compatibility.md)
