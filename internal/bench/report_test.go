package bench

import (
	"testing"
	"time"
)

func TestPercentile(t *testing.T) {
	samples := make([]time.Duration, 100)
	for i := range samples {
		samples[i] = time.Duration(i+1) * time.Millisecond
	}
	r := summarize("x", samples, 0)
	if r.P50 != 50*time.Millisecond {
		t.Errorf("p50: got %s want 50ms", r.P50)
	}
	if r.Max != 100*time.Millisecond {
		t.Errorf("max: got %s want 100ms", r.Max)
	}
	if r.P95 < 90*time.Millisecond || r.P95 > 100*time.Millisecond {
		t.Errorf("p95 out of range: %s", r.P95)
	}
}

func TestSummarize_MissesTarget(t *testing.T) {
	samples := []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 80 * time.Millisecond}
	r := summarize("locks", samples, 50*time.Millisecond)
	if r.Passed {
		t.Fatalf("p95 %s should miss 50ms", r.P95)
	}
}

func TestSummarize_Empty(t *testing.T) {
	r := summarize("election", nil, TargetElection)
	if r.Passed {
		t.Fatal("empty targeted result should not pass")
	}
}
