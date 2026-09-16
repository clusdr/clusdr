package leases_test

import (
	"testing"
	"time"

	"github.com/clusdr/clusdr/internal/events"
	"github.com/clusdr/clusdr/internal/leases"
)

func TestGrant_SecondOwnerLoses(t *testing.T) {
	tab := leases.New()
	tok, ok, err := tab.Grant("worker-1", "node-a", time.Time{}, 0)
	if err != nil || !ok || tok == 0 {
		t.Fatalf("first: token=%d ok=%v err=%v", tok, ok, err)
	}
	tok2, ok, err := tab.Grant("worker-1", "node-b", time.Time{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("second owner must not grant")
	}
	if tok2 != tok {
		t.Errorf("held token: got %d want %d", tok2, tok)
	}
}

func TestGrant_SameOwnerIdempotent(t *testing.T) {
	tab := leases.New()
	tok, _, err := tab.Grant("worker-1", "node-a", time.Time{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	tok2, ok, err := tab.Grant("worker-1", "node-a", time.Now().Add(time.Hour), time.Hour)
	if err != nil || !ok {
		t.Fatalf("re-grant: ok=%v err=%v", ok, err)
	}
	if tok2 != tok {
		t.Errorf("token changed on re-grant: %d → %d", tok, tok2)
	}
}

func TestGrant_EmitsGrantedOnce(t *testing.T) {
	tab := leases.New()
	var types []string
	tab.Emit = func(e events.Event) { types = append(types, e.Type) }
	if _, _, err := tab.Grant("worker-1", "node-a", time.Time{}, 0); err != nil {
		t.Fatal(err)
	}
	if _, _, err := tab.Grant("worker-1", "node-a", time.Time{}, 0); err != nil {
		t.Fatal(err)
	}
	if len(types) != 1 || types[0] != events.TypeLeaseGranted {
		t.Errorf("events: %v", types)
	}
}

func TestRevoke_WrongTokenRejected(t *testing.T) {
	tab := leases.New()
	var types []string
	tab.Emit = func(e events.Event) { types = append(types, e.Type) }
	tok, _, err := tab.Grant("worker-1", "node-a", time.Time{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := tab.Revoke("worker-1", "node-a", tok+1); err == nil {
		t.Fatal("expected fencing mismatch")
	}
	if err := tab.Revoke("worker-1", "node-a", tok); err != nil {
		t.Fatal(err)
	}
	if _, ok := tab.Get("worker-1"); ok {
		t.Fatal("lease still held")
	}
	if len(types) != 2 || types[1] != events.TypeLeaseRevoked {
		t.Errorf("events: %v", types)
	}
}

func TestExpire_DropsDueLeaseAndEmits(t *testing.T) {
	tab := leases.New()
	var got events.Event
	tab.Emit = func(e events.Event) { got = e }

	past := time.Now().Add(-time.Millisecond)
	tok, _, err := tab.Grant("worker-1", "node-a", past, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if len(tab.Due(time.Now())) != 1 {
		t.Fatal("expected due lease")
	}
	if !tab.Expire("worker-1", tok) {
		t.Fatal("expire should drop the grant")
	}
	if _, ok := tab.Get("worker-1"); ok {
		t.Fatal("lease still held after expire")
	}
	if got.Type != events.TypeLeaseExpired || got.Source != "worker-1" {
		t.Errorf("event: %+v", got)
	}
}

func TestRenew_ExtendsDeadline(t *testing.T) {
	tab := leases.New()
	tok, _, err := tab.Grant("worker-1", "node-a", time.Now().Add(20*time.Millisecond), 20*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Hour)
	if err := tab.Renew("worker-1", "node-a", tok, later, time.Hour); err != nil {
		t.Fatal(err)
	}
	if due := tab.Due(time.Now().Add(time.Second)); len(due) != 0 {
		t.Fatalf("renewed lease still due: %+v", due)
	}
}

func TestTable_Restore(t *testing.T) {
	tab := leases.New()
	tok, _, err := tab.Grant("worker-1", "node-a", time.Time{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	snap := tab.SnapshotCopy()
	tab2 := leases.New()
	tab2.Restore(snap)
	rec, ok := tab2.Get("worker-1")
	if !ok || rec.Owner != "node-a" || rec.Token != tok {
		t.Fatalf("restore: %+v ok=%v", rec, ok)
	}
}

func TestValidName(t *testing.T) {
	if err := leases.ValidName(""); err == nil {
		t.Error("empty name")
	}
	if err := leases.ValidName("worker-1"); err != nil {
		t.Fatal(err)
	}
	if err := leases.ValidName("bad name"); err == nil {
		t.Error("space should be invalid")
	}
}
