package presence_test

import (
	"testing"

	"github.com/odurgut/clusdr/internal/events"
	"github.com/odurgut/clusdr/internal/presence"
)

func TestLeaseName_RoundTrip(t *testing.T) {
	name := presence.LeaseName("node-a")
	if name != "presence.node-a" {
		t.Errorf("name: %q", name)
	}
	id, ok := presence.NodeID(name)
	if !ok || id != "node-a" {
		t.Errorf("parse: id=%q ok=%v", id, ok)
	}
	if _, ok := presence.NodeID("worker-1"); ok {
		t.Error("user lease must not parse as presence")
	}
	if _, ok := presence.NodeID("presence."); ok {
		t.Error("empty node id")
	}
}

func TestOnExpired_RemovesPeerOnLeader(t *testing.T) {
	var got string
	presence.OnExpired(events.Event{
		Type:   events.TypeLeaseExpired,
		Source: presence.LeaseName("node-b"),
	}, "node-a", true, func(id string) { got = id })
	if got != "node-b" {
		t.Errorf("removed: %q", got)
	}
}

func TestOnExpired_IgnoresNonPresenceAndSelf(t *testing.T) {
	var n int
	rm := func(string) { n++ }
	presence.OnExpired(events.Event{Type: events.TypeLeaseExpired, Source: "worker-1"}, "node-a", true, rm)
	presence.OnExpired(events.Event{Type: events.TypeLeaseGranted, Source: presence.LeaseName("node-b")}, "node-a", true, rm)
	presence.OnExpired(events.Event{Type: events.TypeLeaseExpired, Source: presence.LeaseName("node-a")}, "node-a", true, rm)
	presence.OnExpired(events.Event{Type: events.TypeLeaseExpired, Source: presence.LeaseName("node-b")}, "node-a", false, rm)
	if n != 0 {
		t.Errorf("remove called %d times", n)
	}
}
