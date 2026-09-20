package soak

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// Heap and goroutine growth allowed after warmup. Race + Raft snapshot
// buffers move the heap; a leak is unbounded growth, not a few MB.
const (
	MaxHeapGrowth      = 128 << 20
	MaxGoroutineGrowth = 48
)

// LogSummary is the captured slog traffic for the noise audit.
type LogSummary struct {
	Info  int            `json:"info"`
	Warn  int            `json:"warn"`
	Error int            `json:"error"`
	ByMsg map[string]int `json:"by_msg,omitempty"`
}

// Report is the full soak run.
type Report struct {
	Duration     time.Duration `json:"-"`
	Ticks        int           `json:"ticks"`
	Joins        int           `json:"joins"`
	Leaves       int           `json:"leaves"`
	Locks        int           `json:"locks"`
	LockExpires  int           `json:"lock_expires"`
	Leases       int           `json:"leases"`
	LeaseExpires int           `json:"lease_expires"`
	HeapStart    uint64        `json:"heap_start_bytes"`
	HeapEnd      uint64        `json:"heap_end_bytes"`
	HeapGrowth   uint64        `json:"heap_growth_bytes"`
	GoStart      int           `json:"goroutines_start"`
	GoEnd        int           `json:"goroutines_end"`
	Logs         LogSummary    `json:"logs"`
	Passed       bool          `json:"passed"`
	Failures     []string      `json:"failures,omitempty"`
}

// MarshalJSON emits duration as a string next to the counters.
func (r Report) MarshalJSON() ([]byte, error) {
	type out struct {
		Duration     string     `json:"duration"`
		Ticks        int        `json:"ticks"`
		Joins        int        `json:"joins"`
		Leaves       int        `json:"leaves"`
		Locks        int        `json:"locks"`
		LockExpires  int        `json:"lock_expires"`
		Leases       int        `json:"leases"`
		LeaseExpires int        `json:"lease_expires"`
		HeapStart    uint64     `json:"heap_start_bytes"`
		HeapEnd      uint64     `json:"heap_end_bytes"`
		HeapGrowth   uint64     `json:"heap_growth_bytes"`
		GoStart      int        `json:"goroutines_start"`
		GoEnd        int        `json:"goroutines_end"`
		Logs         LogSummary `json:"logs"`
		Passed       bool       `json:"passed"`
		Failures     []string   `json:"failures,omitempty"`
	}
	return json.Marshal(out{
		Duration:     r.Duration.String(),
		Ticks:        r.Ticks,
		Joins:        r.Joins,
		Leaves:       r.Leaves,
		Locks:        r.Locks,
		LockExpires:  r.LockExpires,
		Leases:       r.Leases,
		LeaseExpires: r.LeaseExpires,
		HeapStart:    r.HeapStart,
		HeapEnd:      r.HeapEnd,
		HeapGrowth:   r.HeapGrowth,
		GoStart:      r.GoStart,
		GoEnd:        r.GoEnd,
		Logs:         r.Logs,
		Passed:       r.Passed,
		Failures:     r.Failures,
	})
}

// WriteText prints a short operator report to w.
func (r Report) WriteText(w io.Writer) error {
	status := "PASS"
	if !r.Passed {
		status = "FAIL"
	}
	fmt.Fprintf(w, "soak %s  ticks=%d  join/leave=%d/%d  locks=%d (expire %d)  leases=%d (expire %d)\n",
		status, r.Ticks, r.Joins, r.Leaves, r.Locks, r.LockExpires, r.Leases, r.LeaseExpires)
	fmt.Fprintf(w, "  duration %s\n", r.Duration)
	fmt.Fprintf(w, "  heap %d -> %d (growth %d, cap %d)\n", r.HeapStart, r.HeapEnd, r.HeapGrowth, MaxHeapGrowth)
	fmt.Fprintf(w, "  goroutines %d -> %d (cap +%d)\n", r.GoStart, r.GoEnd, MaxGoroutineGrowth)
	fmt.Fprintf(w, "  logs info=%d warn=%d error=%d\n", r.Logs.Info, r.Logs.Warn, r.Logs.Error)
	for _, f := range r.Failures {
		fmt.Fprintf(w, "  fail: %s\n", f)
	}
	return nil
}

func (r Report) String() string {
	var b strings.Builder
	_ = r.WriteText(&b)
	return b.String()
}
