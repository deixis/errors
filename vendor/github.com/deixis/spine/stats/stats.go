package stats

import (
	"context"
	"time"

	"github.com/deixis/spine/contextutil"
	"github.com/deixis/spine/log"
)

// Stats is an interface for app statistics
type Stats interface {
	Start()
	Stop()

	// Count is a simple counter
	Count(key string, n interface{}, meta ...map[string]string)
	// Inc increments the given counter by 1
	Inc(key string, meta ...map[string]string)
	// Dec decrements the given counter by 1
	Dec(key string, meta ...map[string]string)
	// Gauge measures the amount, level, or contents of something
	// The given value replaces the current one
	// e.g. in-flight requests, uptime, ...
	Gauge(key string, n interface{}, meta ...map[string]string)
	// Timing measures how long it takes to accomplish something
	// e.g. algorithm, request, ...
	Timing(key string, t time.Duration, meta ...map[string]string)
	// Histogram measures the distribution of values over the time
	Histogram(key string, n interface{}, tags ...map[string]string)

	// With returns a child Stats, and add meta to that Stats
	With(meta map[string]string) Stats
	// Log attaches a logger to a Stats instance
	Log(l log.Logger) Stats
}

// Count calls `Count` on the context `Stats`
func Count(ctx context.Context, key string, n interface{}, meta ...map[string]string) {
	FromContext(ctx).Count(key, n, meta...)
}

// Inc calls `Inc` on the context `Stats`
func Inc(ctx context.Context, key string, meta ...map[string]string) {
	FromContext(ctx).Inc(key, meta...)
}

// Dec calls `Dec` on the context `Stats`
func Dec(ctx context.Context, key string, meta ...map[string]string) {
	FromContext(ctx).Dec(key, meta...)
}

// Gauge calls `Gauge` on the context `Stats`
func Gauge(ctx context.Context, key string, n interface{}, meta ...map[string]string) {
	FromContext(ctx).Gauge(key, n, meta...)
}

// Timing calls `Timing` on the context `Stats`
func Timing(ctx context.Context, key string, t time.Duration, meta ...map[string]string) {
	FromContext(ctx).Timing(key, t, meta...)
}

// Histogram calls `Histogram` on the context `Stats`
func Histogram(ctx context.Context, key string, n interface{}, tags ...map[string]string) {
	FromContext(ctx).Histogram(key, n, tags...)
}

type contextKey struct{}

var activeContextKey = contextKey{}

// FromContext returns a `Stats` instance associated with `ctx`, or
// `NopStats` if no `Stats` instance could be found.
func FromContext(ctx contextutil.ValueContext) Stats {
	val := ctx.Value(activeContextKey)
	if o, ok := val.(Stats); ok {
		return o
	}
	return NopStats()
}

// WithContext returns a copy of parent in which `Stats` is stored
func WithContext(ctx context.Context, s Stats) context.Context {
	return context.WithValue(ctx, activeContextKey, s)
}
