// Package version provides build-time version information.
package version

// These variables are set at build time via -ldflags.
// See Makefile for the exact flags.
var (
	Version   = "dev"
	Commit    = "none"
	BuildTime = "unknown"
)
