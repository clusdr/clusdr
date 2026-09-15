package grpcserver_test

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/durguto/clusdr/api/clusdr/v1alpha1"
	"github.com/durguto/clusdr/internal/grpcserver"
	"github.com/durguto/clusdr/internal/membership"
	"github.com/durguto/clusdr/internal/mtls"
	"github.com/durguto/clusdr/internal/pki"
)

func startMTLSServer(t *testing.T, ca, cert, key []byte) (*grpcserver.Server, string) {
	t.Helper()
	tlsCfg, err := mtls.ServerTLS(ca, cert, key)
	if err != nil {
		t.Fatal(err)
	}
	cfg := grpcserver.DefaultConfig()
	cfg.TLS = tlsCfg
	srv := grpcserver.New(nopLogger(), cfg)

	engine := membership.New("node-a", "127.0.0.1:0", nopLogger())
	grpcserver.RegisterHealthService(srv.GRPCServer(), grpcserver.NodeInfo{
		NodeID: "node-a", ClusterID: "cluster-a", Role: "standalone",
	})
	grpcserver.RegisterMembershipService(srv.GRPCServer(), engine)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go srv.Serve(ln) //nolint:errcheck
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })
	return srv, ln.Addr().String()
}

func TestMTLS_SameCertAllowed_WrongCertRejected(t *testing.T) {
	bundle, _, err := pki.Generate("cluster-a", "node-a")
	if err != nil {
		t.Fatal(err)
	}
	_, addr := startMTLSServer(t, bundle.CACert, bundle.NodeCert, bundle.NodeKey)

	good, err := mtls.ClientTLS(bundle.CACert, bundle.NodeCert, bundle.NodeKey)
	if err != nil {
		t.Fatal(err)
	}
	cc, err := mtls.Dial(addr, good)
	if err != nil {
		t.Fatal(err)
	}
	defer cc.Close()
	if _, err := pb.NewMembershipServiceClient(cc).ListMembers(context.Background(), &pb.ListMembersRequest{}); err != nil {
		t.Fatalf("same CA members: %v", err)
	}

	foreign, _, err := pki.Generate("cluster-other", "node-x")
	if err != nil {
		t.Fatal(err)
	}
	bad, err := mtls.ClientTLS(foreign.CACert, foreign.NodeCert, foreign.NodeKey)
	if err != nil {
		t.Fatal(err)
	}
	bcc, err := mtls.Dial(addr, bad)
	if err != nil {
		t.Fatal(err)
	}
	defer bcc.Close()
	if _, err := pb.NewMembershipServiceClient(bcc).ListMembers(context.Background(), &pb.ListMembersRequest{}); err == nil {
		t.Fatal("wrong CA: expected connection rejected")
	}
}

func TestMTLS_HealthWithoutClientCert_MembersRequiresCert(t *testing.T) {
	bundle, _, err := pki.Generate("cluster-a", "node-a")
	if err != nil {
		t.Fatal(err)
	}
	_, addr := startMTLSServer(t, bundle.CACert, bundle.NodeCert, bundle.NodeKey)

	cc, err := mtls.Dial(addr, mtls.BootstrapTLS())
	if err != nil {
		t.Fatal(err)
	}
	defer cc.Close()

	if _, err := pb.NewHealthServiceClient(cc).Health(context.Background(), &pb.HealthRequest{}); err != nil {
		t.Fatalf("health without client cert: %v", err)
	}

	_, err = pb.NewMembershipServiceClient(cc).ListMembers(context.Background(), &pb.ListMembersRequest{})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("members without cert: got %v, want Unauthenticated", err)
	}
}

func TestMTLS_JoinWithoutClientCert(t *testing.T) {
	bundle, token, err := pki.Generate("cluster-sec", "node-a")
	if err != nil {
		t.Fatal(err)
	}
	tlsCfg, err := mtls.ServerTLS(bundle.CACert, bundle.NodeCert, bundle.NodeKey)
	if err != nil {
		t.Fatal(err)
	}
	cfg := grpcserver.DefaultConfig()
	cfg.TLS = tlsCfg
	srv := grpcserver.New(nopLogger(), cfg)
	engine := membership.New("node-a", "127.0.0.1:0", nopLogger())
	sec := &testJoinSec{hash: bundle.JoinTokenHash, caCert: bundle.CACert, caKey: bundle.CAKey}
	grpcserver.RegisterJoinService(srv.GRPCServer(), "cluster-sec", engine, nil, nopLogger(), sec, nil)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go srv.Serve(ln) //nolint:errcheck
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	cc, err := mtls.Dial(ln.Addr().String(), mtls.BootstrapTLS())
	if err != nil {
		t.Fatal(err)
	}
	defer cc.Close()

	resp, err := pb.NewJoinServiceClient(cc).Join(context.Background(), &pb.JoinRequest{
		NodeId:    "node-b",
		ClusterId: "cluster-sec",
		Address:   "127.0.0.1:1",
		JoinToken: token,
	})
	if err != nil {
		t.Fatalf("join over bootstrap TLS: %v", err)
	}
	if !resp.Accepted || len(resp.NodeCert) == 0 {
		t.Fatalf("join: accepted=%v cert=%d", resp.Accepted, len(resp.NodeCert))
	}
}

func TestMTLS_DisabledServerAcceptsInsecure(t *testing.T) {
	// grpcserver.New with TLS unset must remain plaintext (unit tests / CLUSDR_TLS=disabled).
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpcserver.New(nopLogger(), grpcserver.DefaultConfig())
	grpcserver.RegisterHealthService(srv.GRPCServer(), grpcserver.NodeInfo{NodeID: "n"})
	go srv.Serve(ln) //nolint:errcheck
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	cc, err := mtls.Dial(ln.Addr().String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cc.Close()
	if _, err := pb.NewHealthServiceClient(cc).Health(context.Background(), &pb.HealthRequest{}); err != nil {
		t.Fatalf("insecure health: %v", err)
	}
}
