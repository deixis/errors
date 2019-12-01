package disco

import (
	"errors"
	"sync"
)

// Watcher watches for the updates on the specified service
type Watcher interface {
	// Next blocks until an event or error happens. It may return one or more
	// events. The first call should get the full set of the results. It should
	// return an error if and only if Watcher cannot recover.
	Next() ([]*Event, error)
	// Close closes the Watcher.
	Close() error
}

// Operation defines the corresponding operations for a service update
type Operation uint8

const (
	// Add indicates a new instance is added.
	Add Operation = iota
	// Update indicates an existing instance is updated.
	Update
	// Delete indicates an exisiting instance is deleted.
	Delete
)

// Event defines an instance event
type Event struct {
	// Op indicates the operation of the update.
	Op Operation
	// Instance is the updated instance.
	Instance *Instance
}

var (
	ErrWatcherClosed = errors.New("watcher closed")
)

// Diff stores a snapshot of instances and generate all events needed to
// go from one snapshot to the other. This simplifies the development of
// adapters that don't support incremental updates.
type Diff struct {
	mu sync.Mutex
	m  map[string]*Instance
}

// Apply returns all events needed to go from the current state to the new state
func (d *Diff) Apply(state []*Instance) (events []*Event) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.m == nil {
		d.m = map[string]*Instance{}
	}
	running := map[string]struct{}{}

	// Check added/updated state
	for _, inst := range state {
		running[inst.ID] = struct{}{}

		if _, ok := d.m[inst.ID]; !ok {
			events = append(events, &Event{
				Op:       Add,
				Instance: inst,
			})
		} else {
			// TODO: Check if it needs an update (optimisation)
			events = append(events, &Event{
				Op:       Update,
				Instance: inst,
			})
		}

		d.m[inst.ID] = inst
	}

	// Check deleted state
	for _, inst := range d.m {
		if _, ok := running[inst.ID]; !ok {
			delete(d.m, inst.ID)
			events = append(events, &Event{
				Op:       Delete,
				Instance: inst,
			})
		}
	}

	return events
}
