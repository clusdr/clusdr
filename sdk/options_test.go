package clusdr

import (
	"testing"
	"time"
)

func TestLocalAddr_Env(t *testing.T) {
	t.Setenv("CLUSDR_GRPC_ADDR", " 127.0.0.1:9000 ")
	if got := localAddr(); got != "127.0.0.1:9000" {
		t.Fatalf("got %q", got)
	}
	t.Setenv("CLUSDR_GRPC_ADDR", "")
	if got := localAddr(); got != defaultAddr {
		t.Fatalf("default: %q", got)
	}
}

func TestEnvInsecure(t *testing.T) {
	on := []string{"disabled", "OFF", " false ", "0"}
	for _, v := range on {
		t.Setenv("CLUSDR_TLS", v)
		if !envInsecure() {
			t.Errorf("%q should be insecure", v)
		}
	}
	t.Setenv("CLUSDR_TLS", "enabled")
	if envInsecure() {
		t.Fatal("enabled must not be insecure")
	}
	t.Setenv("CLUSDR_TLS", "")
	if envInsecure() {
		t.Fatal("empty must not be insecure")
	}
}

func TestEnvDataDir(t *testing.T) {
	t.Setenv("CLUSDR_DATA_DIR", " /tmp/clusdr-data ")
	if got := envDataDir(); got != "/tmp/clusdr-data" {
		t.Fatalf("got %q", got)
	}
}

func TestApplyOptions(t *testing.T) {
	o := defaultOptions()
	o.apply([]Option{
		nil,
		WithInsecure(),
		WithDataDir("/data"),
		WithHolder("  app-1  "),
		WithRequestTimeout(0),
		WithRequestTimeout(3 * time.Second),
	})
	if !o.insecure || o.dataDir != "/data" || o.holder != "app-1" {
		t.Fatalf("%+v", o)
	}
	if o.requestTimeout != 3*time.Second {
		t.Fatalf("timeout %s", o.requestTimeout)
	}
}
