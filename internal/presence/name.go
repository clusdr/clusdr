// Package presence maps cluster nodes to reserved leases used as a
// faster dead-detection path alongside heartbeats.
package presence

import "strings"

// Prefix is reserved for per-node presence leases. User leases must not
// use this prefix if they want to avoid membership side effects.
const Prefix = "presence."

// LeaseName is the Raft lease identity for nodeID.
func LeaseName(nodeID string) string {
	return Prefix + nodeID
}

// NodeID extracts the node id from a presence lease name.
func NodeID(leaseName string) (string, bool) {
	if !strings.HasPrefix(leaseName, Prefix) {
		return "", false
	}
	id := strings.TrimPrefix(leaseName, Prefix)
	if id == "" {
		return "", false
	}
	return id, true
}
