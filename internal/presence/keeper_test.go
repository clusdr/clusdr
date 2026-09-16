package presence_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/clusdr/clusdr/internal/presence"
)

type fakeHolder struct {
	mu        sync.Mutex
	grants    int
	renews    int
	token     uint64
	failRenew bool
}

func (f *fakeHolder) Grant(_ context.Context, _, _ string, _ time.Duration) (uint64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.grants++
	f.token++
	return f.token, nil
}

func (f *fakeHolder) Renew(_ context.Context, _, _ string, token uint64, _ time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failRenew {
		return presence.ErrHeld
	}
	f.renews++
	if token == 0 {
		return presence.ErrHeld
	}
	return nil
}

func TestKeeper_GrantsAndRenews(t *testing.T) {
	h := &fakeHolder{}
	k := presence.NewKeeper(presence.Config{
		NodeID:         "node-a",
		TTL:            80 * time.Millisecond,
		RenewInterval:  20 * time.Millisecond,
		RequestTimeout: time.Second,
	}, h, nil)
	k.Start()
	t.Cleanup(k.Stop)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		h.mu.Lock()
		g, r := h.grants, h.renews
		h.mu.Unlock()
		if g >= 1 && r >= 1 && k.Token() != 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	t.Fatalf("grants=%d renews=%d token=%d", h.grants, h.renews, k.Token())
}

func TestKeeper_RegrantsAfterRenewFailure(t *testing.T) {
	h := &fakeHolder{failRenew: true}
	k := presence.NewKeeper(presence.Config{
		NodeID:         "node-a",
		TTL:            50 * time.Millisecond,
		RenewInterval:  15 * time.Millisecond,
		RequestTimeout: time.Second,
	}, h, nil)
	k.Start()
	t.Cleanup(k.Stop)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		h.mu.Lock()
		g := h.grants
		h.mu.Unlock()
		if g >= 2 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("did not re-grant after renew failure")
}
