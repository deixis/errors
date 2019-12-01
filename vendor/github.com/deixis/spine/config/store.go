package config

import (
	"fmt"
	"net/url"
	"sort"
	"sync"

	"github.com/deixis/spine/config/adapter"
	"github.com/deixis/spine/config/adapter/consul"
	"github.com/deixis/spine/config/adapter/file"
)

var (
	sfMu     sync.RWMutex
	adapters = make(map[string]store.Adapter)
)

func init() {
	// Register default adapters
	Register(consul.Name, consul.New)
	Register(file.Name, file.New)
}

// Adapters returns the list of registered adapters
func Adapters() []string {
	sfMu.RLock()
	defer sfMu.RUnlock()

	var l []string
	for a := range adapters {
		l = append(l, a)
	}

	sort.Strings(l)

	return l
}

// Register makes a store adapter available by the provided name.
// If an adapter is registered twice or if an adapter is nil, it will panic.
func Register(name string, adapter store.Adapter) {
	sfMu.Lock()
	defer sfMu.Unlock()

	if adapter == nil {
		panic("config: Registered adapter is nil")
	}
	if _, dup := adapters[name]; dup {
		panic("config: Duplicated adapter")
	}

	adapters[name] = adapter
}

// NewStore returns a loaded config store defined by the ConfigStorage env
func NewStore(configStoreURI string) (store.Store, error) {
	sfMu.RLock()
	defer sfMu.RUnlock()

	uri, err := url.Parse(configStoreURI)
	if err != nil {
		return nil, err
	}

	if f, ok := adapters[uri.Scheme]; ok {
		return f(uri)
	}

	return nil, fmt.Errorf("store adapter not found <%s>", uri.Scheme)
}
