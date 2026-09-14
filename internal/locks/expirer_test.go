package locks_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/odurgut/clusdr/internal/locks"
)

func TestRunExpirer_ReleasesDueLock(t *testing.T) {
	tab := locks.New()
	past := time.Now().Add(-time.Millisecond)
	tok, _, err := tab.Acquire("job", "holder", past, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go locks.RunExpirer(ctx, tab, func() bool { return true }, func(name string, token uint64) error {
		tab.Expire(name, token)
		return nil
	}, 10*time.Millisecond, nil)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok := tab.Get("job"); !ok {
			if tok == 0 {
				t.Fatal("token was zero")
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expirer did not drop due lock")
}

func TestRunExpirer_SkipsWhenNotLeader(t *testing.T) {
	tab := locks.New()
	past := time.Now().Add(-time.Millisecond)
	if _, _, err := tab.Acquire("job", "holder", past, time.Millisecond); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	locks.RunExpirer(ctx, tab, func() bool { return false }, func(string, uint64) error {
		calls.Add(1)
		return nil
	}, 10*time.Millisecond, nil)
	if calls.Load() != 0 {
		t.Fatalf("follower applied expire %d times", calls.Load())
	}
	if _, ok := tab.Get("job"); !ok {
		t.Fatal("follower must not expire locally")
	}
}
