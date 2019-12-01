// Package file reads configuration from a JSON file
//
// e.g.
// CONFIG_URI=file://${PWD}/config/dev.json
package file

import (
	"io"
	"net/url"
	"os"

	"github.com/pkg/errors"
	a "github.com/deixis/spine/config/adapter"
)

// Name contains the adapter registered name
const Name = "file"

// New returns a new file config store
func New(uri *url.URL) (a.Store, error) {
	if _, err := os.Stat(uri.Path); os.IsNotExist(err) {
		return nil, errors.Wrapf(err, "config file does not exist (%s)", uri)
	}

	return &Store{Path: uri.Path}, nil
}

// Store reads config from a file
type Store struct {
	Path string
}

// Load implements Store
func (s *Store) Load() (io.ReadCloser, error) {
	// Load file
	file, err := os.Open(s.Path)
	if err != nil {
		return nil, errors.Wrap(err, "config file cannot be opened")
	}
	return file, nil
}
