package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/odurgut/clusdr/internal/config"
)

func TestDefaults(t *testing.T) {
	cfg := config.Defaults()

	if cfg.Node.Addr == "" {
		t.Error("default node addr must not be empty")
	}
	if cfg.Data.Dir == "" {
		t.Error("default data dir must not be empty")
	}
	if cfg.Log.Level == "" {
		t.Error("default log level must not be empty")
	}
	if cfg.Log.Format == "" {
		t.Error("default log format must not be empty")
	}
	if cfg.GRPC.DialTimeout == 0 {
		t.Error("default gRPC dial timeout must not be zero")
	}
	if cfg.GRPC.RequestTimeout == 0 {
		t.Error("default gRPC request timeout must not be zero")
	}
	if cfg.TLS.Mode != "enabled" {
		t.Errorf("default TLS mode: got %q, want enabled", cfg.TLS.Mode)
	}
	if !cfg.TLSEnabled() {
		t.Error("default TLS must be enabled")
	}
	if cfg.Lock.TTL != 15*time.Second {
		t.Errorf("default lock TTL: got %v, want 15s", cfg.Lock.TTL)
	}
	if cfg.Lock.ExpireInterval != 100*time.Millisecond {
		t.Errorf("default lock expire interval: got %v, want 100ms", cfg.Lock.ExpireInterval)
	}
	if cfg.Lease.TTL != 15*time.Second {
		t.Errorf("default lease TTL: got %v, want 15s", cfg.Lease.TTL)
	}
	if cfg.Lease.ExpireInterval != 100*time.Millisecond {
		t.Errorf("default lease expire interval: got %v, want 100ms", cfg.Lease.ExpireInterval)
	}
	if !cfg.Lease.Presence {
		t.Error("default lease presence must be enabled")
	}
	if cfg.Lease.PresenceTTL != 3*time.Second {
		t.Errorf("default presence TTL: got %v, want 3s", cfg.Lease.PresenceTTL)
	}
	if cfg.Raft.HeartbeatTimeout != 150*time.Millisecond {
		t.Errorf("default raft heartbeat: got %v, want 150ms", cfg.Raft.HeartbeatTimeout)
	}
	if cfg.Raft.ElectionTimeout != 150*time.Millisecond {
		t.Errorf("default raft election: got %v, want 150ms", cfg.Raft.ElectionTimeout)
	}
	if cfg.Raft.LeaderLeaseTimeout != 75*time.Millisecond {
		t.Errorf("default raft leader lease: got %v, want 75ms", cfg.Raft.LeaderLeaseTimeout)
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		if cfg.Data.Dir != "/var/lib/clusdr" {
			t.Errorf("data dir: got %q, want /var/lib/clusdr", cfg.Data.Dir)
		}
		if cfg.GRPC.ControlSocket != "/var/lib/clusdr/clusdr.sock" {
			t.Errorf("control socket: got %q, want /var/lib/clusdr/clusdr.sock", cfg.GRPC.ControlSocket)
		}
		return
	}
	if cfg.Data.Dir != home+"/.clusdr" {
		t.Errorf("data dir: got %q, want %q", cfg.Data.Dir, home+"/.clusdr")
	}
	if cfg.GRPC.ControlSocket != home+"/.clusdr/clusdr.sock" {
		t.Errorf("control socket: got %q, want %q", cfg.GRPC.ControlSocket, home+"/.clusdr/clusdr.sock")
	}
}

func TestValidate_RaftLeaseVsHeartbeat(t *testing.T) {
	cfg := config.Defaults()
	cfg.Raft.HeartbeatTimeout = 50 * time.Millisecond
	cfg.Raft.LeaderLeaseTimeout = 75 * time.Millisecond
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected lease > heartbeat to fail")
	}
}

func TestLoadFrom_RaftTimeoutEnv(t *testing.T) {
	cfg, err := config.LoadFrom("", func(k string) string {
		if k == "CLUSDR_RAFT_HEARTBEAT_TIMEOUT" {
			return "50ms"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Raft.HeartbeatTimeout != 50*time.Millisecond {
		t.Errorf("heartbeat: got %v", cfg.Raft.HeartbeatTimeout)
	}
	if cfg.Raft.LeaderLeaseTimeout > cfg.Raft.HeartbeatTimeout {
		t.Errorf("lease %s > heartbeat %s after clamp", cfg.Raft.LeaderLeaseTimeout, cfg.Raft.HeartbeatTimeout)
	}
}

func TestLoadFrom_EnvOverrides(t *testing.T) {
	env := map[string]string{
		"CLUSDR_NODE_ID":        "node-abc",
		"CLUSDR_NODE_ADDR":      "1.2.3.4:9000",
		"CLUSDR_CLUSTER_ID":     "cluster-xyz",
		"CLUSDR_DATA_DIR":       "/tmp/clusdr-test",
		"CLUSDR_LOG_LEVEL":      "debug",
		"CLUSDR_LOG_FORMAT":     "json",
		"CLUSDR_GRPC_ADDR":      "0.0.0.0:8000",
		"CLUSDR_CONTROL_SOCKET": "/tmp/clusdr.sock",
	}
	getenv := func(k string) string { return env[k] }

	cfg, err := config.LoadFrom("", getenv)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"node id", cfg.Node.ID, "node-abc"},
		{"node addr", cfg.Node.Addr, "1.2.3.4:9000"},
		{"cluster id", cfg.Cluster.ID, "cluster-xyz"},
		{"data dir", cfg.Data.Dir, "/tmp/clusdr-test"},
		{"log level", cfg.Log.Level, "debug"},
		{"log format", cfg.Log.Format, "json"},
		{"grpc addr", cfg.GRPC.Addr, "0.0.0.0:8000"},
		{"control socket", cfg.GRPC.ControlSocket, "/tmp/clusdr.sock"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, tt.got, tt.want)
		}
	}
}

func TestLoadFrom_YAMLFile(t *testing.T) {
	yaml := `
node:
  id: yaml-node
  addr: 10.0.0.1:7947
log:
  level: warn
`
	dir := t.TempDir()
	path := filepath.Join(dir, "clusdr.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadFrom(path, func(string) string { return "" })
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}

	if cfg.Node.ID != "yaml-node" {
		t.Errorf("node id: got %q, want %q", cfg.Node.ID, "yaml-node")
	}
	if cfg.Node.Addr != "10.0.0.1:7947" {
		t.Errorf("node addr: got %q, want %q", cfg.Node.Addr, "10.0.0.1:7947")
	}
	if cfg.Log.Level != "warn" {
		t.Errorf("log level: got %q, want %q", cfg.Log.Level, "warn")
	}
	// Defaults should still apply for unset fields.
	if cfg.GRPC.DialTimeout == 0 {
		t.Error("gRPC dial timeout should fall back to default")
	}
}

func TestLoadFrom_EnvOverridesYAML(t *testing.T) {
	yaml := `
node:
  id: from-yaml
`
	dir := t.TempDir()
	path := filepath.Join(dir, "clusdr.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}

	getenv := func(k string) string {
		if k == "CLUSDR_NODE_ID" {
			return "from-env"
		}
		return ""
	}

	cfg, err := config.LoadFrom(path, getenv)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}

	if cfg.Node.ID != "from-env" {
		t.Errorf("env should override yaml: got %q, want %q", cfg.Node.ID, "from-env")
	}
}

func TestLoadFrom_MissingFileIsOK(t *testing.T) {
	_, err := config.LoadFrom("/nonexistent/clusdr.yaml", func(string) string { return "" })
	if err != nil {
		t.Errorf("missing config file should not be an error: %v", err)
	}
}

func TestTLSEnabled(t *testing.T) {
	tests := []struct {
		mode string
		want bool
	}{
		{"", true},
		{"enabled", true},
		{"ENABLED", true},
		{"disabled", false},
		{"off", false},
		{"false", false},
		{"0", false},
	}
	for _, tt := range tests {
		cfg := config.Config{TLS: config.TLSConfig{Mode: tt.mode}}
		if got := cfg.TLSEnabled(); got != tt.want {
			t.Errorf("mode %q: TLSEnabled=%v, want %v", tt.mode, got, tt.want)
		}
	}
}

func TestLoadFrom_CLUSDR_TLS(t *testing.T) {
	cfg, err := config.LoadFrom("", func(k string) string {
		if k == "CLUSDR_TLS" {
			return "disabled"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TLS.Mode != "disabled" {
		t.Errorf("mode: got %q, want disabled", cfg.TLS.Mode)
	}
	if cfg.TLSEnabled() {
		t.Error("CLUSDR_TLS=disabled should turn TLS off")
	}
}

func TestLoadFrom_LockEnv(t *testing.T) {
	cfg, err := config.LoadFrom("", func(k string) string {
		switch k {
		case "CLUSDR_LOCK_TTL":
			return "30s"
		case "CLUSDR_LOCK_EXPIRE_INTERVAL":
			return "50ms"
		default:
			return ""
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Lock.TTL != 30*time.Second {
		t.Errorf("ttl: got %v, want 30s", cfg.Lock.TTL)
	}
	if cfg.Lock.ExpireInterval != 50*time.Millisecond {
		t.Errorf("expire interval: got %v, want 50ms", cfg.Lock.ExpireInterval)
	}
}

func TestLoadFrom_LeaseEnv(t *testing.T) {
	cfg, err := config.LoadFrom("", func(k string) string {
		switch k {
		case "CLUSDR_LEASE_TTL":
			return "8s"
		case "CLUSDR_LEASE_EXPIRE_INTERVAL":
			return "25ms"
		case "CLUSDR_LEASE_PRESENCE":
			return "false"
		case "CLUSDR_LEASE_PRESENCE_TTL":
			return "1s"
		default:
			return ""
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Lease.TTL != 8*time.Second {
		t.Errorf("ttl: got %v, want 8s", cfg.Lease.TTL)
	}
	if cfg.Lease.ExpireInterval != 25*time.Millisecond {
		t.Errorf("expire interval: got %v, want 25ms", cfg.Lease.ExpireInterval)
	}
	if cfg.Lease.Presence {
		t.Error("CLUSDR_LEASE_PRESENCE=false should disable presence")
	}
	if cfg.Lease.PresenceTTL != time.Second {
		t.Errorf("presence ttl: got %v, want 1s", cfg.Lease.PresenceTTL)
	}
}
