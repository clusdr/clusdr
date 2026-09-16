package membership_test

import (
	"io"
	"log/slog"
	"testing"

	"github.com/clusdr/clusdr/internal/events"
	"github.com/clusdr/clusdr/internal/membership"
)

func nopLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError + 10}))
}

func TestEngine_InitialState(t *testing.T) {
	e := membership.New("node-1", "10.0.0.1:7947", nopLog())

	members := e.Members()
	if len(members) != 1 {
		t.Fatalf("members: got %d, want 1", len(members))
	}
	if members[0].ID != "node-1" {
		t.Errorf("id: got %q, want %q", members[0].ID, "node-1")
	}
	if members[0].Status != membership.StatusAlive {
		t.Errorf("status: got %q, want alive", members[0].Status)
	}
	if !members[0].Leader {
		t.Error("initial node should be leader")
	}
}

func TestEngine_Join(t *testing.T) {
	e := membership.New("node-1", "10.0.0.1:7947", nopLog())

	if _, err := e.Join("node-2", "10.0.0.2:7947"); err != nil {
		t.Fatalf("Join: %v", err)
	}

	members := e.Members()
	if len(members) != 2 {
		t.Fatalf("members: got %d, want 2", len(members))
	}

	found := false
	for _, m := range members {
		if m.ID == "node-2" {
			found = true
			if m.Status != membership.StatusAlive {
				t.Errorf("node-2 status: got %q, want alive", m.Status)
			}
		}
	}
	if !found {
		t.Error("node-2 not found after join")
	}
}

func TestEngine_JoinIdempotent(t *testing.T) {
	e := membership.New("node-1", "10.0.0.1:7947", nopLog())

	_, _ = e.Join("node-2", "10.0.0.2:7947")
	_, _ = e.Join("node-2", "10.0.0.2:7947") // second join should not duplicate

	if got := len(e.Members()); got != 2 {
		t.Errorf("members after double join: got %d, want 2", got)
	}
}

func TestEngine_Leave(t *testing.T) {
	e := membership.New("node-1", "10.0.0.1:7947", nopLog())
	_, _ = e.Join("node-2", "10.0.0.2:7947")

	e.Leave("node-2")

	for _, m := range e.Members() {
		if m.ID == "node-2" && m.Status != membership.StatusLeaving {
			t.Errorf("node-2 status: got %q, want leaving", m.Status)
		}
	}
}

func TestEngine_SetLeader(t *testing.T) {
	e := membership.New("node-1", "10.0.0.1:7947", nopLog())
	_, _ = e.Join("node-2", "10.0.0.2:7947")

	e.SetLeader("node-2")

	leader, ok := e.Leader()
	if !ok {
		t.Fatal("no leader found")
	}
	if leader.ID != "node-2" {
		t.Errorf("leader: got %q, want %q", leader.ID, "node-2")
	}

	for _, m := range e.Members() {
		if m.ID == "node-2" && !m.Leader {
			t.Error("node-2 should be leader")
		}
		if m.ID == "node-1" && m.Leader {
			t.Error("node-1 should not be leader after SetLeader(node-2)")
		}
	}
}

func TestEngine_SetLeaderIdempotent(t *testing.T) {
	e := membership.New("node-1", "10.0.0.1:7947", nopLog())
	_, _ = e.Join("node-2", "10.0.0.2:7947")

	var n int
	e.Emit = func(events.Event) { n++ }

	e.SetLeader("node-2")
	e.SetLeader("node-2")
	if n != 1 {
		t.Errorf("emits: got %d, want 1", n)
	}

	e.SetLeader("node-1")
	if n != 2 {
		t.Errorf("emits after real change: got %d, want 2", n)
	}
}

func TestEngine_JoinEmptyIDIsError(t *testing.T) {
	e := membership.New("node-1", "10.0.0.1:7947", nopLog())
	if _, err := e.Join("", "addr"); err == nil {
		t.Error("expected error for empty node id")
	}
}

func TestEngine_JoinAsObserver(t *testing.T) {
	e := membership.New("node-1", "10.0.0.1:7947", nopLog())
	if _, err := e.JoinAs("node-obs", "10.0.0.9:7947", membership.RoleObserver); err != nil {
		t.Fatalf("JoinAs: %v", err)
	}
	if _, err := e.JoinAs("empty-role", "10.0.0.8:7947", ""); err != nil {
		t.Fatalf("JoinAs empty role: %v", err)
	}

	got := map[string]string{}
	for _, m := range e.Members() {
		got[m.ID] = m.Role
	}
	if got["node-1"] != membership.RoleVoter {
		t.Errorf("seed role: got %q, want voter", got["node-1"])
	}
	if got["node-obs"] != membership.RoleObserver {
		t.Errorf("observer role: got %q, want observer", got["node-obs"])
	}
	if got["empty-role"] != membership.RoleVoter {
		t.Errorf("empty role: got %q, want voter", got["empty-role"])
	}
	if e.SelfRole() != membership.RoleVoter {
		t.Errorf("self role: got %q, want voter", e.SelfRole())
	}
}

func TestNormalizeRole(t *testing.T) {
	if membership.NormalizeRole("") != membership.RoleVoter {
		t.Fatal("empty should be voter")
	}
	if membership.NormalizeRole("unknown") != membership.RoleVoter {
		t.Fatal("unknown should be voter")
	}
	if membership.NormalizeRole(membership.RoleObserver) != membership.RoleObserver {
		t.Fatal("observer should stay observer")
	}
}
