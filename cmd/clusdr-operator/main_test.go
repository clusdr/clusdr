package main

import (
	"path/filepath"
	"testing"
)

func TestVersionRequested(t *testing.T) {
	cases := []struct {
		args []string
		want bool
	}{
		{[]string{"version"}, true},
		{[]string{"--version"}, true},
		{[]string{"-version"}, true},
		{[]string{}, false},
		{[]string{"run"}, false},
		{[]string{"version", "extra"}, false},
	}
	for _, tc := range cases {
		if got := versionRequested(tc.args); got != tc.want {
			t.Fatalf("versionRequested(%q) = %v, want %v", tc.args, got, tc.want)
		}
	}
}

func TestKubeConfig_MissingFile(t *testing.T) {
	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "no-such"))
	if _, err := kubeConfig(); err == nil {
		t.Fatal("expected missing kubeconfig error")
	}
}
