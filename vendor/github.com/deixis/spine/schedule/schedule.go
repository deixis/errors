package schedule

import (
	"context"
	"time"

	"github.com/deixis/spine/contextutil"
	"github.com/google/uuid"
)

const (
	// DefaultRetryLimit is the default limit for retrying a failed job, measured
	// from when the job was first run.
	DefaultRetryLimit = 5
	// DefaultMinBackOff is the default minimum duration to wait before retrying
	// a job after it fails.
	DefaultMinBackOff = time.Second
	// DefaultMaxBackOff is the default maximum duration to wait before retrying
	// a job after it fails.
	DefaultMaxBackOff = time.Hour
)

// Scheduler is a time-based job scheduler. It executes jobs at fixed times or intervals
type Scheduler interface {
	// Start does the initialisation work to bootstrap a Scheduler. For example,
	// this function may start the event loop and watch the updates.
	Start(ctx context.Context) error

	// HandleFunc registers fn for the given target. For each new job, fn will be called.
	// There can be only one handler per target.
	HandleFunc(target string, fn Fn) (deregister func(), err error)

	// At registers a job that will be executed at time t
	At(ctx context.Context, t time.Time, target string, data []byte, o ...JobOption) (string, error)
	// In registers a job that will be executed in duration d from now
	In(ctx context.Context, d time.Duration, target string, data []byte, o ...JobOption) (string, error)

	// TODO: Interval API
	// It should create a Schedule struct that will generate Jobs.
	// When a job is being executed, it should generates the next occurence based on the interval rule.
	//
	// Interval schedules one to multiple jobs based on the given interval rules.
	// Interval(ctx context.Context, r RRule, target string, data []byte, o ...JobOption) error

	// Drain finishes the ongoing job batch and stops after that.
	// When a scheduler is drained, it should still be possible to register new
	// jobs, but none of them will be executed until the scheduler restart.
	// Once a scheduler is drained, it can safely be closed.
	Drain()
	// Close immediately closes the scheduler and any ongoing job will be left unfinished.
	// For a graceful shutdown, use Drain first and then Close.
	Close() error
}

// Fn is a job handler that is called for each job process.
// When an error is returned, a new occurence will be re-scheduled based on the
// JobOption rules.
type Fn func(ctx context.Context, id string, data []byte) error

// A Job is a one-time task executed at a specific time.
type Job struct {
	// ID is a globally unique identifier
	ID string
	// Target contains the subscriber that needs to be called back when the job
	// is being executed
	Target string
	// Due defines when the job is bound to be executed.
	// It is defined in unix ns since epoch
	Due int64
	// Data is the job payload (optional)
	Data []byte
	// Options contains information about the job execution, such as its retry limit,
	// back off duration upon failure, consistency guarantee, ...
	Options JobOptions
}

// BuildJob builds a new job with its default values and options applied
func BuildJob(o ...JobOption) *Job {
	j := &Job{
		ID: uuid.New().String(),
		Options: JobOptions{
			RetryLimit:  DefaultRetryLimit,
			MinBackOff:  DefaultMinBackOff,
			MaxBackOff:  DefaultMaxBackOff,
			Consistency: AtLeastOnce,
		},
	}
	for _, o := range o {
		o(&j.Options)
	}
	return j
}

// JobOption configures how we set up a job
type JobOption func(*JobOptions)

// JobOptions configure a Job. JobOptions are set by the JobOption values passed
// to At, In, or Interval.
type JobOptions struct {
	RetryLimit  uint32
	MinBackOff  time.Duration
	MaxBackOff  time.Duration
	AgeLimit    *time.Duration
	Consistency Consistency
}

// WithConsistency sets the job consistency guarantee when it uses a distributed scheduler
//
// It can either be executed at most once or at least once. The consistency guarantee
// strongly depends on the situation.
func WithConsistency(c Consistency) JobOption {
	return func(o *JobOptions) {
		o.Consistency = c
	}
}

// WithRetryLimit sets how many times a job can be retried upon failure.
//
// When omitted from the parameters, the limit is set to 'DefaultRetryLimit' by default.
func WithRetryLimit(l uint32) JobOption {
	return func(o *JobOptions) {
		o.RetryLimit = l
	}
}

// MinBackOff sets the minimum duration to wait before retrying a job after it fails.
func MinBackOff(d time.Duration) JobOption {
	return func(o *JobOptions) {
		o.MinBackOff = d
	}
}

// MaxBackOff sets the maximum duration to wait before retrying a job after it fails.
func MaxBackOff(d time.Duration) JobOption {
	return func(o *JobOptions) {
		o.MaxBackOff = d
	}
}

// WithAgeLimit sets a time limit for retrying a failed job, measured from when
// the job was first run. If specified with WithRetryLimit, the scheduler retries
// the job until both limits are reached.
func WithAgeLimit(d time.Duration) JobOption {
	return func(o *JobOptions) {
		o.AgeLimit = &d
	}
}

// Consistency is a job consistency guarantee on a distributed system
type Consistency uint8

const (
	// AtMostOnce is a consistency guarantee when a job is on a distributed scheduler
	// that ensures the job will be executed at most once.
	// That means it will be either executed once or not executed at all.
	AtMostOnce Consistency = iota
	// AtLeastOnce is a consistency guarantee when a job is on a distributed scheduler
	// that ensures the job will be executed at least once.
	// That means it will be either executed once or executed multiple times.
	AtLeastOnce
)

// Config contains the configuration to start a job scheduler
type Config struct {
	On      bool              `json:"on"`
	Adapter string            `json:"adapter"`
	Config  map[string]string `json:"config"`
}

// At calls `At` on the context `Scheduler`
func At(
	ctx context.Context, t time.Time, target string, data []byte, o ...JobOption,
) (string, error) {
	return SchedulerFromContext(ctx).At(ctx, t, target, data, o...)
}

// In calls `In` on the context `Scheduler`
func In(
	ctx context.Context,
	d time.Duration,
	target string,
	data []byte,
	o ...JobOption,
) (string, error) {
	return SchedulerFromContext(ctx).In(ctx, d, target, data, o...)
}

// An Event is an occurence of a job executed at a specific time.
// There is one event per job execution.
type Event struct {
	ID      string
	Due     int64
	Attempt uint32

	Job Job
}

type Storage interface {
	Create(e *Event) error
	Load(uuid string) (*Event, error)
	Delete(uuid string)
	Next(from, to time.Time) (*Event, error)
	Between(from, to time.Time) (EventIterator, error)
}

type EventIterator interface {
	// Advance attempts to advance the iterator to the next record. Advance
	// returns true if there are more records satisfying the range, or false
	// if the range has been exhausted. You must call this prev every call to Load
	Advance() bool
	// Load returns the Event
	Load() (*Event, error)
}

type contextKey struct{}

var activeSchedulerContextKey = contextKey{}

// SchedulerFromContext returns an `Scheduler` instance associated with `ctx`, or
// the local `Scheduler` if no instance could be found.
func SchedulerFromContext(ctx contextutil.ValueContext) Scheduler {
	val := ctx.Value(activeSchedulerContextKey)
	if o, ok := val.(Scheduler); ok {
		return o
	}
	return NopScheduler()
}

// SchedulerWithContext returns a copy of parent in which the `Scheduler` is stored
func SchedulerWithContext(ctx context.Context, agent Scheduler) context.Context {
	return context.WithValue(ctx, activeSchedulerContextKey, agent)
}
