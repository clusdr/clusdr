package presence

import "github.com/clusdr/clusdr/internal/events"

// OnExpired turns a presence lease.expired event into member.dead.
// Non-presence leases, self, and non-leaders are ignored. remove is
// ApplyRemoveMember (mark dead). It must not RemoveServer; that is clusdr leave.
func OnExpired(e events.Event, selfID string, isLeader bool, remove func(id string)) {
	if e.Type != events.TypeLeaseExpired {
		return
	}
	id, ok := NodeID(e.Source)
	if !ok || id == "" || id == selfID || !isLeader || remove == nil {
		return
	}
	remove(id)
}
