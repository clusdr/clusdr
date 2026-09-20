package soak

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestCapture_Audit(t *testing.T) {
	c := newCapture(nil)
	log := c.logger()
	log.Info("member.join")
	log.Info("member.left")
	log.Warn("lock expire apply")
	log.Error("boom")
	log.Info("unexpected-spam")
	log.Info("unexpected-spam")
	log.Info("unexpected-spam")

	fails := c.audit(time.Second, 2)
	if len(fails) == 0 {
		t.Fatal("expected audit failures")
	}
	foundErr, foundNoise := false, false
	for _, f := range fails {
		if strings.Contains(f, "error-level") || strings.Contains(f, "error: boom") {
			foundErr = true
		}
		if strings.Contains(f, "unexpected-spam") {
			foundNoise = true
		}
	}
	if !foundErr {
		t.Fatalf("missing error failure: %v", fails)
	}
	if !foundNoise {
		t.Fatalf("missing noise failure: %v", fails)
	}

	s := c.snapshot()
	if s.Info != 5 || s.Warn != 1 || s.Error != 1 {
		t.Fatalf("counts info=%d warn=%d error=%d", s.Info, s.Warn, s.Error)
	}

	// WithAttrs must still count on the shared state.
	log.With("node_id", "x").Info("member.join")
	s = c.snapshot()
	if s.ByMsg["member.join"] != 2 {
		t.Fatalf("with-attrs member.join=%d", s.ByMsg["member.join"])
	}

	_ = c.Enabled(context.Background(), slog.LevelInfo)
}

func TestCapture_AuditAllowsFSMSnapshotRestore(t *testing.T) {
	c := newCapture(nil)
	log := c.logger()
	for i := 0; i < 20; i++ {
		log.Info("fsm snapshot restored")
		log.Info("member.join")
	}
	fails := c.audit(time.Minute, 20)
	for _, f := range fails {
		if strings.Contains(f, "fsm snapshot restored") {
			t.Fatalf("snapshot restore is expected on extra join: %v", fails)
		}
	}
}
