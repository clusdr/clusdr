package soak

import (
	"context"
	"log/slog"
	"runtime"
	"slices"
	"strconv"
	"sync"
	"time"
)

// Messages that join/leave and Raft leadership are expected to emit at Info.
var expectedInfo = map[string]struct{}{
	"raft bootstrap complete": {},
	"raft voter added":        {},
	"raft server removed":     {},
	"raft leadership changed": {},
	"member.join":             {},
	"member.left":             {},
	"leader.changed":          {},
}

// Expire apply can warn when leadership moves mid-scan. A healthy soak
// should almost never hit these.
var expectedWarn = map[string]struct{}{
	"lock expire apply":  {},
	"lease expire apply": {},
}

type logRecord struct {
	Level slog.Level
	Msg   string
}

type captureState struct {
	mu    sync.Mutex
	byMsg map[string]int
	byLvl map[slog.Level]int
	warns []logRecord
}

type capture struct {
	next  slog.Handler
	state *captureState
}

func newCapture(next slog.Handler) *capture {
	return &capture{
		next: next,
		state: &captureState{
			byMsg: make(map[string]int),
			byLvl: make(map[slog.Level]int),
		},
	}
}

func (c *capture) Enabled(ctx context.Context, level slog.Level) bool {
	if c.next != nil && c.next.Enabled(ctx, level) {
		return true
	}
	return level >= slog.LevelInfo
}

func (c *capture) Handle(ctx context.Context, r slog.Record) error {
	s := c.state
	s.mu.Lock()
	s.byLvl[r.Level]++
	s.byMsg[r.Message]++
	if r.Level >= slog.LevelWarn {
		s.warns = append(s.warns, logRecord{Level: r.Level, Msg: r.Message})
	}
	s.mu.Unlock()
	if c.next != nil && c.next.Enabled(ctx, r.Level) {
		return c.next.Handle(ctx, r)
	}
	return nil
}

func (c *capture) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := c.next
	if next != nil {
		next = next.WithAttrs(attrs)
	}
	return &capture{next: next, state: c.state}
}

func (c *capture) WithGroup(name string) slog.Handler {
	next := c.next
	if next != nil {
		next = next.WithGroup(name)
	}
	return &capture{next: next, state: c.state}
}

func (c *capture) logger() *slog.Logger {
	return slog.New(c)
}

func (c *capture) snapshot() LogSummary {
	s := c.state
	s.mu.Lock()
	defer s.mu.Unlock()
	out := LogSummary{
		Info:  s.byLvl[slog.LevelInfo],
		Warn:  s.byLvl[slog.LevelWarn],
		Error: s.byLvl[slog.LevelError],
		ByMsg: make(map[string]int, len(s.byMsg)),
	}
	for k, v := range s.byMsg {
		out.ByMsg[k] = v
	}
	return out
}

func (c *capture) audit(dur time.Duration, ticks int) []string {
	sum := c.snapshot()
	var fails []string
	if sum.Error > 0 {
		fails = append(fails, "error-level log records: "+strconv.Itoa(sum.Error))
	}
	maxWarn := 2 + int(dur/time.Minute)
	if sum.Warn > maxWarn {
		fails = append(fails, "warn-level records "+strconv.Itoa(sum.Warn)+" exceed "+strconv.Itoa(maxWarn))
	}
	c.state.mu.Lock()
	warns := slices.Clone(c.state.warns)
	c.state.mu.Unlock()
	for _, w := range warns {
		if w.Level >= slog.LevelError {
			fails = append(fails, "error: "+w.Msg)
			continue
		}
		if _, ok := expectedWarn[w.Msg]; !ok {
			fails = append(fails, "unexpected warn: "+w.Msg)
		}
	}
	if ticks < 1 {
		ticks = 1
	}
	for msg, n := range sum.ByMsg {
		if msg == "member.dead" {
			fails = append(fails, "member.dead during soak: "+strconv.Itoa(n))
			continue
		}
		if _, ok := expectedInfo[msg]; ok {
			continue
		}
		if _, ok := expectedWarn[msg]; ok {
			continue
		}
		// Unknown Info should stay rare: at most one per two ticks.
		if n > (ticks+1)/2 {
			fails = append(fails, "noisy message "+msg+": "+strconv.Itoa(n)+" in "+strconv.Itoa(ticks)+" ticks")
		}
	}
	return unique(fails)
}

type memSnap struct {
	Heap       uint64
	Goroutines int
}

func snapshotMem() memSnap {
	runtime.GC()
	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return memSnap{Heap: ms.HeapAlloc, Goroutines: runtime.NumGoroutine()}
}

func unique(in []string) []string {
	if len(in) < 2 {
		return in
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
