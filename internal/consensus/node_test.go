package consensus_test

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/durguto/clusdr/internal/consensus"
	"github.com/durguto/clusdr/internal/membership"
)

// nopApplier satisfies consensus.MemberApplier without any real membership engine.
type nopApplier struct{}

func (n *nopApplier) Join(_ string, _ string) (bool, error) { return false, nil }
func (n *nopApplier) JoinAs(_ string, _ string, _ string) (bool, error) {
	return false, nil
}
func (n *nopApplier) MarkLeaving(_ string)         {}
func (n *nopApplier) Members() []membership.Member { return nil }

func nopLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError + 10}))
}

// TestNode_Bootstrap_SingleNode verifies that a single bootstrapped node
// elects itself leader within a reasonable timeout.
func TestNode_Bootstrap_SingleNode(t *testing.T) {
	dir := t.TempDir()
	cfg := consensus.Config{
		Addr:      "127.0.0.1:17946",
		Bootstrap: true,
	}

	node, err := consensus.New(cfg, "test-node-1", dir, &nopApplier{}, nil, nil, nopLog())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { node.Shutdown() }) //nolint:errcheck

	// Wait for leadership to be established.
	leaderCh := make(chan bool, 1)
	node.WatchLeadership(func(isLeader bool) {
		if isLeader {
			select {
			case leaderCh <- true:
			default:
			}
		}
	})

	select {
	case <-leaderCh:
		// good — node elected itself
	case <-time.After(5 * time.Second):
		t.Fatal("single node did not become leader within 5s")
	}

	if !node.IsLeader() {
		t.Error("IsLeader() returned false after leadership event")
	}
	if node.LeaderID() != "test-node-1" {
		t.Errorf("LeaderID: got %q, want %q", node.LeaderID(), "test-node-1")
	}
}

// TestNode_NoBootstrap_NotLeader verifies that a non-bootstrap node does not
// spontaneously elect itself (waits for Raft peers).
func TestNode_NoBootstrap_NotLeader(t *testing.T) {
	dir := t.TempDir()
	cfg := consensus.Config{
		Addr:      "127.0.0.1:17947",
		Bootstrap: false,
	}

	node, err := consensus.New(cfg, "test-node-2", dir, &nopApplier{}, nil, nil, nopLog())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { node.Shutdown() }) //nolint:errcheck

	// A non-bootstrap node should NOT become leader on its own.
	time.Sleep(500 * time.Millisecond)
	if node.IsLeader() {
		t.Error("non-bootstrap node should not elect itself leader")
	}
}

// TestNode_Shutdown_Clean verifies that Shutdown does not block or panic.
func TestNode_Shutdown_Clean(t *testing.T) {
	dir := t.TempDir()
	cfg := consensus.Config{
		Addr:      "127.0.0.1:17948",
		Bootstrap: true,
	}

	node, err := consensus.New(cfg, "test-node-3", dir, &nopApplier{}, nil, nil, nopLog())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := node.Shutdown(); err != nil {
			t.Errorf("Shutdown: %v", err)
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown did not return within 5s")
	}
}
