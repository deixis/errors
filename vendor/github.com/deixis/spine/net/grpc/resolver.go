package grpc

import (
	"fmt"

	spine "github.com/deixis/spine/net/naming"
	grpc "google.golang.org/grpc/naming"
)

// WrapResolver wraps a spine resolver with a gRPC resolver
func WrapResolver(r spine.Resolver) grpc.Resolver {
	return &resolver{r: r}
}

type resolver struct {
	r spine.Resolver
}

func (r *resolver) Resolve(target string) (grpc.Watcher, error) {
	w, err := r.r.Resolve(target)
	if err != nil {
		return nil, err
	}
	return &watcher{w: w}, nil
}

type watcher struct {
	w spine.Watcher
}

func (w *watcher) Next() ([]*grpc.Update, error) {
	res, err := w.w.Next()
	if err != nil {
		return nil, err
	}

	var updates []*grpc.Update
	for _, u := range res {
		switch u.Op {
		case spine.Add:
			updates = append(updates, &grpc.Update{
				Op:       grpc.Add,
				Addr:     u.Addr,
				Metadata: u.Metadata,
			})
		case spine.Delete:
			updates = append(updates, &grpc.Update{
				Op:       grpc.Delete,
				Addr:     u.Addr,
				Metadata: u.Metadata,
			})
		default:
			return nil, fmt.Errorf("net/grpc: unsupported naming op %d", u.Op)
		}
	}
	return updates, nil
}

func (w *watcher) Close() {
	w.w.Close() // Igrpcore error
}
