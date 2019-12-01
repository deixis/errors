package net

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/deixis/spine/log"
)

const (
	// StateDown mode is the default state. The handler is not ready to accept
	// new connections
	StateDown uint32 = iota
	// StateUp mode is when a handler accepts connections
	StateUp
	// StateDrain mode is when a handler stops accepting new connection, but wait
	// for all existing in-flight requests to complete
	StateDrain
)

var (
	// ErrEmptyReg is the error returned when there are no servers registered
	ErrEmptyReg = errors.New("there must be at least one registered server")
	// ErrDraining occurs when there is an attempt to access a draining `Server`
	ErrDraining = errors.New("server is draining")
)

// Server is the interface to implement to be a valid server
type Server interface {
	Serve(ctx context.Context, addr string) error
	Drain()
}

// Reg (registry) holds a list of H
type Reg struct {
	mu sync.Mutex

	log   log.Logger
	l     map[string]Server
	drain bool
}

// NewReg builds a new registry
func NewReg(log log.Logger) *Reg {
	return &Reg{
		log: log,
		l:   map[string]Server{},
	}
}

// Add adds the given server to the list of servers
func (r *Reg) Add(addr string, h Server) {
	r.mu.Lock()
	defer r.mu.Unlock()

	err := r.register(addr, h)
	if err != nil {
		// If we attempt to register on the same address, we can assume it is a
		// config error, therefore we should fail loudly and as fast as possible,
		// hence the panic.
		panic(err)
	}
}

// Serve starts all registered servers
func (r *Reg) Serve(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.l) == 0 {
		return ErrEmptyReg
	}

	r.log.Trace("server.serve.init", "Starting servers...")

	wg := sync.WaitGroup{}
	wg.Add(len(r.l))
	for addr, h := range r.l {
		go func(addr string, s Server) {
			// Deregister itself upon completion
			defer func() {
				r.log.Trace("lego.serve.s.stop", "Server has stopped running",
					log.String("addr", addr),
					log.Type("server", s),
				)
				r.mu.Lock()
				r.deregister(addr)
				r.mu.Unlock()
			}()

			r.log.Trace("lego.serve.s", "Server starts serving",
				log.String("addr", addr),
				log.Type("server", s),
			)
			wg.Done()
			// TODO: Send pre-flight requests to make sure the server is ready
			err := s.Serve(ctx, addr)
			if err != nil {
				r.log.Error("lego.serve.s", "Server error",
					log.String("addr", addr),
					log.Error(err),
				)
			}
		}(addr, h)
	}

	wg.Wait() // Wait to boot all servers
	r.log.Trace("server.serve.ready", "All servers are running")
	return nil
}

// Drain notify all servers to enter in draining mode. It means they are no
// longer accepting new requests, but they can finish all in-flight requests
func (r *Reg) Drain() {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if we are already draining
	if r.drain {
		return
	}

	// Flag registry as draining
	r.drain = true

	// Build WG
	l := len(r.l)
	wg := sync.WaitGroup{}
	wg.Add(l)

	// Drain servers
	r.log.Trace("server.drain.init", "Start draining",
		log.Int("servers", l),
	)
	for _, s := range r.l {
		r.log.Trace("server.drain.s", "Drain server",
			log.Type("server", s),
		)
		go func(s Server) {
			s.Drain()
			wg.Done()
		}(s)
	}

	wg.Wait()

	r.drain = false
	r.log.Trace("server.drain.done", "All servers have been drained")
}

func (r *Reg) register(addr string, s Server) error {
	if _, ok := r.l[addr]; ok {
		return fmt.Errorf(
			"server listening on <%s> has already been registered (%T)",
			addr,
			r.l[addr],
		)
	}

	r.l[addr] = s
	return nil
}

func (r *Reg) deregister(addr string) {
	delete(r.l, addr)
}

// JoinHostPort combines host and port into a network address of the
// form "host:port". If host contains a colon, as found in literal
// IPv6 addresses, then JoinHostPort returns "[host]:port".
//
// See func Dial for a description of the host and port parameters.
func JoinHostPort(host, port string) string {
	return net.JoinHostPort(host, port)
}
