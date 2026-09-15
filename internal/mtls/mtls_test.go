package mtls_test

import (
	"bytes"
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/durguto/clusdr/internal/mtls"
	"github.com/durguto/clusdr/internal/pki"
	pb "github.com/durguto/clusdr/api/clusdr/v1alpha1"
)

func TestWriteLoadFiles(t *testing.T) {
	dir := t.TempDir()
	wantCA, wantCert, wantKey := []byte("ca"), []byte("cert"), []byte("key")
	if err := mtls.WriteFiles(dir, wantCA, wantCert, wantKey); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(filepath.Join(dir, mtls.KeyFile))
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm()&0o077 != 0 {
		t.Errorf("node.key must not be group/world accessible: %o", st.Mode().Perm())
	}
	ca, cert, key, err := mtls.LoadFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ca, wantCA) || !bytes.Equal(cert, wantCert) || !bytes.Equal(key, wantKey) {
		t.Fatal("round-trip mismatch")
	}
}

func TestDial_SameCAAccepted_WrongCARejected(t *testing.T) {
	good, _, err := pki.Generate("cluster-a", "node-a")
	if err != nil {
		t.Fatal(err)
	}
	peerCert, peerKey, err := pki.IssueNodeCert(good.CACert, good.CAKey, "node-b")
	if err != nil {
		t.Fatal(err)
	}
	foreign, _, err := pki.Generate("cluster-other", "node-x")
	if err != nil {
		t.Fatal(err)
	}

	tlsCfg, err := mtls.ServerTLS(good.CACert, good.NodeCert, good.NodeKey)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer(grpc.Creds(credentials.NewTLS(tlsCfg)))
	pb.RegisterHealthServiceServer(srv, &healthStub{id: "node-a"})
	go srv.Serve(ln) //nolint:errcheck
	t.Cleanup(srv.GracefulStop)

	goodCreds, err := mtls.ClientTLS(good.CACert, peerCert, peerKey)
	if err != nil {
		t.Fatal(err)
	}
	cc, err := mtls.Dial(ln.Addr().String(), goodCreds)
	if err != nil {
		t.Fatal(err)
	}
	defer cc.Close()
	if _, err := pb.NewHealthServiceClient(cc).Health(context.Background(), &pb.HealthRequest{}); err != nil {
		t.Fatalf("same CA: %v", err)
	}

	badCreds, err := mtls.ClientTLS(foreign.CACert, foreign.NodeCert, foreign.NodeKey)
	if err != nil {
		t.Fatal(err)
	}
	bad, err := mtls.Dial(ln.Addr().String(), badCreds)
	if err != nil {
		t.Fatal(err)
	}
	defer bad.Close()
	if _, err := pb.NewHealthServiceClient(bad).Health(context.Background(), &pb.HealthRequest{}); err == nil {
		t.Fatal("wrong CA: expected handshake/RPC failure")
	}
}

type healthStub struct {
	pb.UnimplementedHealthServiceServer
	id string
}

func (h *healthStub) Health(context.Context, *pb.HealthRequest) (*pb.HealthResponse, error) {
	return &pb.HealthResponse{NodeId: h.id, Healthy: true}, nil
}
