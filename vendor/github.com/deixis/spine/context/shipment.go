package context

import (
	"context"

	"github.com/deixis/spine/contextutil"
)

// Shipment is a just key:value pair that crosses process boundaries
type shipment struct {
	next *shipment
	key  string
	val  interface{}
}

// WithShipment returns a copy of parent in which the value associated with key
// is val.
//
// Use context Values only for request-scoped data that transits processes and
// APIs, not for passing optional parameters to functions.
//
// The provided key must be comparable and should not be of type
// string or any other built-in type to avoid collisions between
// packages using context. Users of WithValue should define their own
// types for keys. To avoid allocating when assigning to an
// interface{}, context keys often have concrete type
// struct{}. Alternatively, exported context key variables' static
// type should be a pointer or interface.
func WithShipment(parent context.Context, key string, val interface{}) context.Context {
	next := shipmentFromContext(parent)
	return shipmentWithContext(parent, &shipment{next, key, val})
}

// Shipment returns the shipment associated with this context for key, or nil
// if no value is associated with key. Successive calls to Value with
// the same key returns the same result.
func Shipment(ctx contextutil.ValueContext, key string) interface{} {
	for sh := shipmentFromContext(ctx); sh != nil; sh = sh.next {
		if sh.key == key {
			return sh.val
		}
	}
	return nil
}

// ShipmentRange calls f sequentially for each shipment in the context stack.
// If f returns false, range stops the iteration.
func ShipmentRange(ctx contextutil.ValueContext, f func(key string, value interface{}) bool) {
	for sh := shipmentFromContext(ctx); sh != nil; sh = sh.next {
		if !f(sh.key, sh.val) {
			return
		}
	}
}

type shipmentKey struct{}

var activeShipmentKey = shipmentKey{}

// shipmentFromContext extracts `Shipment` from context and returns `nil` when
// no instance of `Shipment` can be found
func shipmentFromContext(ctx contextutil.ValueContext) *shipment {
	val := ctx.Value(activeShipmentKey)
	if o, ok := val.(*shipment); ok {
		return o
	}
	return nil
}

// shipmentWithContext injects `Shipment` to context
func shipmentWithContext(ctx context.Context, sh *shipment) context.Context {
	return context.WithValue(ctx, activeShipmentKey, sh)
}
