package bg

import "time"

// Task is a background job for simple long running tasks
type Task struct {
	done chan struct{}
	f    func()
}

// NewTask returns a new Task
func NewTask(f func()) *Task {
	return &Task{
		done: make(chan struct{}, 1),
		f:    f,
	}
}

func (t *Task) Start() {
	defer func() {
		recover()
		t.done <- struct{}{}
	}()

	t.f()
}

func (t *Task) Stop() {
	select {
	case <-t.done:
	case <-time.After(time.Second * 10):
	}
}
