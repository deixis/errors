package naming

import "context"

// URI returns a resolver that uses the target URI scheme to select a real resolver
func URI(ctx context.Context) Resolver {
	return &uriResolver{ctx: ctx}
}

type uriResolver struct {
	ctx context.Context
}

// Resolve creates a Watcher for target.
func (r *uriResolver) Resolve(target string) (Watcher, error) {
	return Resolve(r.ctx, target)
}
