package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInit_AlreadyInitialized(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "clusdr.yaml")
	data := filepath.Join(dir, "data")
	t.Setenv("CLUSDR_DATA_DIR", data)
	t.Setenv("HOME", dir)

	run := func() (string, error) {
		cmd := newRootCmd()
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs([]string{"init", "--config", cfg})
		err := cmd.Execute()
		return buf.String(), err
	}

	out, err := run()
	if err != nil {
		t.Fatalf("first init: %v\n%s", err, out)
	}
	if !strings.Contains(out, "join token") {
		t.Fatalf("first init missing token:\n%s", out)
	}

	out, err = run()
	if err != nil {
		t.Fatalf("second init: %v\n%s", err, out)
	}
	if !strings.Contains(out, "already initialized") {
		t.Fatalf("second init want already initialized, got:\n%s", out)
	}
	if strings.Contains(out, "join token") {
		t.Fatal("second init must not print a new join token")
	}

	if _, err := os.Stat(cfg); err != nil {
		t.Fatalf("config: %v", err)
	}
}
