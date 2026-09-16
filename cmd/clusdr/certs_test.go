package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clusdr/clusdr/internal/mtls"
	"github.com/clusdr/clusdr/internal/pki"
	"github.com/clusdr/clusdr/internal/store"
)

func TestShowCerts_ReadsPEMFiles(t *testing.T) {
	dir := t.TempDir()
	bundle, _, err := pki.Generate("cluster-1", "node-1")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if err := mtls.WriteFiles(dir, bundle.CACert, bundle.NodeCert, bundle.NodeKey); err != nil {
		t.Fatalf("WriteFiles: %v", err)
	}
	st, err := store.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := st.SaveCerts(store.Certs{CACert: bundle.CACert, JoinTokenHash: bundle.JoinTokenHash}); err != nil {
		st.Close()
		t.Fatalf("SaveCerts: %v", err)
	}
	st.Close()

	var buf bytes.Buffer
	if err := showCerts(&buf, dir); err != nil {
		t.Fatalf("showCerts: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "cluster CA") {
		t.Errorf("missing CA block:\n%s", out)
	}
	if !strings.Contains(out, "node certificate") {
		t.Errorf("missing node block:\n%s", out)
	}
	if !strings.Contains(out, "configured (sha256:") {
		t.Errorf("missing token hash:\n%s", out)
	}
}

func TestShowCerts_StoreLockedStillPrintsCerts(t *testing.T) {
	dir := t.TempDir()
	bundle, _, err := pki.Generate("cluster-1", "node-1")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if err := mtls.WriteFiles(dir, bundle.CACert, bundle.NodeCert, bundle.NodeKey); err != nil {
		t.Fatalf("WriteFiles: %v", err)
	}
	st, err := store.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	var buf bytes.Buffer
	if err := showCerts(&buf, dir); err != nil {
		t.Fatalf("showCerts: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "cluster CA") {
		t.Errorf("missing CA block:\n%s", out)
	}
	if !strings.Contains(out, "join token     : (store locked)") {
		t.Errorf("want store locked, got:\n%s", out)
	}
}

func TestShowCerts_MissingCA(t *testing.T) {
	dir := t.TempDir()
	if err := showCerts(io.Discard, dir); err == nil {
		t.Fatal("expected error for missing ca.crt")
	}
	if _, err := os.Stat(filepath.Join(dir, "state.db")); err == nil {
		t.Error("missing CA must not create state.db")
	}
}
