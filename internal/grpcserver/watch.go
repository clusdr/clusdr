package grpcserver

import (
	"encoding/json"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	proto "github.com/clusdr/clusdr/api/clusdr/v1alpha1"
	"github.com/clusdr/clusdr/internal/eventbus"
	"github.com/clusdr/clusdr/internal/events"
	"github.com/clusdr/clusdr/internal/membership"
)

// WatchStater provides the current cluster state for the snapshot sent to
// every new watcher on connect.
type WatchStater interface {
	Members() []membership.Member
	LeaderID() string
}

// watchService implements WatchServiceServer.
type watchService struct {
	proto.UnimplementedWatchServiceServer
	bus   *eventbus.Bus
	state WatchStater
	log   *slog.Logger
}

// RegisterWatchService attaches the WatchService to a gRPC server.
func RegisterWatchService(srv *grpc.Server, bus *eventbus.Bus, state WatchStater, log *slog.Logger) {
	proto.RegisterWatchServiceServer(srv, &watchService{bus: bus, state: state, log: log})
}

// Watch streams cluster events to the connected client.
//
// Protocol:
//  1. Subscribe to the event bus BEFORE reading current state to avoid a
//     race where an event fires between snapshot and subscribe.
//  2. Send a snapshot of current cluster state (alive members + leader) as
//     synthetic events with seq=0.
//  3. Send watch.sync with the bus high-water mark. If the client provided
//     last_seq and the bus has moved past it, also send watch.gap.
//  4. Forward live events. If the subscriber dropped events (backpressure),
//     emit watch.gap before the next live event.
func (s *watchService) Watch(req *proto.WatchRequest, stream proto.WatchService_WatchServer) error {
	if err := validateWatchRequest(req); err != nil {
		return err
	}

	sub := s.bus.Subscribe(64)
	defer sub.Unsubscribe()

	fromSeq := s.bus.LastSeq()

	if err := s.sendSnapshot(req, stream); err != nil {
		return err
	}
	if err := s.sendSync(stream, fromSeq); err != nil {
		return err
	}
	if last := req.GetLastSeq(); last > 0 && fromSeq > last {
		if err := s.sendGap(stream, last+1, fromSeq); err != nil {
			return err
		}
	}

	var lastLive uint64
	for {
		select {
		case <-stream.Context().Done():
			return nil
		case <-sub.Done:
			return nil
		case e, ok := <-sub.C:
			if !ok {
				return nil
			}
			if !matchesWatch(req, e.Type) {
				continue
			}
			if sub.TakeDropped() {
				from := uint64(1)
				if lastLive > 0 {
					from = lastLive + 1
				}
				to := e.Seq - 1
				if e.Seq > 0 && from <= to {
					if err := s.sendGap(stream, from, to); err != nil {
						return err
					}
				}
			}
			if err := stream.Send(eventToProto(e)); err != nil {
				s.log.Debug("watch stream send error", "err", err)
				return err
			}
			lastLive = e.Seq
		}
	}
}

func (s *watchService) sendSync(stream proto.WatchService_WatchServer, seq uint64) error {
	return stream.Send(&proto.WatchResponse{
		Type:            events.TypeWatchSync,
		Seq:             seq,
		TimestampUnixMs: time.Now().UnixMilli(),
	})
}

func (s *watchService) sendGap(stream proto.WatchService_WatchServer, from, to uint64) error {
	payload, _ := json.Marshal(map[string]uint64{"from": from, "to": to})
	return stream.Send(&proto.WatchResponse{
		Type:            events.TypeWatchGap,
		Payload:         payload,
		TimestampUnixMs: time.Now().UnixMilli(),
	})
}

// sendSnapshot sends the current cluster state as synthetic watch events.
// Alive → member.join; dead → member.dead; current leader → leader.changed.
// All snapshot events carry Seq=0 so clients can distinguish them from live
// events. watch.sync (non-zero or zero high-water) follows the snapshot.
func (s *watchService) sendSnapshot(req *proto.WatchRequest, stream proto.WatchService_WatchServer) error {
	now := time.Now().UnixMilli()
	members := s.state.Members()
	leaderID := s.state.LeaderID()

	for _, m := range members {
		evType := events.TypeMemberJoin
		if membership.NormalizeStatus(m.Status) == membership.StatusDead {
			evType = events.TypeMemberDead
		} else if m.Status != membership.StatusAlive {
			continue
		}
		if !matchesWatch(req, evType) {
			continue
		}
		payload, _ := json.Marshal(map[string]any{
			"address":  m.Address,
			"snapshot": true,
		})
		resp := &proto.WatchResponse{
			Type:            evType,
			Source:          m.ID,
			Payload:         payload,
			TimestampUnixMs: now,
			Seq:             0,
		}
		if err := stream.Send(resp); err != nil {
			return err
		}
	}

	if leaderID != "" && matchesWatch(req, events.TypeLeaderChanged) {
		resp := &proto.WatchResponse{
			Type:            events.TypeLeaderChanged,
			Source:          leaderID,
			TimestampUnixMs: now,
			Seq:             0,
		}
		if err := stream.Send(resp); err != nil {
			return err
		}
	}

	return nil
}

func validateWatchRequest(req *proto.WatchRequest) error {
	for _, t := range req.GetTopics() {
		if err := events.ValidTopic(t); err != nil {
			return status.Errorf(codes.InvalidArgument, "topics: %v", err)
		}
	}
	return nil
}

// matchesWatch applies event_types and topics filters. Protocol events always pass.
// A non-empty topics list restricts the stream to custom.<topic> for those keys
// (membership events are omitted). event_types still filters by full type string.
func matchesWatch(req *proto.WatchRequest, eventType string) bool {
	if isProtocolEvent(eventType) {
		return true
	}
	if topics := req.GetTopics(); len(topics) > 0 {
		topic, ok := events.TopicFromType(eventType)
		if !ok || !topicListed(topics, topic) {
			return false
		}
	}
	return matchesType(req.GetEventTypes(), eventType)
}

func topicListed(topics []string, topic string) bool {
	for _, t := range topics {
		if events.NormalizeTopic(t) == topic {
			return true
		}
	}
	return false
}

// matchesType returns true when filter is empty (all types pass) or eventType
// is listed. Protocol events are handled by matchesWatch before this is called.
func matchesType(filter []string, eventType string) bool {
	if len(filter) == 0 {
		return true
	}
	for _, f := range filter {
		if f == eventType {
			return true
		}
	}
	return false
}

func isProtocolEvent(t string) bool {
	return t == events.TypeWatchSync || t == events.TypeWatchGap
}

// eventToProto converts an internal events.Event to a proto WatchResponse.
func eventToProto(e events.Event) *proto.WatchResponse {
	return &proto.WatchResponse{
		Type:            e.Type,
		Source:          e.Source,
		Payload:         e.Payload,
		TimestampUnixMs: e.Timestamp.UnixMilli(),
		Seq:             e.Seq,
	}
}
