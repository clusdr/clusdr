package grpcserver_test

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	proto "github.com/durguto/clusdr/api/clusdr/v1alpha1"
	"github.com/durguto/clusdr/internal/eventbus"
	"github.com/durguto/clusdr/internal/events"
	"github.com/durguto/clusdr/internal/grpcserver"
	"github.com/durguto/clusdr/internal/membership"
)

// fakeWatchState implements WatchStater for tests.
type fakeWatchState struct {
	members  []membership.Member
	leaderID string
}

func (f *fakeWatchState) Members() []membership.Member { return f.members }
func (f *fakeWatchState) LeaderID() string             { return f.leaderID }

// startWatchServer starts an in-process gRPC server with only WatchService.
func startWatchServer(t *testing.T, bus *eventbus.Bus, state grpcserver.WatchStater) proto.WatchServiceClient {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := grpc.NewServer()
	grpcserver.RegisterWatchService(srv, bus, state, nopLogger())
	go srv.Serve(ln) //nolint:errcheck
	t.Cleanup(srv.GracefulStop)

	cc, err := grpc.NewClient(ln.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { cc.Close() })
	return proto.NewWatchServiceClient(cc)
}

func recvOne(t *testing.T, stream proto.WatchService_WatchClient) *proto.WatchResponse {
	t.Helper()
	resp, err := stream.Recv()
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}
	return resp
}

// drainToSync consumes snapshot events until watch.sync and returns them.
func drainToSync(t *testing.T, stream proto.WatchService_WatchClient) (snapshot []*proto.WatchResponse, sync *proto.WatchResponse) {
	t.Helper()
	for i := 0; i < 32; i++ {
		r := recvOne(t, stream)
		if r.Type == events.TypeWatchSync {
			return snapshot, r
		}
		if r.Type == events.TypeWatchGap {
			continue
		}
		snapshot = append(snapshot, r)
	}
	t.Fatal("did not receive watch.sync")
	return nil, nil
}

func TestWatch_SnapshotSentOnConnect(t *testing.T) {
	bus := eventbus.New()
	state := &fakeWatchState{
		members: []membership.Member{
			{ID: "node-a", Address: "127.0.0.1:7945", Status: membership.StatusAlive},
			{ID: "node-b", Address: "127.0.0.1:7946", Status: membership.StatusAlive},
		},
		leaderID: "node-a",
	}
	client := startWatchServer(t, bus, state)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	stream, err := client.Watch(ctx, &proto.WatchRequest{})
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}

	snapshot, sync := drainToSync(t, stream)
	if sync.Type != events.TypeWatchSync {
		t.Fatalf("expected watch.sync, got %q", sync.Type)
	}

	types := make(map[string]int)
	for _, r := range snapshot {
		if r.Seq != 0 {
			t.Errorf("snapshot event should have seq=0, got %d", r.Seq)
		}
		types[r.Type]++
	}
	if types[events.TypeMemberJoin] != 2 {
		t.Errorf("expected 2 member.join, got %d", types[events.TypeMemberJoin])
	}
	if types[events.TypeLeaderChanged] != 1 {
		t.Errorf("expected 1 leader.changed, got %d", types[events.TypeLeaderChanged])
	}
}

func TestWatch_LiveEventsDelivered(t *testing.T) {
	bus := eventbus.New()
	state := &fakeWatchState{} // empty cluster
	client := startWatchServer(t, bus, state)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	stream, err := client.Watch(ctx, &proto.WatchRequest{})
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}

	_, _ = drainToSync(t, stream)

	go func() {
		time.Sleep(20 * time.Millisecond)
		bus.Publish(events.Event{Type: events.TypeMemberJoin, Source: "node-z"})
	}()

	resp := recvOne(t, stream)
	if resp.Type != events.TypeMemberJoin {
		t.Errorf("type: got %q, want member.join", resp.Type)
	}
	if resp.Source != "node-z" {
		t.Errorf("source: got %q, want node-z", resp.Source)
	}
	if resp.Seq == 0 {
		t.Error("live event should have seq > 0")
	}
}

func TestWatch_TypeFilterApplied(t *testing.T) {
	bus := eventbus.New()
	state := &fakeWatchState{}
	client := startWatchServer(t, bus, state)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Subscribe only to leader.changed events.
	stream, err := client.Watch(ctx, &proto.WatchRequest{
		EventTypes: []string{events.TypeLeaderChanged},
	})
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}

	_, _ = drainToSync(t, stream)

	go func() {
		time.Sleep(20 * time.Millisecond)
		bus.Publish(events.Event{Type: events.TypeMemberJoin, Source: "x"})
		time.Sleep(20 * time.Millisecond)
		bus.Publish(events.Event{Type: events.TypeLeaderChanged, Source: "node-1"})
	}()

	resp := recvOne(t, stream)
	if resp.Type != events.TypeLeaderChanged {
		t.Errorf("type: got %q, want leader.changed (member.join should be filtered)", resp.Type)
	}
}

func TestWatch_ReconnectGetsSnapshot(t *testing.T) {
	bus := eventbus.New()
	state := &fakeWatchState{
		members: []membership.Member{
			{ID: "node-a", Address: "127.0.0.1:7945", Status: membership.StatusAlive},
		},
		leaderID: "node-a",
	}
	client := startWatchServer(t, bus, state)

	{
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		stream, _ := client.Watch(ctx, &proto.WatchRequest{})
		_, _ = drainToSync(t, stream)
		cancel()
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()
	stream2, err := client.Watch(ctx2, &proto.WatchRequest{})
	if err != nil {
		t.Fatalf("Watch (reconnect): %v", err)
	}
	snapshot, _ := drainToSync(t, stream2)
	if len(snapshot) == 0 || snapshot[0].Type != events.TypeMemberJoin {
		t.Errorf("first snapshot event after reconnect: want member.join, got %+v", snapshot)
	}
}

func TestWatch_ReconnectResumesLive(t *testing.T) {
	bus := eventbus.New()
	state := &fakeWatchState{
		members: []membership.Member{
			{ID: "node-a", Address: "127.0.0.1:7945", Status: membership.StatusAlive},
		},
		leaderID: "node-a",
	}
	client := startWatchServer(t, bus, state)

	var lastSeq uint64
	{
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		stream, err := client.Watch(ctx, &proto.WatchRequest{})
		if err != nil {
			t.Fatalf("Watch: %v", err)
		}
		_, _ = drainToSync(t, stream)
		bus.Publish(events.Event{Type: events.TypeMemberJoin, Source: "node-b"})
		live := recvOne(t, stream)
		lastSeq = live.Seq
		cancel()
	}

	bus.Publish(events.Event{Type: events.TypeMemberJoin, Source: "node-c"})
	state.members = append(state.members, membership.Member{
		ID: "node-c", Address: "127.0.0.1:7947", Status: membership.StatusAlive,
	})

	ctx2, cancel2 := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel2()
	stream2, err := client.Watch(ctx2, &proto.WatchRequest{LastSeq: lastSeq})
	if err != nil {
		t.Fatalf("reconnect Watch: %v", err)
	}

	snapshot, sync := drainToSync(t, stream2)
	foundC := false
	for _, e := range snapshot {
		if e.Type == events.TypeMemberJoin && e.Source == "node-c" {
			foundC = true
		}
	}
	if !foundC {
		t.Fatal("reconnect snapshot missing node-c (state changed while disconnected)")
	}
	if sync.Seq < lastSeq {
		t.Errorf("watch.sync seq=%d should be >= last_seq=%d", sync.Seq, lastSeq)
	}

	gap := recvOne(t, stream2)
	if gap.Type != events.TypeWatchGap {
		t.Fatalf("after sync: got %q, want watch.gap", gap.Type)
	}

	bus.Publish(events.Event{Type: events.TypeMemberLeft, Source: "node-c"})
	live := recvOne(t, stream2)
	if live.Type != events.TypeMemberLeft {
		t.Errorf("resumed live: got %q, want member.left", live.Type)
	}
	if live.Seq <= lastSeq {
		t.Errorf("resumed live seq=%d should be > last_seq=%d", live.Seq, lastSeq)
	}
}

func TestWatch_GapAfterDroppedEvents(t *testing.T) {
	bus := eventbus.New()
	state := &fakeWatchState{}
	client := startWatchServer(t, bus, state)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream, err := client.Watch(ctx, &proto.WatchRequest{})
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	_, _ = drainToSync(t, stream)

	for i := 0; i < 4000; i++ {
		bus.Publish(events.Event{Type: events.TypeMemberJoin, Source: "flood"})
	}

	gotGap := make(chan struct{})
	go func() {
		for {
			resp, err := stream.Recv()
			if err != nil {
				return
			}
			if resp.Type == events.TypeWatchGap {
				close(gotGap)
				return
			}
		}
	}()

	// Unclog the stream, then publish one more event so Watch observes
	// the dropped flag and emits watch.gap.
	time.Sleep(50 * time.Millisecond)
	bus.Publish(events.Event{Type: events.TypeMemberJoin, Source: "probe"})

	select {
	case <-gotGap:
	case <-time.After(3 * time.Second):
		t.Fatal("expected watch.gap after dropped events")
	}
}
