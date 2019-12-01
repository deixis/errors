package cache

import "context"

// nopCache is a cache which does not do anything.
type nopCache struct{}

// NopCache returns a new cache instance which does not cache anything
func NopCache() Cache {
	return &nopCache{}
}

func (c *nopCache) NewGroup(
	name string, cacheBytes int64, loader LoadFunc,
) Group {
	return &group{load: loader}
}

type group struct {
	load LoadFunc
}

func (g *group) Get(ctx context.Context, key string) ([]byte, error) {
	return g.load(ctx, key)
}
