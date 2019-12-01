package bg

import (
	"context"
	"errors"
	"sync"

	"github.com/deixis/spine/log"
	"github.com/deixis/spine/stats"
)

// ErrDrain is the error returned when a new job attempts to be started during
// and the registry is draining
var ErrDrain = errors.New("registry is draining")

// ErrDup is the error returned when a new job has already been registered
var ErrDup = errors.New("job has already been registered")

// Job is a an interface to implement to be a background job
type Job interface {
	Start()
	Stop()
}

// Reg (registry) holds a list of running jobs
type Reg struct {
	mu sync.Mutex

	drain   bool
	service string
	ctx     context.Context
	log     log.Logger
	stats   stats.Stats
	jobs    map[Job]*status
}

// NewReg builds a new registry
func NewReg(service string, ctx context.Context) *Reg {
	return &Reg{
		service: service,
		ctx:     ctx,
		log:     log.FromContext(ctx),
		stats:   stats.FromContext(ctx),
		jobs:    map[Job]*status{},
	}
}

// Dispatch registers the given job and runs it in background
func (r *Reg) Dispatch(j Job) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Do not accept new jobs when the registry is draining
	if r.drain {
		return ErrDrain
	}

	// Ensure that it has not been already accepted
	if _, ok := r.jobs[j]; ok {
		return ErrDup
	}

	// Add it to registry
	s := r.register(j)

	go func() {
		// Deregister itself upon completion
		defer func() {
			r.mu.Lock()
			r.deregister(j)
			r.mu.Unlock()
		}()

		// Start job
		r.log.Trace("bg.job.start", "Start job",
			log.Type("j", j),
			log.Ptr("addr", j),
		)
		s.started <- struct{}{}
		j.Start()
	}()

	return nil
}

// Drain sends a Stop() signal to all registered jobs and rejects new jobs
func (r *Reg) Drain() {
	// Check if we are already draining
	r.mu.Lock()
	if r.drain {
		r.mu.Unlock()
		return
	}
	r.drain = true

	// Build WG
	wg := &sync.WaitGroup{}
	wg.Add(len(r.jobs))

	// Release lock
	r.mu.Unlock()

	// Start draining jobs
	r.log.Trace("bg.drain.start", "Draining registry",
		log.Int("jobs", len(r.jobs)),
	)
	for j, s := range r.jobs {
		go func(j Job, s *status) {
			defer wg.Done()

			// Wait for job to be started
			<-s.started

			// Stop job
			r.log.Trace("bg.job.stop", "Stop job",
				log.Type("j", j),
				log.Ptr("addr", j),
			)
			j.Stop()
		}(j, s)
	}

	wg.Wait()
	r.log.Trace("bg.drain.done", "Registry drained")
}

func (r *Reg) register(j Job) *status {
	s := &status{
		started: make(chan struct{}, 1),
	}
	r.jobs[j] = s

	r.addStats()

	return s
}

func (r *Reg) deregister(j Job) {
	delete(r.jobs, j)
	r.addStats()
}

func (r *Reg) addStats() {
	r.stats.Gauge("bg.jobs", len(r.jobs), map[string]string{
		"service": r.service,
	})
}

type status struct {
	started chan struct{}
}
