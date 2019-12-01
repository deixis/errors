package context

import "context"

// A Leg is a point-to-point request bounded by a process
type Leg interface {
	UUID() string
	ShortID() string
	// Tick increments the stepper
	Tick() uint
	// Step returns a string representation of the current step
	Step() Step
}

type legKey struct{}

var activeLegKey = legKey{}

// LegFromContext extracts `Leg` from context and returns `nil` when
// no instance of `Leg` can be found
func LegFromContext(ctx context.Context) Leg {
	val := ctx.Value(activeLegKey)
	if o, ok := val.(Leg); ok {
		return o
	}
	val = ctx.Value(activeTransitKey)
	if o, ok := val.(Leg); ok {
		return o
	}
	return nil
}

// LegWithContext injects `Leg` to context
func LegWithContext(ctx context.Context, t Leg) context.Context {
	return context.WithValue(ctx, activeLegKey, t)
}
