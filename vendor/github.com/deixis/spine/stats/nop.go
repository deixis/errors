package stats

import (
	"time"

	"github.com/deixis/spine/log"
)

type nop struct{}

// NopStats returns a stats adapter that does not do anything
func NopStats() Stats {
	return &nop{}
}

func (s *nop) Start()                                                         {}
func (s *nop) Stop()                                                          {}
func (s *nop) SetLogger(l log.Logger)                                         {}
func (s *nop) Count(key string, n interface{}, meta ...map[string]string)     {}
func (s *nop) Inc(key string, meta ...map[string]string)                      {}
func (s *nop) Dec(key string, meta ...map[string]string)                      {}
func (s *nop) Gauge(key string, n interface{}, meta ...map[string]string)     {}
func (s *nop) Timing(key string, t time.Duration, meta ...map[string]string)  {}
func (s *nop) Histogram(key string, n interface{}, tags ...map[string]string) {}
func (s *nop) With(meta map[string]string) Stats {
	return &nop{}
}
func (s *nop) Log(l log.Logger) Stats {
	return &nop{}
}
