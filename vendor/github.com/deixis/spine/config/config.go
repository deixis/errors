package config

import (
	"context"
	"time"
)

// TODO: Move these structs to lego package once ctx.App is gone

// Config defines the app config
type Config struct {
	Node    string  `toml:"node"`
	Version string  `toml:"version"`
	Request Request `toml:"request"`
}

// Request defines the request default configuration
type Request struct {
	TimeoutMS    time.Duration `toml:"timeout_ms"`
	AllowContext bool          `toml:"allow_context"`
	Panic        bool          `toml:"panic"`
}

// Timeout returns the TimeoutMS field in time.Duration
func (r *Request) Timeout() time.Duration {
	return time.Millisecond * r.TimeoutMS
}

type contextKey struct{}

var activeTreeContextKey = contextKey{}

// TreeFromContext returns a config `Tree` instance associated with `ctx`, or
// a `NopTree` if no `Tree` instance could be found.
func TreeFromContext(ctx context.Context) Tree {
	val := ctx.Value(activeTreeContextKey)
	if o, ok := val.(Tree); ok {
		return o
	}
	return NopTree()
}

// TreeWithContext returns a copy of parent in which the `Tree` is stored
func TreeWithContext(ctx context.Context, t Tree) context.Context {
	return context.WithValue(ctx, activeTreeContextKey, t)
}
