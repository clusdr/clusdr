package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/clusdr/clusdr/internal/pki"
	"github.com/clusdr/clusdr/internal/store"
)

func TestJoinSec_CheckTokenAndIssue(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	j := &joinSec{store: st}
	if err := j.CheckToken("anything"); err != nil {
		t.Fatalf("no token configured: %v", err)
	}
	if _, _, _, _, err := j.IssueNode("n2"); err == nil {
		t.Fatal("issue without certs")
	}

	bundle, token, err := pki.Generate("cluster-a", "node-a")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SaveCerts(store.Certs{
		CACert:        bundle.CACert,
		CAKey:         bundle.CAKey,
		NodeCert:      bundle.NodeCert,
		NodeKey:       bundle.NodeKey,
		JoinTokenHash: bundle.JoinTokenHash,
	}); err != nil {
		t.Fatal(err)
	}
	if err := j.CheckToken("wrong"); err == nil {
		t.Fatal("bad token")
	}
	if err := j.CheckToken(token); err != nil {
		t.Fatal(err)
	}

	cert, key, ca, hash, err := j.IssueNode("n2")
	if err != nil {
		t.Fatal(err)
	}
	if len(cert) == 0 || len(key) == 0 || len(ca) == 0 || len(hash) == 0 {
		t.Fatal("empty issue")
	}

	if err := st.SaveJoinCerts(bundle.CACert, bundle.NodeCert, bundle.NodeKey, bundle.JoinTokenHash); err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, err := j.IssueNode("n3"); err == nil {
		t.Fatal("follower must not mint certs")
	}
}

func TestJoinBoot_SaveJoinCerts(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	bundle, _, err := pki.Generate("cluster-a", "node-a")
	if err != nil {
		t.Fatal(err)
	}

	empty := &joinBoot{store: st}
	if err := empty.SaveJoinCerts(bundle.CACert, bundle.NodeCert, bundle.NodeKey, bundle.JoinTokenHash); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(dir, "files")
	j := &joinBoot{store: st, dataDir: out}
	if err := j.SaveJoinCerts(bundle.CACert, bundle.NodeCert, bundle.NodeKey, bundle.JoinTokenHash); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "ca.crt")); err != nil {
		t.Fatal(err)
	}
}
