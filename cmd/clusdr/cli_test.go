package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pb "github.com/clusdr/clusdr/api/clusdr/v1alpha1"
	"github.com/clusdr/clusdr/internal/config"
	"github.com/clusdr/clusdr/internal/mtls"
	"github.com/clusdr/clusdr/internal/pki"
)

func TestDisplayMemberRole(t *testing.T) {
	if got := displayMemberRole(&pb.Member{Leader: true, Role: "voter"}); got != "leader" {
		t.Fatalf("%q", got)
	}
	if got := displayMemberRole(&pb.Member{Role: "observer"}); got != "observer" {
		t.Fatalf("%q", got)
	}
	if got := displayMemberRole(&pb.Member{}); got != "voter" {
		t.Fatalf("%q", got)
	}
}

func TestStrOrEmpty(t *testing.T) {
	if got := strOrEmpty("", "fallback"); got != "fallback" {
		t.Fatalf("%q", got)
	}
	if got := strOrEmpty("id", "fallback"); got != "id" {
		t.Fatalf("%q", got)
	}
}

func TestLoadConfig_FlagOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "clusdr.yaml")
	if err := os.WriteFile(path, []byte("log:\n  level: info\n  format: json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(&rootFlags{configPath: path, logLevel: "debug", logFormat: "text"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Log.Level != "debug" || cfg.Log.Format != "text" {
		t.Fatalf("%+v", cfg.Log)
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	cfg, err := loadConfig(&rootFlags{configPath: filepath.Join(t.TempDir(), "missing.yaml")})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Log.Level == "" {
		t.Fatal("defaults")
	}
}

func TestRootCmd_Version(t *testing.T) {
	cmd := newRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"version"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "clusdr") {
		t.Fatalf("%q", buf.String())
	}
}

func TestRootCmd_ConfigValidate(t *testing.T) {
	cmd := newRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		"config", "validate",
		"--config", filepath.Join(t.TempDir(), "missing.yaml"),
		"--log-level", "warn",
		"--log-format", "json",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "resolved configuration") || !strings.Contains(out, "warn") {
		t.Fatalf("%s", out)
	}
}

func TestDaemonCreds(t *testing.T) {
	off := config.Config{TLS: config.TLSConfig{Mode: "disabled"}}
	if daemonCreds(off) != nil {
		t.Fatal("disabled")
	}

	missing := config.Config{
		TLS:  config.TLSConfig{Mode: "enabled"},
		Data: config.DataConfig{Dir: t.TempDir()},
	}
	if daemonCreds(missing) == nil {
		t.Fatal("bootstrap when certs missing")
	}

	dir := t.TempDir()
	bundle, _, err := pki.Generate("cluster-a", "node-a")
	if err != nil {
		t.Fatal(err)
	}
	if err := mtls.WriteFiles(dir, bundle.CACert, bundle.NodeCert, bundle.NodeKey); err != nil {
		t.Fatal(err)
	}
	ok := config.Config{
		TLS:  config.TLSConfig{Mode: "enabled"},
		Data: config.DataConfig{Dir: dir},
	}
	if daemonCreds(ok) == nil {
		t.Fatal("client certs")
	}
}
