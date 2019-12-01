package context

import (
	"context"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/deixis/spine/tracing"
)

// WithTracer returns a copy of parent with a contextualised `log.Tracer`
func WithTracer(ctx context.Context, tr tracing.Tracer) context.Context {
	return tracing.WithContext(ctx, &tracer{
		Leg:    TransitFromContext(ctx),
		Tracer: tr,
	})
}

// tracer wraps a `tracing.Tracer` to contextualise tracing
type tracer struct {
	Leg    Leg
	Tracer tracing.Tracer
}

func (t *tracer) StartSpan(operationName string, opts ...opentracing.StartSpanOption) opentracing.Span {
	span := t.Tracer.StartSpan(operationName, opts...)
	span.SetTag("id", t.Leg.ShortID())
	span.SetTag("uuid", t.Leg.UUID())
	return span
}
func (t *tracer) Inject(sm opentracing.SpanContext, format interface{}, carrier interface{}) error {
	return t.Tracer.Inject(sm, format, carrier)
}
func (t *tracer) Extract(format interface{}, carrier interface{}) (opentracing.SpanContext, error) {
	return t.Tracer.Extract(format, carrier)
}
