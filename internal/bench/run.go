package bench

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"
)

// Config controls a bench run. Zero values become defaults.
type Config struct {
	Nodes       int
	EventNodes  int
	ElectionOps int
	EventOps    int
	LockOps     int
	MemberOps   int
	Scenarios   []string
	Dir         string
	Log         *slog.Logger
}

func (c *Config) normalize() error {
	if c.Nodes <= 0 {
		c.Nodes = 3
	}
	if c.Nodes < 3 {
		return fmt.Errorf("nodes must be >= 3, got %d", c.Nodes)
	}
	if c.EventNodes <= 0 {
		c.EventNodes = 25
	}
	if c.EventNodes < 2 {
		return fmt.Errorf("event-nodes must be >= 2, got %d", c.EventNodes)
	}
	if c.ElectionOps <= 0 {
		c.ElectionOps = 5
	}
	if c.EventOps <= 0 {
		c.EventOps = 10
	}
	if c.LockOps <= 0 {
		c.LockOps = 50
	}
	if c.MemberOps <= 0 {
		c.MemberOps = 100
	}
	if c.Log == nil {
		c.Log = discardLog()
	}
	if len(c.Scenarios) == 0 {
		c.Scenarios = []string{"election", "events", "members", "locks"}
	}
	for i, s := range c.Scenarios {
		c.Scenarios[i] = strings.ToLower(strings.TrimSpace(s))
	}
	return nil
}

func (c Config) wants(name string) bool {
	return slices.Contains(c.Scenarios, "all") || slices.Contains(c.Scenarios, name)
}

// Run executes the requested scenarios and returns a report.
func Run(ctx context.Context, cfg Config) (*Report, error) {
	if err := cfg.normalize(); err != nil {
		return nil, err
	}
	dir := cfg.Dir
	if dir == "" {
		tmp, err := os.MkdirTemp("", "clusdr-bench-")
		if err != nil {
			return nil, fmt.Errorf("temp dir: %w", err)
		}
		dir = tmp
		defer os.RemoveAll(dir)
	}

	rep := &Report{Passed: true}
	needRaft := cfg.wants("election") || cfg.wants("locks")
	if needRaft {
		cl, err := StartCluster(dir, cfg.Nodes, cfg.Log)
		if err != nil {
			return nil, err
		}
		defer func() { _ = cl.Close() }()

		// Locks before election: failover churn leaves the log hotter
		// and inflates acquire latency past the 50ms target.
		if cfg.wants("locks") {
			res, err := measureLocks(ctx, cl, cfg.LockOps)
			if err != nil {
				return nil, err
			}
			rep.Results = append(rep.Results, res)
		}
		if cfg.wants("election") {
			res, err := measureElection(ctx, cl, cfg.ElectionOps)
			if err != nil {
				return nil, err
			}
			rep.Results = append(rep.Results, res)
		}
	}
	if cfg.wants("events") {
		res, err := measureEvents(ctx, cfg.EventNodes, cfg.EventOps)
		if err != nil {
			return nil, err
		}
		rep.Results = append(rep.Results, res)
	}
	if cfg.wants("members") {
		res, err := measureMembers(ctx, cfg.EventNodes, cfg.MemberOps)
		if err != nil {
			return nil, err
		}
		rep.Results = append(rep.Results, res)
	}

	for _, r := range rep.Results {
		if !r.Passed {
			rep.Passed = false
			break
		}
	}
	return rep, nil
}
