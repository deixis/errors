package tracing

import (
	"context"

	"github.com/deixis/spine/contextutil"
	"github.com/deixis/spine/log"
	"github.com/deixis/spine/stats"
	opentracing "github.com/opentracing/opentracing-go"
)

// Tracer follows the opentracing standard https://opentracing.io
type Tracer opentracing.Tracer

// TracerOption configures how we set up a job
type TracerOption func(*TracerOptions)

// TracerOptions configure a Tracer.
type TracerOptions struct {
	Logger log.Logger
	Stats  stats.Stats
}

// WithLogger injects a `Logger` to `Tracer`
func WithLogger(l log.Logger) TracerOption {
	return func(o *TracerOptions) {
		o.Logger = l
	}
}

// WithStats injects a `Stats` to `Tracer`
func WithStats(s stats.Stats) TracerOption {
	return func(o *TracerOptions) {
		o.Stats = s
	}
}

// StartSpanFromContext starts and returns a Span with `operationName`, using
// any Span found within `ctx` as a ChildOfRef. If no such parent could be
// found, StartSpanFromContext creates a root (parentless) Span.
//
// The second return value is a context.Context object built around the
// returned Span.
//
// Example usage:
//
//    SomeFunction(ctx context.Context, ...) {
//        sp, ctx := tracing.StartSpanFromContext(ctx, "SomeFunction")
//        defer sp.Finish()
//        ...
//    }
func StartSpanFromContext(
	ctx context.Context, operationName string, opts ...opentracing.StartSpanOption,
) (opentracing.Span, context.Context) {
	if parent := SpanFromContext(ctx); parent != nil {
		opts = append(opts, opentracing.ChildOf(parent.Context()))
	}
	span := FromContext(ctx).StartSpan(operationName, opts...)
	return span, opentracing.ContextWithSpan(ctx, span)
}

// StartSpan creates, starts, and returns a new Span with the given
// `operationName` and incorporate the given StartSpanOption `opts`.
//
// StartSpan calls `StartSpan` on the context `Tracer`
func StartSpan(
	ctx context.Context, operationName string, opts ...opentracing.StartSpanOption,
) (span opentracing.Span) {
	return FromContext(ctx).StartSpan(operationName, opts...)
}

// Inject takes the `sm` SpanContext instance and injects it for
// propagation within `carrier`. The actual type of `carrier` depends on
// the value of `format`.
//
// Inject calls `Inject` on the context `Tracer`
func Inject(
	ctx context.Context, sm opentracing.SpanContext, format interface{}, carrier interface{},
) error {
	return FromContext(ctx).Inject(sm, format, carrier)
}

// Extract returns a SpanContext instance given `format` and `carrier`.
//
// Extract calls `Extract` on the context `Tracer`
func Extract(
	ctx context.Context, format interface{}, carrier interface{},
) (opentracing.SpanContext, error) {
	return FromContext(ctx).Extract(format, carrier)
}

// ContextWithSpan returns a new `context.Context` that holds a reference to
// `span`'s SpanContext.
func ContextWithSpan(ctx context.Context, span opentracing.Span) context.Context {
	return opentracing.ContextWithSpan(ctx, span)
}

// SpanFromContext returns the `Span` previously associated with `ctx`, or
// `nil` if no such `Span` could be found.
//
// NOTE: context.Context != SpanContext: the former is Go's intra-process
// context propagation mechanism, and the latter houses OpenTracing's per-Span
// identity and baggage information.
func SpanFromContext(ctx context.Context) opentracing.Span {
	return opentracing.SpanFromContext(ctx)
}

type contextKey struct{}

var activeContextKey = contextKey{}

// FromContext returns a `Tracer` instance associated with `ctx`, or
// the `opentracing.GlobalTracer` if no `Tracer` instance could be found.
func FromContext(ctx contextutil.ValueContext) Tracer {
	val := ctx.Value(activeContextKey)
	if o, ok := val.(Tracer); ok {
		return o
	}
	return opentracing.GlobalTracer()
}

// WithContext returns a copy of parent in which the `Tracer` is stored
func WithContext(ctx context.Context, t Tracer) context.Context {
	return context.WithValue(ctx, activeContextKey, t)
}
