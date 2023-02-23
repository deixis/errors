package testing

import (
	"context"
	"sync"
	"testing"

	"github.com/deixis/spine/config"
	"github.com/deixis/spine/disco"
	"github.com/deixis/spine/log"
	"github.com/deixis/spine/stats"
)

// T is a wrapper of go standard testing.T
// It adds a few additional functions useful to spine
type T struct {
	once sync.Once
	t    *testing.T

	logger log.Logger
	stats  stats.Stats
	config *config.Config
	disco  disco.Agent
}

// New returns a new instance of T
func New(t *testing.T) *T {
	config := &config.Config{}
	return &T{
		t:      t,
		logger: NewLogger(t, true),
		stats:  NewStats(t),
		config: config,
		disco:  disco.NewLocalAgent(),
	}
}

// Logger returns a spine logger interface
func (t *T) Logger() log.Logger {
	return t.logger
}

// Stats returns a spine stats interface
func (t *T) Stats() stats.Stats {
	return t.stats
}

// Config returns an empty spine config
func (t *T) Config() *config.Config {
	return t.config
}

// Disco returns a local service discovery agent
func (t *T) Disco() disco.Agent {
	return t.disco
}

// WithCancel returns a new context
func (t *T) WithCancel(parent context.Context) (context.Context, func()) {
	ctx, cancel := context.WithCancel(parent)
	ctx = log.WithContext(ctx, t.logger)
	ctx = stats.WithContext(ctx, t.stats)
	ctx = disco.AgentWithContext(ctx, t.disco)
	return ctx, cancel
}

// DisableStrictMode will stop making error logs failing a test
func (t *T) DisableStrictMode() {
	t.logger = NewLogger(t.t, false)
}

// Standard go testing.T functions
func (t *T) Error(args ...interface{}) {
	t.t.Error(args...)
}

func (t *T) Errorf(format string, args ...interface{}) {
	t.t.Errorf(format, args...)
}

func (t *T) Fail() {
	t.t.Fail()
}

func (t *T) FailNow() {
	t.t.FailNow()
}

func (t *T) Failed() {
	t.t.Failed()
}

func (t *T) Fatal(args ...interface{}) {
	t.t.Fatal(args...)
}

func (t *T) Fatalf(format string, args ...interface{}) {
	t.t.Fatalf(format, args...)
}

func (t *T) Log(args ...interface{}) {
	t.t.Log(args...)
}

func (t *T) Logf(format string, args ...interface{}) {
	t.t.Logf(format, args...)
}

func (t *T) Parallel() {
	t.t.Parallel()
}

func (t *T) Skip(args ...interface{}) {
	t.t.Skip(args...)
}

func (t *T) SkipNow() {
	t.t.SkipNow()
}

func (t *T) Skipf(format string, args ...interface{}) {
	t.t.Skipf(format, args...)
}

func (t *T) Skipped() {
	t.t.Skipped()
}
