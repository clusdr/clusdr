package mtls

import (
	"testing"

	"github.com/clusdr/clusdr/internal/pki"
)

func TestVerifyPeer_EmptyGarbageAndWrongCA(t *testing.T) {
	good, _, err := pki.Generate("cluster-a", "node-a")
	if err != nil {
		t.Fatal(err)
	}
	pool, err := poolFromPEM(good.CACert)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyPeer(pool, nil); err == nil {
		t.Fatal("empty")
	}
	if err := verifyPeer(pool, [][]byte{[]byte("not-der")}); err == nil {
		t.Fatal("garbage")
	}

	peer, err := pki.ParseCertificate(good.NodeCert)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyPeer(pool, [][]byte{peer.Raw}); err != nil {
		t.Fatalf("same CA: %v", err)
	}

	foreign, _, err := pki.Generate("cluster-b", "node-x")
	if err != nil {
		t.Fatal(err)
	}
	other, err := pki.ParseCertificate(foreign.NodeCert)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyPeer(pool, [][]byte{other.Raw}); err == nil {
		t.Fatal("wrong CA")
	}
}

func TestPoolFromPEM_Invalid(t *testing.T) {
	if _, err := poolFromPEM([]byte("nope")); err == nil {
		t.Fatal("expected error")
	}
}

func TestVerifyPeer_IntermediateGarbage(t *testing.T) {
	good, _, err := pki.Generate("cluster-a", "node-a")
	if err != nil {
		t.Fatal(err)
	}
	pool, err := poolFromPEM(good.CACert)
	if err != nil {
		t.Fatal(err)
	}
	peer, err := pki.ParseCertificate(good.NodeCert)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyPeer(pool, [][]byte{peer.Raw, []byte("not-der")}); err == nil {
		t.Fatal("garbage intermediate")
	}
}
