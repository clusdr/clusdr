package grpcserver

import (
	"context"
	"log/slog"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	pb "github.com/durguto/clusdr/api/clusdr/v1alpha1"
	"github.com/durguto/clusdr/internal/eventbus"
	"github.com/durguto/clusdr/internal/events"
	"github.com/durguto/clusdr/internal/membership"
	"github.com/durguto/clusdr/internal/mtls"
	"github.com/durguto/clusdr/internal/uid"
)

const seenEventCap = 4096

// PeerLister is the membership read surface used to fan out custom events.
type PeerLister interface {
	Members() []membership.Member
	SelfID() string
}

// RegisterEventService attaches EventService (PublishEvent) to srv.
func RegisterEventService(srv *grpc.Server, bus *eventbus.Bus, peers PeerLister, log *slog.Logger, peerCreds func() credentials.TransportCredentials) {
	pb.RegisterEventServiceServer(srv, &eventService{
		bus:       bus,
		peers:     peers,
		log:       log,
		seen:      newIDCache(seenEventCap),
		peerCreds: peerCreds,
	})
}

type eventService struct {
	pb.UnimplementedEventServiceServer
	bus       *eventbus.Bus
	peers     PeerLister
	log       *slog.Logger
	seen      *idCache
	peerCreds func() credentials.TransportCredentials
}

func (s *eventService) PublishEvent(_ context.Context, req *pb.PublishEventRequest) (*pb.PublishEventResponse, error) {
	if s.bus == nil {
		return nil, status.Error(codes.FailedPrecondition, "event bus not initialized")
	}
	if err := events.ValidTopic(req.GetTopic()); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	if len(req.GetPayload()) > events.MaxPayloadBytes {
		return nil, status.Errorf(codes.InvalidArgument,
			"payload exceeds %d bytes", events.MaxPayloadBytes)
	}

	eventID := req.GetEventId()
	if eventID == "" {
		id, err := uid.New()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "event id: %v", err)
		}
		eventID = id
	}

	if s.seen.seenOrAdd(eventID) {
		return &pb.PublishEventResponse{
			Accepted: true,
			Message:  "duplicate",
			EventId:  eventID,
			Type:     events.CustomType(req.GetTopic()),
		}, nil
	}

	source := req.GetSource()
	if source == "" && s.peers != nil {
		source = s.peers.SelfID()
	}

	typ := events.CustomType(req.GetTopic())
	s.bus.Publish(events.Event{
		Type:    typ,
		Source:  source,
		Payload: req.GetPayload(),
	})

	if !req.GetRelay() {
		s.fanout(req.GetTopic(), req.GetPayload(), eventID, source)
	}

	return &pb.PublishEventResponse{
		Accepted: true,
		EventId:  eventID,
		Type:     typ,
	}, nil
}

func (s *eventService) fanout(topic string, payload []byte, eventID, source string) {
	if s.peers == nil {
		return
	}
	selfID := s.peers.SelfID()
	for _, m := range s.peers.Members() {
		if m.ID == selfID || m.Status != membership.StatusAlive {
			continue
		}
		addr, id := m.Address, m.ID
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), fanoutTimeout)
			defer cancel()
			conn, err := mtls.Dial(addr, credsOf(s.peerCreds))
			if err != nil {
				s.log.Warn("event fanout dial failed", "target", id, "err", err)
				return
			}
			defer conn.Close()
			_, err = pb.NewEventServiceClient(conn).PublishEvent(ctx, &pb.PublishEventRequest{
				Topic:   topic,
				Payload: payload,
				EventId: eventID,
				Source:  source,
				Relay:   true,
			})
			if err != nil {
				s.log.Warn("event fanout failed", "target", id, "topic", topic, "err", err)
			}
		}()
	}
}

// idCache is a bounded FIFO set of recently seen event IDs.
type idCache struct {
	mu  sync.Mutex
	m   map[string]struct{}
	q   []string
	cap int
}

func newIDCache(n int) *idCache {
	if n <= 0 {
		n = 1024
	}
	return &idCache{m: make(map[string]struct{}, n), cap: n}
}

// seenOrAdd reports true if id was already present. Otherwise it records id
// and returns false. Evicts the oldest entry when full.
func (c *idCache) seenOrAdd(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.m[id]; ok {
		return true
	}
	if len(c.q) >= c.cap {
		old := c.q[0]
		c.q = c.q[1:]
		delete(c.m, old)
	}
	c.m[id] = struct{}{}
	c.q = append(c.q, id)
	return false
}
