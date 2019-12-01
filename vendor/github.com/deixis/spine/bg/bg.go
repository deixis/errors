package bg

import (
	"context"

	"github.com/deixis/spine/cache"
	"github.com/deixis/spine/config"
	lcontext "github.com/deixis/spine/context"
	"github.com/deixis/spine/disco"
	"github.com/deixis/spine/log"
	"github.com/deixis/spine/schedule"
	"github.com/deixis/spine/stats"
	"github.com/deixis/spine/tracing"
)

func BG(parent context.Context, f func(ctx context.Context)) error {
	tr := lcontext.TransitFromContext(parent)

	return RegFromContext(parent).Dispatch(NewTask(func() {
		// TODO: Reference context to parent (Follows ref)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Attach app context services to request context
		ctx = config.TreeWithContext(ctx, config.TreeFromContext(parent))
		ctx = log.WithContext(ctx, log.FromContext(parent))
		ctx = stats.WithContext(ctx, stats.FromContext(parent))
		ctx = RegWithContext(ctx, RegFromContext(parent))
		ctx = tracing.WithContext(ctx, tracing.FromContext(parent))
		ctx = disco.AgentWithContext(ctx, disco.AgentFromContext(parent))
		ctx = schedule.SchedulerWithContext(ctx, schedule.SchedulerFromContext(parent))
		ctx = cache.WithContext(ctx, cache.FromContext(parent))

		if tr != nil {
			ctx = lcontext.TransitWithContext(ctx, tr)
		} else {
			ctx, tr = lcontext.NewTransitWithContext(ctx)
		}
		lcontext.ShipmentRange(parent, func(k string, v interface{}) bool {
			ctx = lcontext.WithShipment(ctx, k, v)
			return true
		})

		f(ctx)
	}))
}

// Dispatch calls `Dispatch` on the context `Registry`
func Dispatch(ctx context.Context, j Job) error {
	return RegFromContext(ctx).Dispatch(j)
}

type contextKey struct{}

var activeContextKey = contextKey{}

// RegFromContext returns a `Reg` instance associated with `ctx`, or
// a new `Reg` if no existing `Reg` instance could be found.
func RegFromContext(ctx context.Context) *Reg {
	val := ctx.Value(activeContextKey)
	if o, ok := val.(*Reg); ok {
		return o
	}
	return NewReg("unnamed", ctx)
}

// RegWithContext returns a copy of parent in which `Reg` is stored
func RegWithContext(ctx context.Context, r *Reg) context.Context {
	return context.WithValue(ctx, activeContextKey, r)
}
