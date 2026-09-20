package clusdr

import (
	"crypto/x509"
	"path/filepath"
	"testing"

	"google.golang.org/grpc/credentials/insecure"
)

func TestClientTLS_InvalidCA(t *testing.T) {
	if _, err := clientTLS([]byte("not-a-cert"), nil, nil); err == nil {
		t.Fatal("expected invalid CA")
	}
}

func TestClientFromDir_Missing(t *testing.T) {
	if _, err := clientFromDir(t.TempDir()); err == nil {
		t.Fatal("expected missing files")
	}
}

func TestTransportCreds_InsecureAndMissing(t *testing.T) {
	creds, err := transportCreds(options{insecure: true})
	if err != nil {
		t.Fatal(err)
	}
	if creds == nil {
		t.Fatal("insecure creds")
	}

	missing := filepath.Join(t.TempDir(), "no-such")
	if _, err = transportCreds(options{dataDir: missing}); err == nil {
		t.Fatal("missing cert dir should error")
	}

	t.Setenv("CLUSDR_DATA_DIR", "")
	t.Setenv("HOME", t.TempDir())
	if _, err = transportCreds(options{}); err == nil {
		t.Fatal("empty data dir should error")
	}
}

func TestVerifyPeer_EmptyAndGarbage(t *testing.T) {
	pool := x509.NewCertPool()
	if err := verifyPeer(pool, nil); err == nil {
		t.Fatal("empty chain")
	}
	if err := verifyPeer(pool, [][]byte{[]byte("not-der")}); err == nil {
		t.Fatal("garbage cert")
	}
}

func TestDialAddr_Lazy(t *testing.T) {
	conn, err := dialAddr("127.0.0.1:1", insecure.NewCredentials())
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
}
