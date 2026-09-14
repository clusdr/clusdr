package app

import "context"

// Fn is one unit init or close step.
// It receives the shared *App so it can read from earlier units
// and write its own output for later units.
type Fn func(ctx context.Context, a *App) error

// Unit is one named stage of process startup.
// InitFn runs in slice order; CloseFn is collected and run in reverse.
type Unit struct {
	Name    string
	InitFn  Fn
	CloseFn Fn
}

type namedClose struct {
	name string
	fn   Fn
}
