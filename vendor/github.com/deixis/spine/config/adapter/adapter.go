package store

import (
	"io"
	"net/url"
)

// Adapter returns a new store initialised with the given config
type Adapter func(uri *url.URL) (Store, error)

// Store is an interface for config store
type Store interface {
	// Load loads the configuration from the store
	Load() (io.ReadCloser, error)
}
