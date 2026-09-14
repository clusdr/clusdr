# Configuration

Merge order (last wins): built-in defaults → YAML (`--config`, default `clusdr.yaml`) → `CLUSDR_*` environment variables.

`clusdr config validate` prints the resolved struct.

Only keys that exist in code are listed. Heartbeat and `shutdown_timeout` are YAML-only.

## YAML

```yaml
node:
  id: ""                 # set by init; required for a joining node
  addr: "0.0.0.0:7947"   # advertised member address — use a dialable host:port
cluster:
  id: ""                 # set by init
data:
  dir: "~/.clusdr"       # actual default is $HOME/.clusdr
log:
  level: info            # debug | info | warn | error
  format: text           # text | json
grpc:
  addr: "127.0.0.1:7947"
  control_socket: "~/.clusdr/clusdr.sock"  # actual default is $HOME/.clusdr/clusdr.sock
  dial_timeout: 5s
  request_timeout: 10s
shutdown_timeout: 15s    # only applied if > 0; otherwise process default 15s
heartbeat:
  interval: 2s
  timeout: 1s
  max_misses: 3
raft:
  addr: "127.0.0.1:7946"
  bootstrap: false
  heartbeat_timeout: 150ms
  election_timeout: 150ms
  leader_lease_timeout: 75ms
tls:
  mode: enabled          # enabled | disabled
lock:
  ttl: 15s
  expire_interval: 100ms
lease:
  ttl: 15s
  expire_interval: 100ms
  presence: true
  presence_ttl: 3s
```

`leader_lease_timeout` must be ≤ `heartbeat_timeout`. If an override breaks that, load clamps lease to half the heartbeat.

## Environment

| Variable | Sets |
|---|---|
| `CLUSDR_NODE_ID` | `node.id` |
| `CLUSDR_NODE_ADDR` | `node.addr` |
| `CLUSDR_CLUSTER_ID` | `cluster.id` |
| `CLUSDR_DATA_DIR` | `data.dir` |
| `CLUSDR_LOG_LEVEL` | `log.level` |
| `CLUSDR_LOG_FORMAT` | `log.format` |
| `CLUSDR_GRPC_ADDR` | `grpc.addr` |
| `CLUSDR_CONTROL_SOCKET` | `grpc.control_socket` |
| `CLUSDR_RAFT_ADDR` | `raft.addr` |
| `CLUSDR_RAFT_BOOTSTRAP` | `raft.bootstrap` when `true` or `1` |
| `CLUSDR_RAFT_HEARTBEAT_TIMEOUT` | duration |
| `CLUSDR_RAFT_ELECTION_TIMEOUT` | duration |
| `CLUSDR_RAFT_LEADER_LEASE_TIMEOUT` | duration |
| `CLUSDR_TLS` | `tls.mode` (`disabled` / `off` / `false` / `0` turn TLS off) |
| `CLUSDR_LOCK_TTL` | duration |
| `CLUSDR_LOCK_EXPIRE_INTERVAL` | duration |
| `CLUSDR_LEASE_TTL` | duration |
| `CLUSDR_LEASE_EXPIRE_INTERVAL` | duration |
| `CLUSDR_LEASE_PRESENCE` | bool (`true`/`1`/`on`/`enabled`, or off) |
| `CLUSDR_LEASE_PRESENCE_TTL` | duration |

CLI `--bootstrap` injects `CLUSDR_RAFT_BOOTSTRAP=true` for that process only.

SDK extra: `CLUSDR_TLS_SERVER_NAME` is read by the **Python** client only.

Flags `--log-level` and `--log-format` override env and YAML for that CLI invocation.

`CLUSDR_SHUTDOWN_TIMEOUT` is **not** implemented.
