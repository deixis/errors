package testing

import (
	"sync"
	"testing"
	"time"

	"github.com/deixis/spine/log"
	"github.com/deixis/spine/stats"
)

// StatsOp an operation, such as count, gauge, timing sent to the testing stats
type StatsOp int

const (
	OpCount = iota
	OpInc
	OpDec
	OpGauge
	OpTiming
	OpHistogram
)

// Stats is a simple Stats interface useful for tests
type Stats struct {
	t  *testing.T
	mu sync.Mutex

	Data map[string][]StatsPoint
}

type StatsPoint struct {
	Op   StatsOp
	N    interface{}
	Meta []map[string]string
}

// NewStats creates a new stats
func NewStats(t *testing.T) stats.Stats {
	return &Stats{
		t:    t,
		Data: map[string][]StatsPoint{},
	}
}

func (s *Stats) Start()                 {}
func (s *Stats) Stop()                  {}
func (s *Stats) SetLogger(l log.Logger) {}
func (s *Stats) Count(key string, n interface{}, meta ...map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	k := s.Data[key]
	data := StatsPoint{
		Op:   OpCount,
		N:    n,
		Meta: meta,
	}
	s.Data[key] = append(k, data)
}
func (s *Stats) Inc(key string, meta ...map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	k := s.Data[key]
	data := StatsPoint{
		Op:   OpInc,
		Meta: meta,
	}
	s.Data[key] = append(k, data)
}
func (s *Stats) Dec(key string, meta ...map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	k := s.Data[key]
	data := StatsPoint{
		Op:   OpDec,
		Meta: meta,
	}
	s.Data[key] = append(k, data)
}
func (s *Stats) Gauge(key string, n interface{}, meta ...map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	k := s.Data[key]
	data := StatsPoint{
		Op:   OpGauge,
		N:    n,
		Meta: meta,
	}
	s.Data[key] = append(k, data)
}
func (s *Stats) Timing(key string, d time.Duration, meta ...map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	k := s.Data[key]
	data := StatsPoint{
		Op:   OpTiming,
		N:    d,
		Meta: meta,
	}
	s.Data[key] = append(k, data)
}
func (s *Stats) Histogram(key string, n interface{}, meta ...map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	k := s.Data[key]
	data := StatsPoint{
		Op:   OpHistogram,
		N:    n,
		Meta: meta,
	}
	s.Data[key] = append(k, data)
}
func (s *Stats) With(meta map[string]string) stats.Stats {
	return s
}
func (s *Stats) Log(l log.Logger) stats.Stats {
	return s
}
