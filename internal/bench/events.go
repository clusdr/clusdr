package bench

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	proto "github.com/durguto/clusdr/api/clusdr/v1alpha1"
	"github.com/durguto/clusdr/internal/eventbus"
	"github.com/durguto/clusdr/internal/events"
	"github.com/durguto/clusdr/internal/grpcserver"
	"github.com/durguto/clusdr/internal/membership"
)

type eventPeer struct {
	id   string
	addr string
	bus  *eventbus.Bus
	mem  *membership.Engine
	pub  proto.EventServiceClient
	cc   *grpc.ClientConn
	srv  *grpc.Server
}

func measureEvents(ctx context.Context, n, ops int) (Result, error) {
	peers, err := startEventMesh(n)
	if err != nil {
		return Result{}, err
	}
	defer closeEventMesh(peers)

	want := events.CustomType("bench")
	subs := make([]*eventbus.Subscription, len(peers))
	for i, p := range peers {
		subs[i] = p.bus.Subscribe(64)
	}
	defer func() {
		for _, s := range subs {
			s.Unsubscribe()
		}
	}()

	samples := make([]time.Duration, 0, ops)
	for i := 0; i < ops; i++ {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		for _, s := range subs {
			_ = s.Drain()
		}
		payload := []byte(strconv.Itoa(i))
		pubCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		start := time.Now()
		_, err := peers[0].pub.PublishEvent(pubCtx, &proto.PublishEventRequest{
			Topic:   "bench",
			Payload: payload,
		})
		cancel()
		if err != nil {
			return Result{}, fmt.Errorf("event sample %d: publish: %w", i, err)
		}
		if err := waitFanout(ctx, subs[1:], want, payload); err != nil {
			return Result{}, fmt.Errorf("event sample %d: %w", i, err)
		}
		samples = append(samples, time.Since(start))
	}
	return summarize("events", samples, TargetEventFanout), nil
}

func waitFanout(ctx context.Context, subs []*eventbus.Subscription, typ string, payload []byte) error {
	deadline, ok := ctx.Deadline()
	if !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		deadline, _ = ctx.Deadline()
	}
	pending := make([]bool, len(subs))
	left := len(subs)
	for left > 0 {
		if time.Now().After(deadline) {
			return fmt.Errorf("fanout: %d/%d peers still missing the event", left, len(subs))
		}
		for i, sub := range subs {
			if pending[i] {
				continue
			}
			select {
			case e := <-sub.C:
				if e.Type == typ && string(e.Payload) == string(payload) {
					pending[i] = true
					left--
				}
			default:
			}
		}
		if left > 0 {
			select {
			case <-ctx.Done():
				return fmt.Errorf("fanout: %w", ctx.Err())
			case <-time.After(time.Millisecond):
			}
		}
	}
	return nil
}

func startEventMesh(n int) ([]*eventPeer, error) {
	log := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError + 10}))
	peers := make([]*eventPeer, n)
	for i := 0; i < n; i++ {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			closeEventMesh(peers)
			return nil, fmt.Errorf("listen: %w", err)
		}
		id := fmt.Sprintf("evt-%d", i)
		addr := ln.Addr().String()
		bus := eventbus.New()
		mem := membership.New(id, addr, log)
		mem.Emit = bus.Publish
		srv := grpc.NewServer()
		grpcserver.RegisterWatchService(srv, bus, mem, log)
		grpcserver.RegisterEventService(srv, bus, mem, log, nil)
		go srv.Serve(ln) //nolint:errcheck

		cc, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			srv.Stop()
			closeEventMesh(peers)
			return nil, fmt.Errorf("dial %s: %w", id, err)
		}
		peers[i] = &eventPeer{
			id:   id,
			addr: addr,
			bus:  bus,
			mem:  mem,
			pub:  proto.NewEventServiceClient(cc),
			cc:   cc,
			srv:  srv,
		}
	}
	for i, a := range peers {
		for j, b := range peers {
			if i == j {
				continue
			}
			if _, err := a.mem.Join(b.id, b.addr); err != nil {
				closeEventMesh(peers)
				return nil, err
			}
		}
	}
	return peers, nil
}

func closeEventMesh(peers []*eventPeer) {
	for _, p := range peers {
		if p == nil {
			continue
		}
		if p.cc != nil {
			_ = p.cc.Close()
		}
		if p.srv != nil {
			p.srv.Stop()
		}
	}
}
