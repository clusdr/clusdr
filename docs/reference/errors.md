# Errors

What operators and apps actually see. This is a lookup page, not a substitute for the [guide](../guide/). Caps that are not bugs live in [Limits](limits.md).

Prefer `clusdr members` when you want to know if the Runtime API is up. `clusdr status` only checks that the Unix socket **file** exists.

## Daemon and dial

| You see | Cause | What to do |
|---|---|---|
| `daemon not running (socket not found: …)` | `clusdr status`: no control socket at that path | Start the daemon, or pass `--config` so the socket path matches. Prefer `clusdr members` |
| `dial daemon at 127.0.0.1:7947: …` / `connection refused` | Nothing listening on the Runtime API | `clusdr start` is not running, or `grpc.addr` is another port. Point CLI `--config` at the same YAML |
| `listen grpc runtime …: bind: address already in use` | Another process owns that port | Second daemon needs its own `grpc.addr`, `raft.addr`, `control_socket`, and `data.dir` ([grow](../guide/grow.md)) |
| `config file "clusdr.yaml" already exists (use --force to overwrite)` | Second `clusdr init` | Keep the existing identity. `--force` replaces config **and** prints a new join token |
| log `node identity not found in store` / hint `run 'clusdr init'` | `start` without `init` | The process runs, but it is not a cluster. Run `init`, then `start --bootstrap` on the seed ([first member](../guide/first-member.md)) |
| Two daemons, shared `data.dir` | BoltDB and certs are per process | Separate directories. Sharing a dir is undefined |

`clusdr status` exit 1 when the socket is missing is intentional for scripts. It is not a readiness probe for Kubernetes unless you control that path ([other hosts](../guide/other-hosts.md)).

## Join

Join talks to the **local** daemon (`RequestJoin`). That daemon then dials `<addr>` (the seed Runtime API).

| You see | Cause | What to do |
|---|---|---|
| `UNAUTHORIZED` / `UNAUTHORIZED: invalid join token` | Token does not match the hash from `init` | Use the plaintext printed **once** by `clusdr init`. Hash is stored; typing a new string will not work ([security](../concepts/security.md)) |
| `join rejected: cluster id mismatch: want "…"` | Both sides set `cluster.id` and they differ | Leave `cluster.id` empty on the joiner, or copy the seed’s id ([grow](../guide/grow.md)) |
| `join: …` / `Unavailable` / `dial leader …` | Seed down, wrong host:port, or no leader to forward to | `<addr>` is Runtime (`grpc.addr`), not Raft. Wait until `clusdr members` on the seed shows a leader |
| `this node has no CA key; join via a seed node` | You asked a joiner (no CA private key) to issue certs | Join through a seed that ran `init` |
| Second process with `--bootstrap` | New Raft group, not a join | Only the first voter uses `--bootstrap`. Joiners: `clusdr start` then `clusdr join` |

Followers forward Join to the leader. `NOT_LEADER` after that means there is no leader (or the forward could not dial it).

## TLS

TLS is on unless **every** node and client sets `CLUSDR_TLS=disabled`.

| You see | Cause | What to do |
|---|---|---|
| Certificate / handshake errors after join | One side plaintext, the other mTLS; or different CAs | Same `tls.mode` everywhere. Apps load `ca.crt` / `node.crt` / `node.key` from **that host’s** `data.dir` |
| Python: `clusdr: TLS enabled but ca.crt/node.crt/node.key missing in …` | No PEMs in `data_dir` / `CLUSDR_DATA_DIR` / `~/.clusdr` | Point `data_dir` at the daemon’s data dir, or `CLUSDR_TLS=disabled` / `insecure=True` (dev only). Python does **not** skip-verify like Go bootstrap TLS ([Python SDK](../sdk/python.md)) |
| Python: `clusdr: TLS hostname unknown; set CLUSDR_TLS_SERVER_NAME …` | No `server_name`, env, or CN on `node.crt` | Set `server_name=` or `CLUSDR_TLS_SERVER_NAME` to the **peer node id** |
| Go SDK connects, Python does not | Go falls back to bootstrap TLS when PEMs are missing | Give Python the PEMs or disable TLS on both |

`clusdr certs show` prints the CA fingerprint. Compare it across nodes. Server identity is the node id (SAN), not the dial hostname.

## Cluster

| You see | Cause | What to do |
|---|---|---|
| `clusdr leader` / SDK `Unavailable` | No current leader | One voter: that process must be up. Three voters: majority must be up ([leadership](../concepts/leadership.md)) |
| Frequent elections, flapping leader | Peer RTT larger than Raft timers | Defaults are 150ms heartbeat/election. Raise them ([limits](limits.md), [configuration](configuration.md)) |
| `member.left` while the process still exists | Presence lease `presence.<nodeID>` expired (default 3s) | Process wedged or partitioned. Heartbeats are the slower backup ([presence](../concepts/presence.md)) |
| After every reboot I must `join` again | Default `presence_ttl` 3s elapsed while the host was down; the leader **removed** the Raft server | Raise `lease.presence_ttl` above reboot time. Same `data.dir` + `clusdr start` if still a member. `join` only after removal ([presence](../concepts/presence.md)) |
| `member.left` after crash / SIGKILL / reboot | Same path as a leave. No `disconnect` event | Expected. Rejoin with the same token and `node.id` only if the TTL already fired |
| Extra elections on Docker / two hosts | `node.addr` is `0.0.0.0` or `127.0.0.1` on a remote peer | Advertise a host:port **peers can dial** ([other hosts](../guide/other-hosts.md)) |
| `Health.healthy` is always true | Not a bug | `Health.role` is always `standalone` in this version. Use `members` |

## Locks and leases

| You see | Cause | What to do |
|---|---|---|
| `FailedPrecondition` / `observer cannot mutate locks` | Lock RPCs on an observer daemon | Take locks on a **voter** (the leader writes the log). Or `clusdr promote` that node ([observers](../concepts/observers.md)) |
| `fencing token mismatch` | Unlock/revoke with a stale token | Store the token from the grant. A newer owner holds the name ([locks](../concepts/locks.md), [leases](../concepts/leases.md)) |
| `clusdr: lock "…" is not held by this client` (same for lease) | This process is not the holder | Another client owns it, or you already released. `try_lock` / `TryLock` returns not-held instead of blocking |
| `too many locks` / `too many leases (max 4096)` | Table full | Release unused names. Cap is [limits](limits.md) |
| Name rejected | Illegal characters or length | `1–128` of `A–Z a–z 0–9 . _ -` |

Observers **can** hold application leases. Presence still runs on observers so a dead observer leaves the member list.

## Applications

SDK errors are wrapped (`clusdr: daemon not ready at …`, `clusdr: lock "name": …`). Unwrap to the gRPC status when you need the code.

| You see | Cause | What to do |
|---|---|---|
| `clusdr: daemon not ready at …` | Dial ok-ish but Health not ready within ~10s, or daemon down | Start the **local** daemon. The app never dials a remote member ([from your app](../guide/from-your-app.md)) |
| `clusdr: empty dial address` | `Dial("")` | Use `Local()` / `local()`, or pass `CLUSDR_GRPC_ADDR` |
| `clusdr: publish rejected` / payload too large | Custom event over 64 KiB or invalid type | Shrink the payload. Publish is gossip, not Raft ([events](../concepts/events.md)) |
| Watch reconnect misses `custom.*` | Not a bug | Custom events are ephemeral. Cluster events come back in the snapshot |

Go: [SDK errors](../sdk/go.md#errors). Python: [SDK errors](../sdk/python.md#errors).

## Related

- [Guide](../guide/) — install through other hosts
- [Configuration](configuration.md) — YAML and `CLUSDR_*`
- [CLI](cli/) — per-command flags
