package naming

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"sync"

	"github.com/pkg/errors"
)

var (
	mu       sync.RWMutex
	builders = make(map[string]Builder)

	defaultScheme = "passthrough"
)

// Builder is a function that builds a watcher from a URI
type Builder func(ctx context.Context, uri *url.URL) (Watcher, error)

func init() {
	// Register default resolvers
	Register("dns", buildDNS)
	Register("disco", buildDisco)
	Register("passthrough", buildPassthrough)
}

// SetDefaultScheme defines the default scheme to use when the URI does not have it
func SetDefaultScheme(s string) {
	defaultScheme = s
}

// Resolvers returns the list of all registered resolvers
func Resolvers() []string {
	mu.RLock()
	defer mu.RUnlock()

	var l []string
	for r := range builders {
		l = append(l, r)
	}

	sort.Strings(l)

	return l
}

// Register makes a resolver available by the provided name.
// If a resolver is registered twice or if it is nil, it will panic.
func Register(name string, r Builder) {
	mu.Lock()
	defer mu.Unlock()

	if r == nil {
		panic("net/naming: Registered resolver is nil")
	}
	if _, dup := builders[name]; dup {
		panic("net/naming: Duplicated resolver")
	}

	builders[name] = r
}

// Resolve builds a watcher from the given URI
//
// A fully qualified, self contained name used channel construction uses the syntax:
//
//   scheme://authority/endpoint_name
//
func Resolve(ctx context.Context, uri string) (Watcher, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, errors.Wrap(err, "net/naming: invalid uri")
	}

	if u.Scheme == "" {
		u, err = url.Parse(defaultScheme + "://" + uri)
		if err != nil {
			return nil, errors.Wrap(err, "net/naming: invalid uri")
		}
	}

	if f, ok := builders[u.Scheme]; ok {
		return f(ctx, u)
	}
	return nil, fmt.Errorf("net/naming: resolver not found <%s>", u.Scheme)
}
