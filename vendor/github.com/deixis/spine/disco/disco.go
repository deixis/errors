package disco

import (
	"context"
	"net"
	"strconv"
)

// An Agent interacts with a service discovery cluster to manage all services
// offered by the local node. It also allows to query the service discovery
// cluster to fetch services offered by other nodes.
type Agent interface {
	// Register adds a new service to the catalogue
	Register(ctx context.Context, s *Registration) (string, error)
	// Deregister removes a service from the catalogue
	// If the service does not exist, no action is taken.
	Deregister(ctx context.Context, id string) error
	// Services returns all registered service instances
	Services(ctx context.Context, tags ...string) (map[string]Service, error)
	// Service returns all instances of a service
	Service(ctx context.Context, name string, tags ...string) (Service, error)
	// Leave is used to have the agent de-register all services from the catalogue
	// that belong to this node, and gracefully leave
	Leave(ctx context.Context)
}

// A Service is a set of functionalities offered by one or multiple nodes on
// the network. A node that offers the service is called an instance.
// A node can offer multiple services, so there will be multiple instances on
// the same node.
type Service interface {
	// Name returns the unique name of a service
	Name() string
	// Watch listens to service updates
	Watch() Watcher
	// Instances returns all available instances of the service
	Instances() []*Instance
}

// An Instance is an instance of a remotely-accessible service on the network
type Instance struct {
	// Local tells whether it is a local or remote instance
	Local bool
	// ID is the unique instance identifier
	ID string
	// Name of the service
	Name string
	// Host is the IP address or DNS name
	Host string
	// Port defines the port on which the service runs
	Port uint16
	// Tags of that instance
	Tags []string
}

// Addr returns the instance host+port
func (i *Instance) Addr() string {
	return net.JoinHostPort(i.Host, strconv.FormatUint(uint64(i.Port), 10))
}

// Registration allows to register a service
type Registration struct {
	// ID is the unique identifier for the service (optional)
	ID string
	// Name is the service name
	Name string
	Addr string
	Port uint16
	Tags []string
}

type contextKey struct{}

var activeAgentContextKey = contextKey{}

// AgentFromContext returns an `Agent` instance associated with `ctx`, or
// the local `Agent` if no instance could be found.
func AgentFromContext(ctx context.Context) Agent {
	val := ctx.Value(activeAgentContextKey)
	if o, ok := val.(Agent); ok {
		return o
	}
	return activeLocalAgent
}

// AgentWithContext returns a copy of parent in which the `Agent` is stored
func AgentWithContext(ctx context.Context, agent Agent) context.Context {
	return context.WithValue(ctx, activeAgentContextKey, agent)
}
