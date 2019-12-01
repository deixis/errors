package disco

import (
	"context"
	"errors"
	"sync"

	"github.com/google/uuid"
	"github.com/deixis/spine/log"
)

var activeLocalAgent = NewLocalAgent()

// agent is a local-only service discovery agent
// This agent is used when service discovery is disabled
type agent struct {
	mu sync.RWMutex

	Registry map[string]*Instance
	// subs contains all event subscriptions
	Subs map[chan *Event]struct{}
}

// NewLocalAgent returns a new local-only service discovery agent. This agent
// is used when service discovery is disabled.
func NewLocalAgent() Agent {
	return &agent{
		Registry: map[string]*Instance{},
		Subs:     map[chan *Event]struct{}{},
	}
}

func (a *agent) Register(ctx context.Context, r *Registration) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	id := r.ID
	if id == "" {
		id = uuid.New().String()
	}
	if _, ok := a.Registry[id]; ok {
		return "", errors.New("service already registered")
	}
	instance := &Instance{
		Local: true,
		ID:    id,
		Name:  r.Name,
		Host:  r.Addr,
		Port:  r.Port,
		Tags:  r.Tags,
	}
	a.Registry[id] = instance

	// Notifiy subscribers
	for sub := range a.Subs {
		sub <- &Event{
			Op:       Add,
			Instance: instance,
		}
	}

	return id, nil
}

func (a *agent) Deregister(ctx context.Context, id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.deregister(ctx, id)
}

func (a *agent) Services(
	ctx context.Context, tags ...string,
) (map[string]Service, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.services(ctx, tags...)
}

func (a *agent) Service(
	ctx context.Context, name string, tags ...string,
) (Service, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	services, err := a.services(ctx, tags...)
	if err != nil {
		return nil, err
	}
	s, ok := services[name]
	if !ok {
		return nil, errors.New("service does not exist")
	}
	return s, nil
}

func (a *agent) Leave(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for id := range a.Registry {
		err := a.deregister(ctx, id)
		if err != nil {
			log.FromContext(ctx).Warning(
				"leave.failure",
				"Could not de-register service",
				log.String("service_id", id),
			)
		}
	}
	a.Registry = map[string]*Instance{}
	a.Subs = map[chan *Event]struct{}{}
}

func (a *agent) services(
	ctx context.Context, tags ...string,
) (map[string]Service, error) {
	services := map[string]Service{}
	for _, instance := range a.Registry {
		services[instance.Name] = &service{
			name:      instance.Name,
			instances: []*Instance{instance},
			watch: func() Watcher {
				sub := make(chan *Event, 1)
				unsub := func() {
					delete(a.Subs, sub)
				}
				a.Subs[sub] = struct{}{}
				return &watcher{
					sub:   sub,
					unsub: unsub,
				}
			},
		}
	}
	return services, nil
}

func (a *agent) deregister(ctx context.Context, id string) error {
	instance, ok := a.Registry[id]
	if !ok {
		return nil
	}
	delete(a.Registry, id)

	// Notifiy subscribers
	for sub := range a.Subs {
		sub <- &Event{
			Op:       Delete,
			Instance: instance,
		}
	}

	return nil
}

// service implements Service
type service struct {
	name      string
	instances []*Instance
	watch     func() Watcher
}

func (s *service) Name() string {
	return s.name
}

func (s *service) Watch() Watcher {
	return s.watch()
}

func (s *service) Instances() []*Instance {
	return s.instances
}

type watcher struct {
	sub   chan *Event
	unsub func()
}

func (w *watcher) Next() ([]*Event, error) {
	e, ok := <-w.sub
	if !ok {
		return nil, ErrWatcherClosed
	}
	return []*Event{e}, nil
}

func (w *watcher) Close() error {
	w.unsub()
	close(w.sub)
	return nil
}
