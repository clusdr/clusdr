package soak

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestReport_TextAndJSON(t *testing.T) {
	r := Report{
		Duration:   2 * time.Second,
		Ticks:      3,
		Joins:      3,
		Leaves:     3,
		Locks:      3,
		Leases:     3,
		HeapStart:  10,
		HeapEnd:    12,
		HeapGrowth: 2,
		GoStart:    8,
		GoEnd:      8,
		Logs:       LogSummary{Info: 4},
		Passed:     true,
	}
	text := r.String()
	if !strings.Contains(text, "PASS") || !strings.Contains(text, "ticks=3") {
		t.Fatalf("text: %s", text)
	}
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"ticks":3`) {
		t.Fatalf("json: %s", raw)
	}

	r.Passed = false
	r.Failures = []string{"heap grew"}
	if !strings.Contains(r.String(), "fail: heap grew") {
		t.Fatalf("fail text: %s", r.String())
	}
}
