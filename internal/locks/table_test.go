package locks_test

import (
	"context"
	"testing"
	"time"

	"github.com/odurgut/clusdr/internal/events"
	"github.com/odurgut/clusdr/internal/locks"
)

func TestAcquire_SecondHolderLoses(t *testing.T) {
	tab := locks.New()
	tok, ok, err := tab.Acquire("scheduler", "node-a", time.Time{}, 0)
	if err != nil || !ok || tok == 0 {
		t.Fatalf("first: token=%d ok=%v err=%v", tok, ok, err)
	}
	tok2, ok, err := tab.Acquire("scheduler", "node-b", time.Time{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("second holder must not acquire")
	}
	if tok2 != tok {
		t.Errorf("held token: got %d want %d", tok2, tok)
	}
}

func TestAcquire_SameHolderIdempotent(t *testing.T) {
	tab := locks.New()
	tok, _, err := tab.Acquire("scheduler", "node-a", time.Time{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	tok2, ok, err := tab.Acquire("scheduler", "node-a", time.Time{}, 0)
	if err != nil || !ok {
		t.Fatalf("re-acquire: ok=%v err=%v", ok, err)
	}
	if tok2 != tok {
		t.Errorf("token changed on re-acquire: %d → %d", tok, tok2)
	}
}

func TestRelease_WrongTokenRejected(t *testing.T) {
	tab := locks.New()
	tok, _, err := tab.Acquire("scheduler", "node-a", time.Time{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := tab.Release("scheduler", "node-a", tok+1); err == nil {
		t.Fatal("expected fencing mismatch")
	}
	if _, ok := tab.Get("scheduler"); !ok {
		t.Fatal("lock released with bad token")
	}
	if err := tab.Release("scheduler", "node-a", tok); err != nil {
		t.Fatal(err)
	}
	if _, ok := tab.Get("scheduler"); ok {
		t.Fatal("lock still held")
	}
}

func TestWait_WakesOnRelease(t *testing.T) {
	tab := locks.New()
	if _, _, err := tab.Acquire("scheduler", "node-a", time.Time{}, 0); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- tab.Wait(ctx) }()

	time.Sleep(20 * time.Millisecond)
	rec, _ := tab.Get("scheduler")
	if err := tab.Release("scheduler", "node-a", rec.Token); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Wait: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Wait did not wake")
	}
}

func TestTable_Restore(t *testing.T) {
	tab := locks.New()
	tok, _, err := tab.Acquire("scheduler", "node-a", time.Time{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	snap := tab.SnapshotCopy()
	tab2 := locks.New()
	tab2.Restore(snap)
	rec, ok := tab2.Get("scheduler")
	if !ok || rec.Holder != "node-a" || rec.Token != tok {
		t.Fatalf("restore: %+v ok=%v", rec, ok)
	}
	_, ok, err = tab2.Acquire("other", "node-b", time.Time{}, 0)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if rec2, _ := tab2.Get("other"); rec2.Token <= tok {
		t.Errorf("next token not preserved: %d", rec2.Token)
	}
}

func TestValidName(t *testing.T) {
	if err := locks.ValidName(""); err == nil {
		t.Error("empty name")
	}
	if err := locks.ValidName("scheduler"); err != nil {
		t.Fatal(err)
	}
	if err := locks.ValidName("bad name"); err == nil {
		t.Error("space should be invalid")
	}
}

func TestAcquire_SameHolderRefreshesDeadline(t *testing.T) {
	tab := locks.New()
	d1 := time.Now().Add(time.Second)
	if _, _, err := tab.Acquire("job", "node-a", d1, time.Second); err != nil {
		t.Fatal(err)
	}
	d2 := time.Now().Add(10 * time.Second)
	tok, ok, err := tab.Acquire("job", "node-a", d2, 10*time.Second)
	if err != nil || !ok || tok == 0 {
		t.Fatalf("re-acquire: tok=%d ok=%v err=%v", tok, ok, err)
	}
	rec, _ := tab.Get("job")
	if rec.Deadline.Before(d2.Add(-time.Millisecond)) {
		t.Errorf("deadline not refreshed: %v", rec.Deadline)
	}
}

func TestExpire_DropsDueLockAndEmits(t *testing.T) {
	tab := locks.New()
	var got []string
	tab.Emit = func(e events.Event) { got = append(got, e.Type+":"+e.Source) }

	past := time.Now().Add(-time.Millisecond)
	tok, _, err := tab.Acquire("job", "node-a", past, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	due := tab.Due(time.Now())
	if len(due) != 1 || due[0].Name != "job" {
		t.Fatalf("due: %+v", due)
	}
	if !tab.Expire("job", tok) {
		t.Fatal("expire should drop the grant")
	}
	if _, ok := tab.Get("job"); ok {
		t.Fatal("lock still held after expire")
	}
	if len(got) != 1 || got[0] != events.TypeLockExpired+":job" {
		t.Errorf("emit: %v", got)
	}
	if tab.Expire("job", tok) {
		t.Fatal("second expire must be a no-op")
	}
}

func TestExpire_WrongTokenNoOp(t *testing.T) {
	tab := locks.New()
	tok, _, err := tab.Acquire("job", "node-a", time.Now().Add(time.Hour), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if tab.Expire("job", tok+1) {
		t.Fatal("wrong token must not expire")
	}
	if _, ok := tab.Get("job"); !ok {
		t.Fatal("lock dropped on token mismatch")
	}
}

func TestRenew_ExtendsDeadline(t *testing.T) {
	tab := locks.New()
	tok, _, err := tab.Acquire("job", "node-a", time.Now().Add(20*time.Millisecond), 20*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Hour)
	if err := tab.Renew("job", "node-a", tok, later, time.Hour); err != nil {
		t.Fatal(err)
	}
	if due := tab.Due(time.Now().Add(time.Second)); len(due) != 0 {
		t.Fatalf("renewed lock still due: %+v", due)
	}
	rec, _ := tab.Get("job")
	if rec.Deadline.Before(later.Add(-time.Millisecond)) {
		t.Errorf("deadline: %v want ~%v", rec.Deadline, later)
	}
}

func TestClampTTL(t *testing.T) {
	if got := locks.ClampTTL(0, 0); got != locks.DefaultTTL {
		t.Errorf("zero: %v", got)
	}
	if got := locks.ClampTTL(0, 3*time.Second); got != 3*time.Second {
		t.Errorf("fallback: %v", got)
	}
	if got := locks.ClampTTL(time.Nanosecond, 0); got != locks.MinTTL {
		t.Errorf("min: %v", got)
	}
	if got := locks.ClampTTL(48*time.Hour, 0); got != locks.MaxTTL {
		t.Errorf("max: %v", got)
	}
}
