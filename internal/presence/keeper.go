package presence

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Holder grants and renews leases. Production uses Raft on the leader and
// forwards to the leader from followers; tests inject a fake.
type Holder interface {
	Grant(ctx context.Context, name, owner string, ttl time.Duration) (token uint64, err error)
	Renew(ctx context.Context, name, owner string, token uint64, ttl time.Duration) error
}

// Config controls the local presence lease.
type Config struct {
	NodeID         string
	TTL            time.Duration
	RenewInterval  time.Duration
	RequestTimeout time.Duration
}

// Keeper holds this node's presence lease alive until Stop.
type Keeper struct {
	cfg    Config
	holder Holder
	log    *slog.Logger

	mu    sync.Mutex
	token uint64

	cancel context.CancelFunc
	done   chan struct{}
}

// NewKeeper creates a keeper. Call Start to grant and renew.
func NewKeeper(cfg Config, holder Holder, log *slog.Logger) *Keeper {
	if cfg.TTL <= 0 {
		cfg.TTL = 3 * time.Second
	}
	if cfg.RenewInterval <= 0 {
		cfg.RenewInterval = cfg.TTL / 3
		if cfg.RenewInterval <= 0 {
			cfg.RenewInterval = cfg.TTL
		}
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = 5 * time.Second
	}
	return &Keeper{
		cfg:    cfg,
		holder: holder,
		log:    log,
		done:   make(chan struct{}),
	}
}

// Start launches the grant/renew loop. Returns immediately.
func (k *Keeper) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	k.cancel = cancel
	go k.run(ctx)
}

// Stop cancels the renew loop and waits for it to exit.
func (k *Keeper) Stop() {
	if k.cancel != nil {
		k.cancel()
	}
	<-k.done
}

func (k *Keeper) run(ctx context.Context) {
	defer close(k.done)
	k.ensure(ctx)
	t := time.NewTicker(k.cfg.RenewInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			k.ensure(ctx)
		}
	}
}

func (k *Keeper) ensure(ctx context.Context) {
	if k.holder == nil || k.cfg.NodeID == "" {
		return
	}
	name := LeaseName(k.cfg.NodeID)
	opCtx, cancel := context.WithTimeout(ctx, k.cfg.RequestTimeout)
	defer cancel()

	k.mu.Lock()
	tok := k.token
	k.mu.Unlock()

	if tok != 0 {
		if err := k.holder.Renew(opCtx, name, k.cfg.NodeID, tok, k.cfg.TTL); err == nil {
			return
		} else if k.log != nil {
			k.log.Debug("presence renew failed; re-granting", "err", err)
		}
	}

	newTok, err := k.holder.Grant(opCtx, name, k.cfg.NodeID, k.cfg.TTL)
	if err != nil {
		if k.log != nil {
			k.log.Warn("presence grant failed", "err", err)
		}
		return
	}
	k.mu.Lock()
	k.token = newTok
	k.mu.Unlock()
	if k.log != nil {
		k.log.Debug("presence lease held", "name", name, "token", newTok)
	}
}

// Token returns the current fencing token, or 0 if not yet granted.
func (k *Keeper) Token() uint64 {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.token
}

// ErrHeld is returned when Grant finds the presence lease owned by someone else.
var ErrHeld = fmt.Errorf("presence lease held by another owner")
