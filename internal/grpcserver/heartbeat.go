package grpcserver

import (
	"context"

	"google.golang.org/grpc"

	pb "github.com/clusdr/clusdr/api/clusdr/v1alpha1"
)

// RegisterHeartbeatService registers the liveness probe service on srv.
func RegisterHeartbeatService(srv *grpc.Server, nodeID string) {
	pb.RegisterHeartbeatServiceServer(srv, &heartbeatService{nodeID: nodeID})
}

type heartbeatService struct {
	pb.UnimplementedHeartbeatServiceServer
	nodeID string
}

// Ping responds to a liveness probe from a peer node.
func (h *heartbeatService) Ping(_ context.Context, _ *pb.PingRequest) (*pb.PingResponse, error) {
	return &pb.PingResponse{NodeId: h.nodeID, Alive: true}, nil
}
