package schedule

import (
	"context"
	"time"
)

// nopScheduler is a scheduler which does not do anything.
type nopScheduler struct{}

// NopScheduler returns a scheduler which discards all jobs
func NopScheduler() Scheduler {
	return &nopScheduler{}
}

func (s *nopScheduler) Start(ctx context.Context) error {
	return nil
}

func (s *nopScheduler) HandleFunc(
	target string, fn Fn,
) (deregister func(), err error) {
	return func() {}, nil
}

func (s *nopScheduler) At(
	ctx context.Context,
	t time.Time,
	target string,
	data []byte,
	o ...JobOption,
) (string, error) {
	return "", nil
}

func (s *nopScheduler) In(
	ctx context.Context,
	d time.Duration,
	target string,
	data []byte,
	o ...JobOption,
) (string, error) {
	return "", nil
}

func (s *nopScheduler) Drain() {}

func (s *nopScheduler) Close() error {
	return nil
}
