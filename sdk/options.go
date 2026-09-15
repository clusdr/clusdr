package clusdr

import (
	"os"
	"strings"
	"time"
)

const (
	defaultAddr           = "127.0.0.1:7947"
	defaultRequestTimeout = 10 * time.Second
	defaultReadyTimeout   = 10 * time.Second
	defaultWatchBuffer    = 64
)

type options struct {
	addr           string
	insecure       bool
	dataDir        string
	holder         string
	requestTimeout time.Duration
	readyTimeout   time.Duration
}

func defaultOptions() options {
	return options{
		requestTimeout: defaultRequestTimeout,
		readyTimeout:   defaultReadyTimeout,
	}
}

func (o *options) apply(opts []Option) {
	for _, opt := range opts {
		if opt != nil {
			opt(o)
		}
	}
}

// Option configures Local or Dial.
type Option func(*options)

// WithInsecure disables TLS (development; matches CLUSDR_TLS=disabled).
func WithInsecure() Option {
	return func(o *options) { o.insecure = true }
}

// WithDataDir is the directory containing ca.crt, node.crt, and node.key.
func WithDataDir(dir string) Option {
	return func(o *options) { o.dataDir = dir }
}

// WithHolder is the lock and lease identity sent to the daemon.
// Empty (default) generates a unique id for this connection so two apps on
// the same host cannot release each other's grants.
func WithHolder(id string) Option {
	return func(o *options) { o.holder = strings.TrimSpace(id) }
}

// WithRequestTimeout is used when the caller context has no deadline
// (default 10s). A context deadline always wins.
func WithRequestTimeout(d time.Duration) Option {
	return func(o *options) {
		if d > 0 {
			o.requestTimeout = d
		}
	}
}

func localAddr() string {
	if v := strings.TrimSpace(os.Getenv("CLUSDR_GRPC_ADDR")); v != "" {
		return v
	}
	return defaultAddr
}

func envInsecure() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("CLUSDR_TLS"))) {
	case "disabled", "off", "false", "0":
		return true
	default:
		return false
	}
}

func envDataDir() string {
	if v := strings.TrimSpace(os.Getenv("CLUSDR_DATA_DIR")); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home + "/.clusdr"
}
