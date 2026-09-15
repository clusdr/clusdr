package grpcserver_test

import (
	"context"
	"crypto/x509"
	"fmt"
	"net"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/durguto/clusdr/internal/grpcserver"
	"github.com/durguto/clusdr/internal/membership"
	"github.com/durguto/clusdr/internal/pki"
	pb "github.com/durguto/clusdr/api/clusdr/v1alpha1"
)

type testJoinSec struct {
	hash   []byte
	caCert []byte
	caKey  []byte
}

func (t *testJoinSec) CheckToken(token string) error {
	if !pki.CheckToken(t.hash, token) {
		return fmt.Errorf("unauthorized")
	}
	return nil
}

func (t *testJoinSec) IssueNode(nodeID string) (nodeCert, nodeKey, caCert, tokenHash []byte, err error) {
	nodeCert, nodeKey, err = pki.IssueNodeCert(t.caCert, t.caKey, nodeID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return nodeCert, nodeKey, t.caCert, t.hash, nil
}

func TestJoinService_WrongTokenUnauthorized(t *testing.T) {
	bundle, token, err := pki.Generate("cluster-sec", "node-a")
	if err != nil {
		t.Fatal(err)
	}
	engine := membership.New("node-a", "127.0.0.1:0", nopLogger())
	sec := &testJoinSec{hash: bundle.JoinTokenHash, caCert: bundle.CACert, caKey: bundle.CAKey}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpcserver.New(nopLogger(), grpcserver.DefaultConfig())
	grpcserver.RegisterJoinService(srv.GRPCServer(), "cluster-sec", engine, nil, nopLogger(), sec, nil)
	go srv.Serve(ln)                                         //nolint:errcheck
	t.Cleanup(func() { srv.Shutdown(context.Background()) }) //nolint:errcheck

	conn, err := grpc.NewClient(ln.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	client := pb.NewJoinServiceClient(conn)

	_, err = client.Join(context.Background(), &pb.JoinRequest{
		NodeId:    "node-b",
		ClusterId: "cluster-sec",
		Address:   "127.0.0.1:1",
		JoinToken: "wrong-token",
	})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("wrong token: got %v, want Unauthenticated", err)
	}
	if st, _ := status.FromError(err); st == nil || !strings.HasPrefix(st.Message(), "UNAUTHORIZED") {
		t.Errorf("message: got %v, want UNAUTHORIZED prefix", err)
	}

	resp, err := client.Join(context.Background(), &pb.JoinRequest{
		NodeId:    "node-b",
		ClusterId: "cluster-sec",
		Address:   "127.0.0.1:1",
		JoinToken: token,
	})
	if err != nil {
		t.Fatalf("correct token: %v", err)
	}
	if !resp.Accepted {
		t.Fatalf("rejected: %s", resp.Message)
	}
	if len(resp.NodeCert) == 0 || len(resp.NodeKey) == 0 {
		t.Fatal("expected issued node certificate")
	}

	ca, err := pki.ParseCertificate(resp.CaCert)
	if err != nil {
		t.Fatal(err)
	}
	node, err := pki.ParseCertificate(resp.NodeCert)
	if err != nil {
		t.Fatal(err)
	}
	if node.Subject.CommonName != "node-b" {
		t.Errorf("CN: got %q", node.Subject.CommonName)
	}
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	if _, err := node.Verify(x509.VerifyOptions{Roots: roots}); err != nil {
		t.Errorf("issued cert not signed by cluster CA: %v", err)
	}
}
