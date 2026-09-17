package main

import "testing"

func TestVersionRequested(t *testing.T) {
	t.Parallel()
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
