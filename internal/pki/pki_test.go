package pki_test

import (
	"bytes"
	"crypto/x509"
	"testing"

	"github.com/durguto/clusdr/internal/pki"
)

func TestGenerate_CAAndNodeCert(t *testing.T) {
	bundle, token, err := pki.Generate("cluster-1", "node-1")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if token == "" {
		t.Fatal("empty join token")
	}
	if len(token) != 64 { // 32 bytes hex
		t.Errorf("token length: got %d, want 64", len(token))
	}
	if !bytes.Equal(bundle.JoinTokenHash, pki.HashToken(token)) {
		t.Error("join token hash mismatch")
	}

	ca, err := pki.ParseCertificate(bundle.CACert)
	if err != nil {
		t.Fatalf("parse CA: %v", err)
	}
	if !ca.IsCA {
		t.Error("CA cert IsCA=false")
	}
	if ca.Subject.CommonName != "clusdr-ca-cluster-1" {
		t.Errorf("CA CN: got %q", ca.Subject.CommonName)
	}

	node, err := pki.ParseCertificate(bundle.NodeCert)
	if err != nil {
		t.Fatalf("parse node: %v", err)
	}
	if node.Subject.CommonName != "node-1" {
		t.Errorf("node CN: got %q", node.Subject.CommonName)
	}

	roots := x509.NewCertPool()
	roots.AddCert(ca)
	if _, err := node.Verify(x509.VerifyOptions{Roots: roots}); err != nil {
		t.Errorf("node cert not signed by CA: %v", err)
	}
}

func TestFingerprint_StableAndDistinct(t *testing.T) {
	a, _, err := pki.Generate("c1", "n1")
	if err != nil {
		t.Fatal(err)
	}
	fp1, err := pki.Fingerprint(a.CACert)
	if err != nil {
		t.Fatal(err)
	}
	fp2, err := pki.Fingerprint(a.CACert)
	if err != nil {
		t.Fatal(err)
	}
	if fp1 != fp2 {
		t.Error("fingerprint not stable")
	}
	if len(fp1) != len("sha256:")+64 {
		t.Errorf("fingerprint format: %q", fp1)
	}

	b, _, err := pki.Generate("c2", "n2")
	if err != nil {
		t.Fatal(err)
	}
	fpB, err := pki.Fingerprint(b.CACert)
	if err != nil {
		t.Fatal(err)
	}
	if fp1 == fpB {
		t.Error("two CAs produced the same fingerprint")
	}
}

func TestGenerate_RequiresIDs(t *testing.T) {
	if _, _, err := pki.Generate("", "n"); err == nil {
		t.Error("expected error for empty cluster id")
	}
	if _, _, err := pki.Generate("c", ""); err == nil {
		t.Error("expected error for empty node id")
	}
}

func TestCheckToken(t *testing.T) {
	_, token, err := pki.Generate("c", "n")
	if err != nil {
		t.Fatal(err)
	}
	hash := pki.HashToken(token)
	if !pki.CheckToken(hash, token) {
		t.Error("valid token rejected")
	}
	if pki.CheckToken(hash, "nope") {
		t.Error("invalid token accepted")
	}
	if !pki.CheckToken(nil, "anything") {
		t.Error("empty hash should accept (no token configured)")
	}
}

func TestIssueNodeCert_SignedByCA(t *testing.T) {
	bundle, _, err := pki.Generate("c", "seed")
	if err != nil {
		t.Fatal(err)
	}
	certPEM, _, err := pki.IssueNodeCert(bundle.CACert, bundle.CAKey, "joiner")
	if err != nil {
		t.Fatalf("IssueNodeCert: %v", err)
	}
	ca, err := pki.ParseCertificate(bundle.CACert)
	if err != nil {
		t.Fatal(err)
	}
	node, err := pki.ParseCertificate(certPEM)
	if err != nil {
		t.Fatal(err)
	}
	if node.Subject.CommonName != "joiner" {
		t.Errorf("CN: got %q", node.Subject.CommonName)
	}
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	if _, err := node.Verify(x509.VerifyOptions{Roots: roots}); err != nil {
		t.Errorf("verify: %v", err)
	}
}
