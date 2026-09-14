package grpcserver

import (
	"context"

	"google.golang.org/grpc"

	pb "github.com/odurgut/clusdr/api/clusdr/v1alpha1"
)

// NodeInfo holds the identifying information the health service reports.
// Populated from config at startup; fields are read-only after registration.
type NodeInfo struct {
	NodeID    string
	ClusterID string
	Role      string // "standalone" | "follower" | "leader"
}

// healthService implements pb.HealthServiceServer.
type healthService struct {
	pb.UnimplementedHealthServiceServer
	info NodeInfo
}

// RegisterHealthService registers the HealthService on srv with the given node info.
func RegisterHealthService(srv *grpc.Server, info NodeInfo) {
	pb.RegisterHealthServiceServer(srv, &healthService{info: info})
}

// Health returns the current health and identity of this node.
func (h *healthService) Health(_ context.Context, _ *pb.HealthRequest) (*pb.HealthResponse, error) {
	return &pb.HealthResponse{
		NodeId:    h.info.NodeID,
		ClusterId: h.info.ClusterID,
		Role:      h.info.Role,
		Healthy:   true,
	}, nil
}
