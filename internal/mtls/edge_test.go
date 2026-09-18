package mtls_test

import (
	"crypto/tls"
	"path/filepath"
	"testing"

	"github.com/clusdr/clusdr/internal/mtls"
	"github.com/clusdr/clusdr/internal/pki"
)

func TestDial_NilCreds(t *testing.T) {
	conn, err := mtls.Dial("127.0.0.1:1", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
}

func TestBootstrapTLS(t *testing.T) {
	if mtls.BootstrapTLS() == nil {
		t.Fatal("nil")
	}
}

func TestServerTLS_BadPEM(t *testing.T) {
	if _, err := mtls.ServerTLS([]byte("ca"), []byte("cert"), []byte("key")); err == nil {
		t.Fatal("expected error")
	}
}

func TestClientTLS_BadPEM(t *testing.T) {
	if _, err := mtls.ClientTLS([]byte("ca"), []byte("cert"), []byte("key")); err == nil {
		t.Fatal("expected error")
	}
}

func TestClientFromDir_Missing(t *testing.T) {
	if _, err := mtls.ClientFromDir(t.TempDir()); err == nil {
		t.Fatal("expected missing files")
	}
}

func TestLoadFiles_Missing(t *testing.T) {
	if _, _, _, err := mtls.LoadFiles(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("expected error")
	}
}

func TestReloadableServerTLS(t *testing.T) {
	if _, err := mtls.ReloadableServerTLS(nil); err == nil {
		t.Fatal("nil loader")
	}
	if _, err := mtls.ReloadableServerTLS(func() ([]byte, []byte, []byte, error) {
		return nil, nil, nil, nil
	}); err == nil {
		t.Fatal("bad material")
	}

	bundle, _, err := pki.Generate("cluster-a", "node-a")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := mtls.ReloadableServerTLS(func() ([]byte, []byte, []byte, error) {
		return bundle.CACert, bundle.NodeCert, bundle.NodeKey, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := cfg.GetConfigForClient(&tls.ClientHelloInfo{})
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("nil handshake config")
	}
}

func TestWriteFiles_EmptySlices(t *testing.T) {
	dir := t.TempDir()
	if err := mtls.WriteFiles(dir, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
}

func TestClientFromDir_OK(t *testing.T) {
	dir := t.TempDir()
	bundle, _, err := pki.Generate("cluster-a", "node-a")
	if err != nil {
		t.Fatal(err)
	}
	if err := mtls.WriteFiles(dir, bundle.CACert, bundle.NodeCert, bundle.NodeKey); err != nil {
		t.Fatal(err)
	}
	creds, err := mtls.ClientFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if creds == nil {
		t.Fatal("nil creds")
	}
}
