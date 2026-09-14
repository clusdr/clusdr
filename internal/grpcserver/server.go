// Package grpcserver manages the Clusdr gRPC endpoints.
//
// Two endpoints are served:
//   - Runtime API (TCP):   applications connect here to query the cluster.
//   - Control API (Unix):  the CLI connects here to manage the daemon.
//
// Both share the same gRPC server instance and service registrations.
// StartServe is called once per listener from the app unit.
package grpcserver

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

const (
	maxRecvMsgSize = 4 << 20  // 4 MiB
	maxSendMsgSize = 16 << 20 // 16 MiB
)

// Config holds gRPC server tunables, resolved from config.Config.
type Config struct {
	// MaxConnectionIdle is how long an idle client connection is kept alive.
	MaxConnectionIdle time.Duration
	// MaxConnectionAge is the maximum lifetime of any connection.
	MaxConnectionAge time.Duration
	// KeepaliveTime is how often the server sends keepalive pings.
	KeepaliveTime time.Duration
	// KeepaliveTimeout is how long the server waits for a ping ACK.
	KeepaliveTimeout time.Duration
	// TLS, when set, serves gRPC over TLS and requires a client certificate
	// for RPCs other than Join, Health, and gRPC reflection.
	TLS *tls.Config
}

// DefaultConfig returns safe production defaults.
func DefaultConfig() Config {
	return Config{
		MaxConnectionIdle: 30 * time.Minute,
		MaxConnectionAge:  1 * time.Hour,
		KeepaliveTime:     30 * time.Second,
		KeepaliveTimeout:  10 * time.Second,
	}
}

// Server wraps a *grpc.Server with structured logging and lifecycle helpers.
type Server struct {
	srv *grpc.Server
	log *slog.Logger
}

// New creates a gRPC server with logging and recovery interceptors.
// Register service implementations on the returned Server before calling Serve.
func New(log *slog.Logger, cfg Config) *Server {
	unary := []grpc.UnaryServerInterceptor{
		loggingInterceptor(log),
		recoveryInterceptor(log),
	}
	stream := []grpc.StreamServerInterceptor{
		streamLoggingInterceptor(log),
		streamRecoveryInterceptor(log),
	}
	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(maxRecvMsgSize),
		grpc.MaxSendMsgSize(maxSendMsgSize),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle: cfg.MaxConnectionIdle,
			MaxConnectionAge:  cfg.MaxConnectionAge,
			Time:              cfg.KeepaliveTime,
			Timeout:           cfg.KeepaliveTimeout,
		}),
	}
	if cfg.TLS != nil {
		unary = append([]grpc.UnaryServerInterceptor{peerCertUnaryInterceptor()}, unary...)
		stream = append([]grpc.StreamServerInterceptor{peerCertStreamInterceptor()}, stream...)
		opts = append(opts, grpc.Creds(credentials.NewTLS(cfg.TLS)))
	}
	opts = append(opts,
		grpc.ChainUnaryInterceptor(unary...),
		grpc.ChainStreamInterceptor(stream...),
	)
	srv := grpc.NewServer(opts...)
	// Reflection lets grpcurl and similar tools discover Watch/Join/Health
	// without shipping proto files. Health and Join stay callable without a
	// client certificate; other RPCs require mTLS when TLS is enabled.
	reflection.Register(srv)
	return &Server{srv: srv, log: log}
}

// GRPCServer returns the underlying *grpc.Server for service registration.
func (s *Server) GRPCServer() *grpc.Server {
	return s.srv
}

// Serve blocks serving requests on ln. Returns nil on graceful stop.
func (s *Server) Serve(ln net.Listener) error {
	s.log.Info("grpc listening", "addr", ln.Addr().String())
	return s.srv.Serve(ln)
}

// Shutdown gracefully stops the server, waiting for in-flight RPCs to finish.
func (s *Server) Shutdown(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		s.srv.GracefulStop()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		s.srv.Stop()
		return ctx.Err()
	}
}

// loggingInterceptor logs every unary RPC with method, duration, and status code.
func loggingInterceptor(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		code := status.Code(err)
		log.Debug("grpc unary",
			"method", info.FullMethod,
			"code", code.String(),
			"elapsed_ms", time.Since(start).Milliseconds(),
		)
		return resp, err
	}
}

// recoveryInterceptor catches panics in unary handlers and returns INTERNAL.
func recoveryInterceptor(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Error("grpc panic recovered", "method", info.FullMethod, "panic", r)
				err = status.Errorf(codes.Internal, "internal error")
			}
		}()
		return handler(ctx, req)
	}
}

// streamLoggingInterceptor logs stream RPC lifecycle.
func streamLoggingInterceptor(log *slog.Logger) grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		start := time.Now()
		err := handler(srv, ss)
		code := status.Code(err)
		log.Debug("grpc stream",
			"method", info.FullMethod,
			"code", code.String(),
			"elapsed_ms", time.Since(start).Milliseconds(),
		)
		return err
	}
}

// streamRecoveryInterceptor catches panics in stream handlers and returns INTERNAL.
func streamRecoveryInterceptor(log *slog.Logger) grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) (err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Error("grpc stream panic recovered", "method", info.FullMethod, "panic", r)
				err = status.Errorf(codes.Internal, "internal error")
			}
		}()
		return handler(srv, ss)
	}
}
