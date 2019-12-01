package naming

import (
	"context"
	"net/url"
)

// Passthrough returns a resolver that defer the name resolution to the client
func Passthrough(ctx context.Context) Resolver {
	return &passThroughResolver{}
}

func buildPassthrough(ctx context.Context, uri *url.URL) (Watcher, error) {
	return Passthrough(ctx).Resolve(uri.String())
}

type passThroughResolver struct{}

func (r *passThroughResolver) Resolve(target string) (Watcher, error) {
	w := &passThroughWatcher{
		updateChan: make(chan *Update, 1),
	}
	w.updateChan <- &Update{Op: Add, Addr: target}
	return w, nil
}

type passThroughWatcher struct {
	updateChan chan *Update
}

// Next returns the target once and will block all following calls.
func (i *passThroughWatcher) Next() ([]*Update, error) {
	u, ok := <-i.updateChan
	if !ok {
		return nil, ErrWatcherClosed
	}
	return []*Update{u}, nil
}

func (i *passThroughWatcher) Close() error {
	close(i.updateChan)
	return nil
}
