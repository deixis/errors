// Package cache is a caching and cache-filling library
package cache

import (
	"context"

	"github.com/deixis/spine/contextutil"
	"github.com/deixis/spine/disco"
)

type Cache interface {
	// NewGroup creates a LRU caching namespace with a size limit and a load
	// function to be called when the value is mising
	NewGroup(name string, cacheBytes int64, loader LoadFunc) Group
}

// A Group is a cache namespace
type Group interface {
	Get(ctx context.Context, key string) ([]byte, error)
}

// A LoadFunc loads data for a key.
type LoadFunc func(context context.Context, key string) ([]byte, error)

// Dependencies is an interface to "inject" required services
type Dependencies interface {
	Disco() disco.Agent
}

// NewGroup calls `NewGroup` on the context `Cache`
func NewGroup(ctx context.Context, name string, cacheBytes int64, loader LoadFunc) Group {
	return FromContext(ctx).NewGroup(name, cacheBytes, loader)
}

type contextKey struct{}

var activeContextKey = contextKey{}

// FromContext returns a `Cache` instance associated with `ctx`, or
// a `NopCache` if no `Cache` instance could be found.
func FromContext(ctx contextutil.ValueContext) Cache {
	val := ctx.Value(activeContextKey)
	if o, ok := val.(Cache); ok {
		return o
	}
	return &nopCache{}
}

// WithContext returns a copy of parent in which the `Cache` is stored
func WithContext(ctx context.Context, c Cache) context.Context {
	return context.WithValue(ctx, activeContextKey, c)
}
