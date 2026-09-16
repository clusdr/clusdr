package store_test

import (
	"errors"
	"testing"

	"github.com/clusdr/clusdr/internal/store"
)

func openTemp(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestStore_SaveAndLoadIdentity(t *testing.T) {
	s := openTemp(t)

	if err := s.SaveIdentity("node-1", "cluster-abc"); err != nil {
		t.Fatalf("SaveIdentity: %v", err)
	}

	nodeID, clusterID, err := s.Identity()
	if err != nil {
		t.Fatalf("Identity: %v", err)
	}
	if nodeID != "node-1" {
		t.Errorf("node_id: got %q, want %q", nodeID, "node-1")
	}
	if clusterID != "cluster-abc" {
		t.Errorf("cluster_id: got %q, want %q", clusterID, "cluster-abc")
	}
}

func TestStore_IdentityNotInitialized(t *testing.T) {
	s := openTemp(t)

	_, _, err := s.Identity()
	if !errors.Is(err, store.ErrNotInitialized) {
		t.Errorf("got %v, want ErrNotInitialized", err)
	}
}

func TestStore_OverwriteIdentity(t *testing.T) {
	s := openTemp(t)

	if err := s.SaveIdentity("old-node", "old-cluster"); err != nil {
		t.Fatalf("SaveIdentity: %v", err)
	}
	if err := s.SaveIdentity("new-node", "new-cluster"); err != nil {
		t.Fatalf("SaveIdentity overwrite: %v", err)
	}

	nodeID, clusterID, err := s.Identity()
	if err != nil {
		t.Fatalf("Identity: %v", err)
	}
	if nodeID != "new-node" {
		t.Errorf("node_id: got %q, want %q", nodeID, "new-node")
	}
	if clusterID != "new-cluster" {
		t.Errorf("cluster_id: got %q, want %q", clusterID, "new-cluster")
	}
}

func TestStore_PersistsAcrossReopen(t *testing.T) {
	dir := t.TempDir()

	s1, err := store.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := s1.SaveIdentity("persistent-node", "persistent-cluster"); err != nil {
		t.Fatalf("SaveIdentity: %v", err)
	}
	s1.Close()

	s2, err := store.Open(dir)
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	defer s2.Close()

	nodeID, clusterID, err := s2.Identity()
	if err != nil {
		t.Fatalf("Identity after reopen: %v", err)
	}
	if nodeID != "persistent-node" {
		t.Errorf("node_id: got %q, want %q", nodeID, "persistent-node")
	}
	if clusterID != "persistent-cluster" {
		t.Errorf("cluster_id: got %q, want %q", clusterID, "persistent-cluster")
	}
}

func TestStore_SaveAndLoadCerts(t *testing.T) {
	s := openTemp(t)

	in := store.Certs{
		CACert:        []byte("-----BEGIN CERTIFICATE-----\nCA\n-----END CERTIFICATE-----"),
		CAKey:         []byte("-----BEGIN PRIVATE KEY-----\nCAKEY\n-----END PRIVATE KEY-----"),
		NodeCert:      []byte("-----BEGIN CERTIFICATE-----\nNODE\n-----END CERTIFICATE-----"),
		NodeKey:       []byte("-----BEGIN PRIVATE KEY-----\nNODEKEY\n-----END PRIVATE KEY-----"),
		JoinTokenHash: []byte("hash-32-bytes-placeholder-value!"),
	}
	if err := s.SaveCerts(in); err != nil {
		t.Fatalf("SaveCerts: %v", err)
	}

	out, err := s.Certs()
	if err != nil {
		t.Fatalf("Certs: %v", err)
	}
	if string(out.CACert) != string(in.CACert) {
		t.Errorf("CACert mismatch")
	}
	if string(out.JoinTokenHash) != string(in.JoinTokenHash) {
		t.Errorf("JoinTokenHash mismatch")
	}
}

func TestStore_SaveJoinCertsRemovesCAKey(t *testing.T) {
	s := openTemp(t)
	if err := s.SaveCerts(store.Certs{
		CACert: []byte("old-ca"),
		CAKey:  []byte("old-key"),
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveJoinCerts([]byte("cluster-ca"), []byte("node-cert"), []byte("node-key"), []byte("hash")); err != nil {
		t.Fatalf("SaveJoinCerts: %v", err)
	}
	c, err := s.Certs()
	if err != nil {
		t.Fatal(err)
	}
	if string(c.CACert) != "cluster-ca" {
		t.Errorf("CACert: got %q", c.CACert)
	}
	if len(c.CAKey) != 0 {
		t.Error("CA key should be deleted on join")
	}
	if string(c.NodeCert) != "node-cert" {
		t.Errorf("NodeCert: got %q", c.NodeCert)
	}
}
