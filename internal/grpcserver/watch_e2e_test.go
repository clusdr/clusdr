package grpcserver_test

// watch_e2e_test.go — in-process end-to-end test for Watch.
//
// Verifies the full Watch stream lifecycle without shell processes:
//   - Snapshot: new watcher receives current cluster state on connect.
//   - Live events: member.join/member.left/leader.changed flow through.
//   - Reconnect: second connect also gets the snapshot.
//   - Filter: event_types filter works server-side.
//   - Backpressure: slow watcher never blocks the cluster.

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/durguto/clusdr/internal/eventbus"
	"github.com/durguto/clusdr/internal/events"
	"github.com/durguto/clusdr/internal/grpcserver"
	"github.com/durguto/clusdr/internal/membership"
	proto "github.com/durguto/clusdr/api/clusdr/v1alpha1"
)

// startE2EServer starts a WatchService backed by a live membership engine and
// event bus — the same combination used in production.
func startE2EServer(t *testing.T) (
	bus *eventbus.Bus,
	mem *membership.Engine,
	client proto.WatchServiceClient,
) {
	t.Helper()

	log := nopLogger()
	bus = eventbus.New()
	mem = membership.New("node-a", "127.0.0.1:8001", log)
	mem.Emit = bus.Publish // wire bus into engine (same as UnitBus)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := grpc.NewServer()
	grpcserver.RegisterWatchService(srv, bus, mem, log)
	go srv.Serve(ln) //nolint:errcheck
	t.Cleanup(srv.GracefulStop)

	cc, err := grpc.NewClient(ln.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { cc.Close() })
	return bus, mem, proto.NewWatchServiceClient(cc)
}

// recv receives the next event within a deadline.
func recv(t *testing.T, stream proto.WatchService_WatchClient, timeout time.Duration) *proto.WatchResponse {
	t.Helper()
	ch := make(chan *proto.WatchResponse, 1)
	go func() {
		resp, err := stream.Recv()
		if err != nil {
			if err != io.EOF {
				t.Logf("recv error: %v", err)
			}
			return
		}
		ch <- resp
	}()
	select {
	case resp := <-ch:
		return resp
	case <-time.After(timeout):
		t.Fatalf("no event within %v", timeout)
		return nil
	}
}

// TestE2E_WatchSnapshotThenLive:
//
// Done criterion: a Watch stream receives member.join when a new node joins.
func TestE2E_WatchSnapshotThenLive(t *testing.T) {
	bus, mem, client := startE2EServer(t)

	// Prime the engine: A is already in the cluster.
	mem.Join("node-a", "127.0.0.1:8001") //nolint:errcheck
	mem.SetLeader("node-a")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.Watch(ctx, &proto.WatchRequest{})
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}

	// snapshot
	// Expect member.join for node-a (snapshot, seq=0).
	snap1 := recv(t, stream, time.Second)
	if snap1.Type != events.TypeMemberJoin {
		t.Errorf("snapshot[0] type: got %q, want member.join", snap1.Type)
	}
	if snap1.Source != "node-a" {
		t.Errorf("snapshot[0] source: got %q, want node-a", snap1.Source)
	}
	if snap1.Seq != 0 {
		t.Errorf("snapshot seq: got %d, want 0", snap1.Seq)
	}

	// Expect leader.changed for node-a (snapshot, seq=0).
	snap2 := recv(t, stream, time.Second)
	if snap2.Type != events.TypeLeaderChanged {
		t.Errorf("snapshot[1] type: got %q, want leader.changed", snap2.Type)
	}

	syncEvt := recv(t, stream, time.Second)
	if syncEvt.Type != events.TypeWatchSync {
		t.Errorf("after snapshot: got %q, want watch.sync", syncEvt.Type)
	}

	// live member.join
	// Simulate a new node B joining: membership.Join emits via bus.
	if _, err := mem.Join("node-b", "127.0.0.1:8002"); err != nil {
		t.Fatalf("Join node-b: %v", err)
	}

	live := recv(t, stream, time.Second)
	if live.Type != events.TypeMemberJoin {
		t.Errorf("live event type: got %q, want member.join", live.Type)
	}
	if live.Source != "node-b" {
		t.Errorf("live event source: got %q, want node-b", live.Source)
	}
	if live.Seq == 0 {
		t.Error("live event seq should be non-zero")
	}

	// live leader.changed
	bus.Publish(events.Event{Type: events.TypeLeaderChanged, Source: "node-b"})
	lc := recv(t, stream, time.Second)
	if lc.Type != events.TypeLeaderChanged {
		t.Errorf("leader event type: got %q, want leader.changed", lc.Type)
	}

	// live member.left
	mem.Leave("node-b")
	left := recv(t, stream, time.Second)
	if left.Type != events.TypeMemberLeft {
		t.Errorf("left event type: got %q, want member.left", left.Type)
	}
	if left.Source != "node-b" {
		t.Errorf("left event source: got %q, want node-b", left.Source)
	}

	t.Logf("✅ All Watch stream events received correctly (snapshot + 3 live events)")
}

// TestE2E_WatchReconnectGetsSnapshot verifies the reconnect contract:
// a client that disconnects and reconnects always sees the current snapshot.
func TestE2E_WatchReconnectGetsSnapshot(t *testing.T) {
	_, mem, client := startE2EServer(t)
	mem.Join("node-a", "127.0.0.1:8001") //nolint:errcheck
	mem.SetLeader("node-a")
	mem.Join("node-b", "127.0.0.1:8002") //nolint:errcheck

	// First connection — consume snapshot then disconnect.
	{
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		stream, _ := client.Watch(ctx, &proto.WatchRequest{})
		recv(t, stream, time.Second) // node-a join
		recv(t, stream, time.Second) // node-b join
		recv(t, stream, time.Second) // leader.changed
		recv(t, stream, time.Second) // watch.sync
		cancel()
	}
	time.Sleep(50 * time.Millisecond) // let stream close

	// Second connection — must still get full snapshot (3 events: 2 joins + 1 leader).
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()
	stream2, err := client.Watch(ctx2, &proto.WatchRequest{})
	if err != nil {
		t.Fatalf("reconnect Watch: %v", err)
	}

	var types []string
	for i := 0; i < 3; i++ {
		r := recv(t, stream2, time.Second)
		types = append(types, r.Type)
	}
	joinCount := 0
	leaderCount := 0
	for _, tp := range types {
		switch tp {
		case events.TypeMemberJoin:
			joinCount++
		case events.TypeLeaderChanged:
			leaderCount++
		}
	}
	if joinCount != 2 {
		t.Errorf("reconnect: expected 2 member.join, got %d (types=%v)", joinCount, types)
	}
	if leaderCount != 1 {
		t.Errorf("reconnect: expected 1 leader.changed, got %d", leaderCount)
	}
}

// TestE2E_FilteredWatch verifies that event_types filter is applied on the server.
func TestE2E_FilteredWatch(t *testing.T) {
	bus, _, client := startE2EServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Subscribe only to leader.changed.
	stream, err := client.Watch(ctx, &proto.WatchRequest{
		EventTypes: []string{events.TypeLeaderChanged},
	})
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}

	// Snapshot always includes leader.changed for the local node (seq=0).
	snap := recv(t, stream, time.Second)
	if snap.Type != events.TypeLeaderChanged || snap.Seq != 0 {
		t.Fatalf("snapshot: type=%q seq=%d, want leader.changed seq=0", snap.Type, snap.Seq)
	}
	syncEvt := recv(t, stream, time.Second)
	if syncEvt.Type != events.TypeWatchSync {
		t.Fatalf("after snapshot: got %q, want watch.sync", syncEvt.Type)
	}

	go func() {
		time.Sleep(20 * time.Millisecond)
		bus.Publish(events.Event{Type: events.TypeMemberJoin, Source: "x"})    // filtered
		bus.Publish(events.Event{Type: events.TypeMemberLeft, Source: "x"})    // filtered
		bus.Publish(events.Event{Type: events.TypeLeaderChanged, Source: "L"}) // passes
	}()

	resp := recv(t, stream, time.Second)
	if resp.Type != events.TypeLeaderChanged {
		t.Errorf("filtered: got %q, want leader.changed", resp.Type)
	}
	if resp.Source != "L" {
		t.Errorf("filtered source: got %q, want L", resp.Source)
	}
}

// TestE2E_SlowWatcherNeverBlocksMembership verifies that a lagging Watch
// client cannot slow down membership operations (the core backpressure guarantee).
func TestE2E_SlowWatcherNeverBlocksMembership(t *testing.T) {
	_, mem, client := startE2EServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Open a watch stream but NEVER read from it → simulate slow client.
	_, err := client.Watch(ctx, &proto.WatchRequest{})
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	// Do NOT call Recv() — the stream is intentionally not read.

	// Perform 200 membership operations; they must all complete quickly.
	start := time.Now()
	for i := 0; i < 200; i++ {
		id := "node-" + string(rune('a'+i%26))
		mem.Join(id, "127.0.0.1:8000") //nolint:errcheck
	}
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Errorf("200 Join calls took %v — slow watcher blocked the cluster", elapsed)
	}

	// Use io.Discard instead
	_ = io.Discard
	t.Logf("200 Join calls completed in %v (slow watcher did not block)", elapsed)
}

// TestE2E_DisconnectReconnectResumes:
// Disconnect, change cluster state, reconnect with last_seq → snapshot of
// current members, watch.sync, watch.gap, then live events resume.
func TestE2E_DisconnectReconnectResumes(t *testing.T) {
	_, mem, client := startE2EServer(t)
	mem.Join("node-a", "127.0.0.1:8001") //nolint:errcheck
	mem.SetLeader("node-a")

	var lastSeq uint64
	{
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		stream, err := client.Watch(ctx, &proto.WatchRequest{})
		if err != nil {
			t.Fatalf("Watch: %v", err)
		}
		recv(t, stream, time.Second) // snapshot join
		recv(t, stream, time.Second) // snapshot leader
		recv(t, stream, time.Second) // watch.sync

		if _, err := mem.Join("node-b", "127.0.0.1:8002"); err != nil {
			t.Fatalf("Join node-b: %v", err)
		}
		live := recv(t, stream, time.Second)
		if live.Type != events.TypeMemberJoin || live.Source != "node-b" {
			t.Fatalf("live: got %s %s", live.Type, live.Source)
		}
		lastSeq = live.Seq
		cancel()
	}
	time.Sleep(50 * time.Millisecond)

	// Cluster changes while the watcher is gone.
	if _, err := mem.Join("node-c", "127.0.0.1:8003"); err != nil {
		t.Fatalf("Join node-c: %v", err)
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel2()
	stream2, err := client.Watch(ctx2, &proto.WatchRequest{LastSeq: lastSeq})
	if err != nil {
		t.Fatalf("reconnect Watch: %v", err)
	}

	var joins []string
	var sawSync, sawGap bool
	for i := 0; i < 8; i++ {
		r := recv(t, stream2, time.Second)
		switch r.Type {
		case events.TypeMemberJoin:
			joins = append(joins, r.Source)
		case events.TypeWatchSync:
			sawSync = true
			if r.Seq < lastSeq {
				t.Errorf("sync seq=%d < last_seq=%d", r.Seq, lastSeq)
			}
		case events.TypeWatchGap:
			sawGap = true
		}
		if sawSync && sawGap && len(joins) >= 3 {
			break
		}
	}
	if !sawSync {
		t.Error("reconnect missing watch.sync")
	}
	if !sawGap {
		t.Error("reconnect missing watch.gap (last_seq was behind)")
	}
	found := map[string]bool{}
	for _, id := range joins {
		found[id] = true
	}
	for _, id := range []string{"node-a", "node-b", "node-c"} {
		if !found[id] {
			t.Errorf("reconnect snapshot missing %s (got %v)", id, joins)
		}
	}

	mem.Leave("node-c")
	left := recv(t, stream2, time.Second)
	if left.Type != events.TypeMemberLeft || left.Source != "node-c" {
		t.Errorf("resumed live: got %s %s, want member.left node-c", left.Type, left.Source)
	}
}
