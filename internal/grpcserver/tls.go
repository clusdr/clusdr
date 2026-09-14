package grpcserver

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	pb "github.com/odurgut/clusdr/api/clusdr/v1alpha1"
)

func peerCertUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		if err := requirePeerCert(ctx, info.FullMethod); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

func peerCertStreamInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		if err := requirePeerCert(ss.Context(), info.FullMethod); err != nil {
			return err
		}
		return handler(srv, ss)
	}
}

func requirePeerCert(ctx context.Context, method string) error {
	if publicRPC(method) {
		return nil
	}
	p, ok := peer.FromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "mtls required")
	}
	ti, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok || len(ti.State.PeerCertificates) == 0 {
		return status.Error(codes.Unauthenticated, "mtls required")
	}
	return nil
}

func publicRPC(method string) bool {
	switch method {
	case pb.JoinService_Join_FullMethodName, pb.HealthService_Health_FullMethodName:
		return true
	}
	return strings.HasPrefix(method, "/grpc.reflection.")
}

func credsOf(fn func() credentials.TransportCredentials) credentials.TransportCredentials {
	if fn == nil {
		return nil
	}
	return fn()
}
