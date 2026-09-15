// Package clusdr is the application SDK for the local Clusdr daemon.
//
// The application is not a cluster member. It dials the daemon on this host
// the same way a process talks to a local Docker engine. The daemon votes,
// stores membership, and forwards mutations to the Raft leader. This package
// never dials other nodes and never joins Raft.
//
//	your process  ──►  clusdr daemon on this host  ──►  the rest of the cluster
//
// # Install
//
//	go get github.com/durguto/clusdr/sdk
//
// Use the same version train as the daemon. A running daemon is required:
//
//	curl -fsSL https://clusdr.io/install.sh | sh
//	clusdr init && clusdr start --bootstrap
//
// # Connect
//
// [Local] is the application path. It dials CLUSDR_GRPC_ADDR or 127.0.0.1:7947,
// then waits on Health (10s). [Dial] takes an explicit address (tests, a second
// daemon on this host). Do not Dial a remote node's Runtime API as the normal
// path — run a daemon on that host and call Local there.
//
//	c, err := clusdr.Local()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer c.Close()
//
// TLS is on unless CLUSDR_TLS=disabled or [WithInsecure]. Certs are
// ca.crt, node.crt, node.key from [WithDataDir], else CLUSDR_DATA_DIR, else
// ~/.clusdr. Missing PEMs fall back to bootstrap TLS (skip hostname).
//
// # Surface
//
// [Cluster] is the handle: Members, Leader, Watch, Publish, Lock / TryLock /
// Unlock, Lease / Renew / Revoke, Close.
//
// Unary RPCs retry Unavailable, Aborted, and ResourceExhausted until the
// context deadline (backoff 50ms → 2s). Watch reconnects with last_seq.
// Close unlocks and revokes what this connection still holds.
//
// One Cluster is safe for concurrent goroutines. Unlock is per connection
// holder, not per goroutine. Each Watch call has its own stream and channel.
//
// # Docs
//
// Walkthrough and options: https://clusdr.io/docs/sdk/go
//
// First daemon: https://clusdr.io/docs/guide/first-member
//
// Examples in this module and https://github.com/durguto/clusdr/tree/main/examples
package clusdr
