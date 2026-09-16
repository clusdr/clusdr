package bench

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	proto "github.com/clusdr/clusdr/api/clusdr/v1alpha1"
	"github.com/clusdr/clusdr/internal/grpcserver"
	"github.com/clusdr/clusdr/internal/membership"
)

func measureMembers(ctx context.Context, members, ops int) (Result, error) {
	log := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError + 10}))
	mem := membership.New("node-0", "127.0.0.1:1", log)
	for i := 1; i < members; i++ {
		id := fmt.Sprintf("node-%d", i)
		if _, err := mem.Join(id, fmt.Sprintf("127.0.0.1:%d", 8000+i)); err != nil {
			return Result{}, err
		}
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return Result{}, err
	}
	srv := grpc.NewServer()
	grpcserver.RegisterMembershipService(srv, mem)
	go srv.Serve(ln) //nolint:errcheck
	defer srv.Stop()

	cc, err := grpc.NewClient(ln.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return Result{}, err
	}
	defer cc.Close()
	client := proto.NewMembershipServiceClient(cc)

	samples := make([]time.Duration, 0, ops)
	for i := 0; i < ops; i++ {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		start := time.Now()
		resp, err := client.ListMembers(rpcCtx, &proto.ListMembersRequest{})
		elapsed := time.Since(start)
		cancel()
		if err != nil {
			return Result{}, fmt.Errorf("members sample %d: %w", i, err)
		}
		if len(resp.Members) != members {
			return Result{}, fmt.Errorf("members sample %d: got %d want %d", i, len(resp.Members), members)
		}
		samples = append(samples, elapsed)
	}
	return summarize("members", samples, 0), nil
}
