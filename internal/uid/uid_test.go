package uid_test

import (
	"strings"
	"testing"

	"github.com/clusdr/clusdr/internal/uid"
)

func TestNew_Format(t *testing.T) {
	id, err := uid.New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	parts := strings.Split(id, "-")
	if len(parts) != 5 {
		t.Errorf("expected 5 dash-separated parts, got %d: %q", len(parts), id)
	}
}

func TestNew_Unique(t *testing.T) {
	const n = 100
	seen := make(map[string]bool, n)
	for i := 0; i < n; i++ {
		id, err := uid.New()
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if seen[id] {
			t.Fatalf("duplicate id: %q", id)
		}
		seen[id] = true
	}
}
