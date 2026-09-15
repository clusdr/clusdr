// Package app is the composition root for the Clusdr daemon.
//
// cmd/clusdr lists Units (InitFn / CloseFn). InitApp runs them in order,
// collecting CloseFns for reverse shutdown. Wait blocks until a server
// goroutine exits or Stop is called. WatchShutdown triggers Stop on
// SIGINT/SIGTERM. No package-level mutable state; all collaborators
// live on *App and are injected via Options for tests.
package app

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/durguto/clusdr/internal/config"
	"github.com/durguto/clusdr/internal/consensus"
	"github.com/durguto/clusdr/internal/eventbus"
	"github.com/durguto/clusdr/internal/grpcserver"
	"github.com/durguto/clusdr/internal/heartbeat"
	"github.com/durguto/clusdr/internal/leases"
	"github.com/durguto/clusdr/internal/locks"
	"github.com/durguto/clusdr/internal/membership"
	"github.com/durguto/clusdr/internal/store"
)

const fallbackShutdown = 15 * time.Second

// Options controls how InitApp binds resources. Tests inject Getenv and
// LogOutput; production code leaves them nil for os.Getenv / os.Stderr.
type Options struct {
	// ConfigPath is the YAML config file path, typically from --config flag.
	ConfigPath string
	// Getenv overrides os.Getenv for config loading. Leave nil in production.
	Getenv func(string) string
	// LogOutput overrides os.Stderr for structured log output. Leave nil in production.
	LogOutput io.Writer
	// GRPCListener overrides the TCP listener for the gRPC Runtime API.
	GRPCListener net.Listener
	// ControlListener overrides the Unix socket listener for the Control API.
	ControlListener net.Listener
}

// App is the in-process bag of collaborators. Units populate fields during
// InitFn and may read previously-set fields from earlier units.
type App struct {
	Opt        Options
	Cfg        config.Config
	Log        *slog.Logger
	Store      *store.Store
	Membership *membership.Engine
	Heartbeat  *heartbeat.Monitor
	Consensus  *consensus.Node
	Bus        *eventbus.Bus
	Locks      *locks.Table
	Leases     *leases.Table
	GRPC       *grpcserver.Server
	presence   *presenceCtl

	closeFns   []namedClose
	serveErr   chan error
	serveCount int
	serveTaken bool

	stopCh    chan struct{}
	stopOnce  sync.Once
	closeOnce sync.Once
	waitOnce  sync.Once
	waitErr   error
}

// InitApp runs each unit's InitFn in order. On the first failure it closes
// already-started units in reverse and returns the error.
func InitApp(ctx context.Context, units []Unit, opt Options) (*App, error) {
	if opt.Getenv == nil {
		opt.Getenv = os.Getenv
	}
	if opt.LogOutput == nil {
		opt.LogOutput = os.Stderr
	}

	a := &App{
		Opt:      opt,
		serveErr: make(chan error, 8),
		stopCh:   make(chan struct{}),
	}

	for _, u := range units {
		if u.InitFn == nil {
			closeWithTimeout(a)
			return nil, fmt.Errorf("unit %q has nil InitFn", u.Name)
		}
		start := time.Now()
		if err := u.InitFn(ctx, a); err != nil {
			if a.Log != nil {
				a.Log.Error("unit init failed",
					"unit", u.Name,
					"err", err,
					"elapsed_ms", time.Since(start).Milliseconds())
			}
			closeWithTimeout(a)
			return nil, fmt.Errorf("%s: %w", u.Name, err)
		}
		if a.Log != nil {
			a.Log.Info("init", "unit", u.Name, "elapsed_ms", time.Since(start).Milliseconds())
		}
		if u.CloseFn != nil {
			a.closeFns = append(a.closeFns, namedClose{name: u.Name, fn: u.CloseFn})
		}
	}
	return a, nil
}

// WatchShutdown blocks on SIGINT/SIGTERM then calls Stop.
// Call in a goroutine before Wait, as in cmd/clusdr.
// SIGHUP is intentionally not handled here so that nohup(1) can properly
// ignore it when the daemon is launched without a controlling terminal.
func (a *App) WatchShutdown() {
	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	if a.Log != nil {
		a.Log.Info("shutdown signal")
	}
	a.Stop()
}

// Stop unblocks Wait. Safe to call more than once.
func (a *App) Stop() {
	if a == nil {
		return
	}
	a.stopOnce.Do(func() { close(a.stopCh) })
}

// Wait blocks until Stop is called or a serve goroutine exits, then closes
// all units in reverse. Safe to call once.
func (a *App) Wait() error {
	if a == nil {
		return nil
	}
	a.waitOnce.Do(func() {
		if a.serveCount > 0 {
			select {
			case err := <-a.serveErr:
				a.serveTaken = true
				a.waitErr = err
			case <-a.stopCh:
			}
		} else {
			<-a.stopCh
		}
		closeWithTimeout(a)
		a.waitErr = drainServeErr(a, a.waitErr)
		if a.Log != nil {
			a.Log.Info("stopped")
		}
	})
	if a.waitErr != nil {
		return fmt.Errorf("serve: %w", a.waitErr)
	}
	return nil
}

// StartServe registers a serve goroutine whose lifetime is tracked by Wait.
// fn receives the listener and should block until the server stops.
// Use this in unit InitFns that start TCP or Unix socket servers.
func (a *App) StartServe(fn func(net.Listener) error, ln net.Listener) {
	a.serveCount++
	go func() { a.serveErr <- fn(ln) }()
}

// Close runs CloseFns in reverse. Safe to call more than once.
func (a *App) Close(ctx context.Context) {
	if a == nil {
		return
	}
	a.closeOnce.Do(func() {
		for i := len(a.closeFns) - 1; i >= 0; i-- {
			c := a.closeFns[i]
			if a.Log != nil {
				a.Log.Info("stop", "unit", c.name)
			}
			if err := c.fn(ctx, a); err != nil && a.Log != nil {
				a.Log.Error("unit close error", "unit", c.name, "err", err)
			}
		}
		a.closeFns = nil
	})
}

// closeWithTimeout closes all units within the shutdown grace period.
func closeWithTimeout(a *App) {
	d := fallbackShutdown
	if a != nil && a.Cfg.ShutdownTimeout > 0 {
		d = a.Cfg.ShutdownTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	a.Close(ctx)
}

// drainServeErr collects remaining serve goroutine errors after the first.
func drainServeErr(a *App, already error) error {
	n := a.serveCount
	if a.serveTaken {
		n--
	}
	if n < 0 {
		n = 0
	}
	err := already
	for i := 0; i < n; i++ {
		if se := <-a.serveErr; se != nil && err == nil {
			err = se
		}
	}
	return err
}
