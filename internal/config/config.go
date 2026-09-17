// Package config loads and validates Clusdr configuration.
//
// Configuration is resolved in the following order (last wins):
//  1. Built-in defaults
//  2. YAML config file
//  3. Environment variables (CLUSDR_*)
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration for the Clusdr daemon.
type Config struct {
	Node    NodeConfig    `yaml:"node"`
	Cluster ClusterConfig `yaml:"cluster"`
	Data    DataConfig    `yaml:"data"`
	Log     LogConfig     `yaml:"log"`
	GRPC    GRPCConfig    `yaml:"grpc"`
	// ShutdownTimeout is how long the daemon waits for units to close cleanly.
	// Defaults to 15s. Override via CLUSDR_SHUTDOWN_TIMEOUT (e.g. "30s").
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`

	Heartbeat HeartbeatConfig `yaml:"heartbeat"`
	Raft      RaftConfig      `yaml:"raft"`
	TLS       TLSConfig       `yaml:"tls"`
	Lock      LockConfig      `yaml:"lock"`
	Lease     LeaseConfig     `yaml:"lease"`
}

// TLSConfig controls gRPC transport security.
type TLSConfig struct {
	// Mode is "enabled" (default, production mTLS) or "disabled" (dev plaintext).
	// Override with CLUSDR_TLS=disabled.
	Mode string `yaml:"mode"`
}

// HeartbeatConfig controls peer liveness detection.
type HeartbeatConfig struct {
	// Interval is how often this node pings its peers. Default: 2s.
	Interval time.Duration `yaml:"interval"`
	// Timeout is the per-ping deadline. Default: 1s.
	Timeout time.Duration `yaml:"timeout"`
	// MaxMisses is consecutive failures before a peer is marked dead. Default: 3.
	MaxMisses int `yaml:"max_misses"`
}

// RaftConfig controls Raft consensus parameters.
type RaftConfig struct {
	// Addr is the TCP address this node binds for Raft peer communication.
	// Must be reachable by all other nodes. Default: 127.0.0.1:7946.
	Addr string `yaml:"addr"`
	// Bootstrap must be true on the first (seed) node to create a new cluster.
	// Idempotent: safe to leave true after the cluster is formed.
	Bootstrap bool `yaml:"bootstrap"`
	// HeartbeatTimeout is how often the leader contacts followers.
	// Default: 150ms (LAN). Raise on high-latency links.
	HeartbeatTimeout time.Duration `yaml:"heartbeat_timeout"`
	// ElectionTimeout is how long a follower waits before campaigning.
	// Default: 150ms. A leader failure elects a replacement in well under 500ms.
	ElectionTimeout time.Duration `yaml:"election_timeout"`
	// LeaderLeaseTimeout is how long a leader considers itself valid without
	// a quorum. Must be <= HeartbeatTimeout. Default: 75ms.
	LeaderLeaseTimeout time.Duration `yaml:"leader_lease_timeout"`
}

// LockConfig controls distributed lock TTL and expiry scanning.
type LockConfig struct {
	// TTL is the default grant lifetime when the client omits ttl_ms. Default: 15s.
	TTL time.Duration `yaml:"ttl"`
	// ExpireInterval is how often the leader scans for expired locks. Default: 100ms.
	ExpireInterval time.Duration `yaml:"expire_interval"`
}

// LeaseConfig controls named TTL lease grants and expiry scanning.
type LeaseConfig struct {
	// TTL is the default grant lifetime when the client omits ttl_ms. Default: 15s.
	TTL time.Duration `yaml:"ttl"`
	// ExpireInterval is how often the leader scans for expired leases. Default: 100ms.
	ExpireInterval time.Duration `yaml:"expire_interval"`
	// Presence, when true (default), has each node hold a presence.<id> lease.
	// Expiry is a faster dead-detection path than heartbeats.
	Presence bool `yaml:"presence"`
	// PresenceTTL is the presence lease lifetime. Default: 3s (shorter than heartbeat).
	PresenceTTL time.Duration `yaml:"presence_ttl"`
}

// NodeConfig identifies this node within the cluster.
type NodeConfig struct {
	// ID is a unique identifier for this node. Generated on init if empty.
	ID string `yaml:"id"`
	// Addr is the address this node listens on for cluster communication.
	Addr string `yaml:"addr"`
}

// ClusterConfig identifies the cluster this node belongs to.
type ClusterConfig struct {
	// ID is set once during clusdr init and must match across all nodes.
	ID string `yaml:"id"`
}

// DataConfig controls persistent storage location.
type DataConfig struct {
	// Dir is the directory for Raft log, state, and certificates.
	Dir string `yaml:"dir"`
}

// LogConfig controls logging behavior.
type LogConfig struct {
	// Level is one of: debug, info, warn, error.
	Level string `yaml:"level"`
	// Format is one of: text, json.
	Format string `yaml:"format"`
}

// GRPCConfig controls gRPC endpoint addresses.
type GRPCConfig struct {
	// Addr is the Runtime API TCP address (applications connect here).
	Addr string `yaml:"addr"`
	// ControlSocket is the Control API Unix socket path (CLI connects here).
	ControlSocket string `yaml:"control_socket"`
	// DialTimeout limits how long a client waits to establish a connection.
	DialTimeout time.Duration `yaml:"dial_timeout"`
	// RequestTimeout limits how long a single RPC may take.
	RequestTimeout time.Duration `yaml:"request_timeout"`
}

// Defaults returns a Config with sensible values for local development.
func Defaults() Config {
	return Config{
		Node: NodeConfig{
			Addr: "0.0.0.0:7947",
		},
		Data: DataConfig{
			Dir: defaultDataDir(),
		},
		Log: LogConfig{
			Level:  "info",
			Format: "text",
		},
		GRPC: GRPCConfig{
			Addr:           "127.0.0.1:7947",
			ControlSocket:  defaultControlSocket(),
			DialTimeout:    5 * time.Second,
			RequestTimeout: 10 * time.Second,
		},
		Heartbeat: HeartbeatConfig{
			Interval:  2 * time.Second,
			Timeout:   1 * time.Second,
			MaxMisses: 3,
		},
		Raft: RaftConfig{
			Addr:               "127.0.0.1:7946",
			Bootstrap:          false,
			HeartbeatTimeout:   150 * time.Millisecond,
			ElectionTimeout:    150 * time.Millisecond,
			LeaderLeaseTimeout: 75 * time.Millisecond,
		},
		TLS: TLSConfig{
			Mode: "enabled",
		},
		Lock: LockConfig{
			TTL:            15 * time.Second,
			ExpireInterval: 100 * time.Millisecond,
		},
		Lease: LeaseConfig{
			TTL:            15 * time.Second,
			ExpireInterval: 100 * time.Millisecond,
			Presence:       true,
			PresenceTTL:    3 * time.Second,
		},
	}
}

// TLSEnabled reports whether gRPC should speak TLS when certificates exist.
// "disabled", "off", "false", and "0" turn TLS off for development.
func (c Config) TLSEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(c.TLS.Mode)) {
	case "disabled", "off", "false", "0":
		return false
	default:
		return true
	}
}

// LoadFrom loads configuration by merging defaults, an optional YAML file,
// and environment variable overrides.
//
// getenv is injectable so tests do not touch os.Environ.
func LoadFrom(path string, getenv func(string) string) (Config, error) {
	cfg := Defaults()

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			return Config{}, fmt.Errorf("read config %q: %w", path, err)
		}
		if err == nil {
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				return Config{}, fmt.Errorf("parse config %q: %w", path, err)
			}
		}
	}

	applyEnv(&cfg, getenv)
	cfg.clampRaft()
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// clampRaft keeps leader_lease_timeout <= heartbeat_timeout when a caller
// overrides only one of the pair.
func (c *Config) clampRaft() {
	if c.Raft.HeartbeatTimeout > 0 && c.Raft.LeaderLeaseTimeout > c.Raft.HeartbeatTimeout {
		c.Raft.LeaderLeaseTimeout = c.Raft.HeartbeatTimeout / 2
		if c.Raft.LeaderLeaseTimeout <= 0 {
			c.Raft.LeaderLeaseTimeout = c.Raft.HeartbeatTimeout
		}
	}
}

// Validate reports inconsistent values after merge.
func (c Config) Validate() error {
	if c.Raft.HeartbeatTimeout > 0 && c.Raft.LeaderLeaseTimeout > c.Raft.HeartbeatTimeout {
		return fmt.Errorf("raft.leader_lease_timeout (%s) must be <= raft.heartbeat_timeout (%s)",
			c.Raft.LeaderLeaseTimeout, c.Raft.HeartbeatTimeout)
	}
	return nil
}

// applyEnv applies CLUSDR_* environment variable overrides to cfg.
func applyEnv(cfg *Config, getenv func(string) string) {
	if v := getenv("CLUSDR_NODE_ID"); v != "" {
		cfg.Node.ID = v
	}
	if v := getenv("CLUSDR_NODE_ADDR"); v != "" {
		cfg.Node.Addr = v
	}
	if v := getenv("CLUSDR_CLUSTER_ID"); v != "" {
		cfg.Cluster.ID = v
	}
	if v := getenv("CLUSDR_DATA_DIR"); v != "" {
		cfg.Data.Dir = v
	}
	if v := getenv("CLUSDR_LOG_LEVEL"); v != "" {
		cfg.Log.Level = v
	}
	if v := getenv("CLUSDR_LOG_FORMAT"); v != "" {
		cfg.Log.Format = v
	}
	if v := getenv("CLUSDR_GRPC_ADDR"); v != "" {
		cfg.GRPC.Addr = v
	}
	if v := getenv("CLUSDR_CONTROL_SOCKET"); v != "" {
		cfg.GRPC.ControlSocket = v
	}
	if v := getenv("CLUSDR_RAFT_ADDR"); v != "" {
		cfg.Raft.Addr = v
	}
	if v := getenv("CLUSDR_RAFT_BOOTSTRAP"); v == "true" || v == "1" {
		cfg.Raft.Bootstrap = true
	}
	envDuration(getenv, "CLUSDR_RAFT_HEARTBEAT_TIMEOUT", &cfg.Raft.HeartbeatTimeout)
	envDuration(getenv, "CLUSDR_RAFT_ELECTION_TIMEOUT", &cfg.Raft.ElectionTimeout)
	envDuration(getenv, "CLUSDR_RAFT_LEADER_LEASE_TIMEOUT", &cfg.Raft.LeaderLeaseTimeout)
	if v := getenv("CLUSDR_TLS"); v != "" {
		cfg.TLS.Mode = v
	}
	envDuration(getenv, "CLUSDR_LOCK_TTL", &cfg.Lock.TTL)
	envDuration(getenv, "CLUSDR_LOCK_EXPIRE_INTERVAL", &cfg.Lock.ExpireInterval)
	envDuration(getenv, "CLUSDR_LEASE_TTL", &cfg.Lease.TTL)
	envDuration(getenv, "CLUSDR_LEASE_EXPIRE_INTERVAL", &cfg.Lease.ExpireInterval)
	envBool(getenv, "CLUSDR_LEASE_PRESENCE", &cfg.Lease.Presence)
	envDuration(getenv, "CLUSDR_LEASE_PRESENCE_TTL", &cfg.Lease.PresenceTTL)
}

func envDuration(getenv func(string) string, key string, dest *time.Duration) {
	v := getenv(key)
	if v == "" {
		return
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return
	}
	*dest = d
}

func envBool(getenv func(string) string, key string, dest *bool) {
	v := strings.ToLower(strings.TrimSpace(getenv(key)))
	switch v {
	case "true", "1", "on", "enabled":
		*dest = true
	case "false", "0", "off", "disabled":
		*dest = false
	}
}

func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "/var/lib/clusdr"
	}
	return home + "/.clusdr"
}

func defaultControlSocket() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "/var/lib/clusdr/clusdr.sock"
	}
	return home + "/.clusdr/clusdr.sock"
}
