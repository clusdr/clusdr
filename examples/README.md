# Examples

Small programs that talk to a **local** daemon. They do not join Raft.

Need a running member first:

```bash
clusdr init
clusdr start --bootstrap
```

TLS stays on. Certs come from `CLUSDR_DATA_DIR` or `~/.clusdr`.

| Program | Language | What it does |
|---|---|---|
| [who](who) | Go | Print members and the leader |
| [scheduler](scheduler) | Go | Hold an exclusive lock, then release |
| [watch](watch) | Python | Print the Watch stream |

From this repository (Go module replace → `./sdk`):

```bash
go run ./examples/who
go run ./examples/scheduler
```

```bash
pip install clusdr
python3 examples/watch/main.py
```

In your own module, `go get github.com/durguto/clusdr/sdk` and copy the Go files. Apps use `Local` / `local()`, not `Dial`.

Guide: [Use it from your app](../docs/guide/from-your-app.md). Errors: [Errors](../docs/reference/errors.md).
