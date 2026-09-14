// Package bench is the synthetic cluster used by cmd/clusdr-bench.
package bench

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"
)

// Product targets from Phase 10.2.
const (
	TargetElection    = 500 * time.Millisecond
	TargetEventFanout = 100 * time.Millisecond
	TargetLockAcquire = 50 * time.Millisecond
)

// Result is one scenario's latency distribution.
type Result struct {
	Name    string          `json:"name"`
	N       int             `json:"n"`
	P50     time.Duration   `json:"-"`
	P95     time.Duration   `json:"-"`
	P99     time.Duration   `json:"-"`
	Max     time.Duration   `json:"-"`
	Target  time.Duration   `json:"-"`
	Samples []time.Duration `json:"-"`
	Passed  bool            `json:"passed"`
	Detail  string          `json:"detail,omitempty"`
}

// MarshalJSON emits millisecond fields so the CLI report is easy to parse.
func (r Result) MarshalJSON() ([]byte, error) {
	type out struct {
		Name     string  `json:"name"`
		N        int     `json:"n"`
		P50Ms    float64 `json:"p50_ms"`
		P95Ms    float64 `json:"p95_ms"`
		P99Ms    float64 `json:"p99_ms"`
		MaxMs    float64 `json:"max_ms"`
		TargetMs float64 `json:"target_ms,omitempty"`
		Passed   bool    `json:"passed"`
		Detail   string  `json:"detail,omitempty"`
	}
	return json.Marshal(out{
		Name:     r.Name,
		N:        r.N,
		P50Ms:    ms(r.P50),
		P95Ms:    ms(r.P95),
		P99Ms:    ms(r.P99),
		MaxMs:    ms(r.Max),
		TargetMs: ms(r.Target),
		Passed:   r.Passed,
		Detail:   r.Detail,
	})
}

// Report is the full bench run.
type Report struct {
	Results []Result `json:"results"`
	Passed  bool     `json:"passed"`
}

func summarize(name string, samples []time.Duration, target time.Duration) Result {
	r := Result{Name: name, N: len(samples), Samples: samples, Target: target, Passed: true}
	if len(samples) == 0 {
		r.Passed = target == 0
		r.Detail = "no samples"
		return r
	}
	sorted := slices.Clone(samples)
	slices.Sort(sorted)
	r.P50 = percentile(sorted, 50)
	r.P95 = percentile(sorted, 95)
	r.P99 = percentile(sorted, 99)
	r.Max = sorted[len(sorted)-1]
	if target > 0 && r.P95 > target {
		r.Passed = false
		r.Detail = fmt.Sprintf("p95 %s exceeds target %s", r.P95, target)
	}
	return r
}

func percentile(sorted []time.Duration, p int) time.Duration {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 100 {
		return sorted[n-1]
	}
	idx := (p*n+99)/100 - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= n {
		idx = n - 1
	}
	return sorted[idx]
}

func ms(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}

// WriteText prints a fixed-width table to w.
func (r Report) WriteText(w io.Writer) error {
	fmt.Fprintf(w, "%-12s %5s %10s %10s %10s %10s %10s %6s\n",
		"scenario", "n", "p50", "p95", "p99", "max", "target", "result")
	for _, res := range r.Results {
		target := "—"
		if res.Target > 0 {
			target = res.Target.String()
		}
		status := "PASS"
		if !res.Passed {
			status = "FAIL"
		}
		if res.N == 0 {
			status = "SKIP"
		}
		fmt.Fprintf(w, "%-12s %5d %10s %10s %10s %10s %10s %6s\n",
			res.Name, res.N, res.P50, res.P95, res.P99, res.Max, target, status)
		if res.Detail != "" && !res.Passed {
			fmt.Fprintf(w, "  %s\n", res.Detail)
		}
	}
	return nil
}

func (r Report) String() string {
	var b strings.Builder
	_ = r.WriteText(&b)
	return b.String()
}
