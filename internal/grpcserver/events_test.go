package grpcserver_test

import (
	"bytes"
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/durguto/clusdr/internal/eventbus"
	"github.com/durguto/clusdr/internal/events"
	"github.com/durguto/clusdr/internal/grpcserver"
	"github.com/durguto/clusdr/internal/membership"
	proto "github.com/durguto/clusdr/api/clusdr/v1alpha1"
)

type eventNode struct {
	id    string
	addr  string
	bus   *eventbus.Bus
	mem   *membership.Engine
	pub   proto.EventServiceClient
	watch proto.WatchServiceClient
}

func startEventNode(t *testing.T, id string) *eventNode {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	log := nopLogger()
	bus := eventbus.New()
	mem := membership.New(id, addr, log)
	mem.Emit = bus.Publish

	srv := grpc.NewServer()
	grpcserver.RegisterWatchService(srv, bus, mem, log)
	grpcserver.RegisterEventService(srv, bus, mem, log, nil)
	go srv.Serve(ln) //nolint:errcheck
	t.Cleanup(srv.GracefulStop)

	cc, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { cc.Close() })

	return &eventNode{
		id:    id,
		addr:  addr,
		bus:   bus,
		mem:   mem,
		pub:   proto.NewEventServiceClient(cc),
		watch: proto.NewWatchServiceClient(cc),
	}
}

func TestPublishEvent_LocalWatchReceives(t *testing.T) {
	n := startEventNode(t, "node-a")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	stream, err := n.watch.Watch(ctx, &proto.WatchRequest{})
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	_, _ = drainToSync(t, stream)

	payload := []byte(`{"sha":"abc"}`)
	resp, err := n.pub.PublishEvent(ctx, &proto.PublishEventRequest{
		Topic:   "deployment",
		Payload: payload,
	})
	if err != nil {
		t.Fatalf("PublishEvent: %v", err)
	}
	if !resp.Accepted {
		t.Fatalf("not accepted: %s", resp.Message)
	}
	if resp.Type != "custom.deployment" {
		t.Errorf("type: got %q", resp.Type)
	}
	if resp.EventId == "" {
		t.Error("expected event_id")
	}

	got := recvOne(t, stream)
	if got.Type != events.CustomType("deployment") {
		t.Errorf("watch type: got %q", got.Type)
	}
	if got.Source != "node-a" {
		t.Errorf("source: got %q", got.Source)
	}
	if !bytes.Equal(got.Payload, payload) {
		t.Errorf("payload: got %q want %q", got.Payload, payload)
	}
	if got.Seq == 0 {
		t.Error("live custom event should have seq > 0")
	}
}

func TestPublishEvent_RejectsBadTopicAndPayload(t *testing.T) {
	n := startEventNode(t, "node-a")
	ctx := context.Background()

	_, err := n.pub.PublishEvent(ctx, &proto.PublishEventRequest{Topic: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("empty topic: got %v, want InvalidArgument", err)
	}

	_, err = n.pub.PublishEvent(ctx, &proto.PublishEventRequest{
		Topic:   "ok",
		Payload: make([]byte, events.MaxPayloadBytes+1),
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("huge payload: got %v, want InvalidArgument", err)
	}
}

func TestPublishEvent_DuplicateEventID(t *testing.T) {
	n := startEventNode(t, "node-a")
	ctx := context.Background()

	first, err := n.pub.PublishEvent(ctx, &proto.PublishEventRequest{
		Topic:   "ping",
		EventId: "same-id",
	})
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := n.pub.PublishEvent(ctx, &proto.PublishEventRequest{
		Topic:   "ping",
		EventId: "same-id",
	})
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if second.Message != "duplicate" {
		t.Errorf("duplicate message: got %q", second.Message)
	}
	if first.EventId != second.EventId {
		t.Errorf("event id mismatch")
	}
}

// TestPublishEvent_AtoB:
// publish on node A → node B's watcher receives it.
func TestPublishEvent_AtoB(t *testing.T) {
	a := startEventNode(t, "node-a")
	b := startEventNode(t, "node-b")
	a.mem.Join("node-b", b.addr) //nolint:errcheck
	b.mem.Join("node-a", a.addr) //nolint:errcheck

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := b.watch.Watch(ctx, &proto.WatchRequest{})
	if err != nil {
		t.Fatalf("Watch B: %v", err)
	}
	_, _ = drainToSync(t, stream)

	payload := []byte("hello-from-a")
	if _, err := a.pub.PublishEvent(ctx, &proto.PublishEventRequest{
		Topic:   "deployment",
		Payload: payload,
	}); err != nil {
		t.Fatalf("PublishEvent A: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := stream.Recv()
		if err != nil {
			t.Fatalf("B Recv: %v", err)
		}
		if resp.Type != events.CustomType("deployment") {
			continue
		}
		if resp.Source != "node-a" {
			t.Errorf("source: got %q, want node-a", resp.Source)
		}
		if !bytes.Equal(resp.Payload, payload) {
			t.Errorf("payload: got %q", resp.Payload)
		}
		return
	}
	t.Fatal("node B watcher did not receive custom.deployment from A")
}

// TestWatch_FilterByTopic:
// a watcher filtering topic "deployment" receives only deployment events.
func TestWatch_FilterByTopic(t *testing.T) {
	n := startEventNode(t, "node-a")

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	stream, err := n.watch.Watch(ctx, &proto.WatchRequest{
		Topics: []string{"deployment"},
	})
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}

	snapshot, sync := drainToSync(t, stream)
	if len(snapshot) != 0 {
		t.Errorf("topic watch should skip membership snapshot, got %d events", len(snapshot))
	}
	if sync.Type != events.TypeWatchSync {
		t.Fatalf("expected watch.sync, got %q", sync.Type)
	}

	if _, err := n.pub.PublishEvent(ctx, &proto.PublishEventRequest{
		Topic:   "other",
		Payload: []byte("nope"),
	}); err != nil {
		t.Fatalf("publish other: %v", err)
	}
	if _, err := n.pub.PublishEvent(ctx, &proto.PublishEventRequest{
		Topic:   "deployment",
		Payload: []byte("yes"),
	}); err != nil {
		t.Fatalf("publish deployment: %v", err)
	}

	got := recvOne(t, stream)
	if got.Type != events.CustomType("deployment") {
		t.Fatalf("got %q, want custom.deployment (other topic should be filtered)", got.Type)
	}
	if string(got.Payload) != "yes" {
		t.Errorf("payload: got %q", got.Payload)
	}
}

func TestWatch_InvalidTopicRejected(t *testing.T) {
	n := startEventNode(t, "node-a")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	stream, err := n.watch.Watch(ctx, &proto.WatchRequest{Topics: []string{"bad topic"}})
	if err != nil {
		if status.Code(err) == codes.InvalidArgument {
			return
		}
		t.Fatalf("Watch: %v", err)
	}
	_, err = stream.Recv()
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("Recv: got %v, want InvalidArgument", err)
	}
}
