package context

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

type Step interface {
	fmt.Stringer
}

const stepperSeparator = "_"

// stepper is the atomic counter for context log lines
type stepper struct {
	mu sync.RWMutex

	Steps []uint32
	I     int
}

// newStepper builds a new main stepper
func newStepper() *stepper {
	return &stepper{
		Steps: []uint32{0},
		I:     0,
	}
}

// Child returns a new "child" stepper
func (s *stepper) Child() *stepper {
	s.mu.Lock()
	defer s.mu.Unlock()

	atomic.AddUint32(&s.Steps[s.I], 1)

	return &stepper{
		Steps: append(s.Steps, 0),
		I:     s.I + 1,
	}
}

// Inc increments the current counter
func (s *stepper) Inc() uint {
	s.mu.Lock()
	defer s.mu.Unlock()

	return uint(atomic.AddUint32(&s.Steps[s.I], 1))
}

// String returns a string representation of the current state
func (s *stepper) String() string {
	t, _ := s.MarshalText()
	return string(t)
}

// MarshalText implements encoding.TextMarshaler
func (s *stepper) MarshalText() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var buf bytes.Buffer

	for i, step := range s.Steps {
		sstep := strconv.FormatUint(uint64(step), 10)

		// Pad number if needed
		rem := 4 - len(sstep)
		if rem > 0 {
			buf.WriteString(strings.Repeat("0", rem))
		}
		buf.WriteString(sstep)

		// Add separator
		if i < s.I {
			buf.WriteString(stepperSeparator)
		}
	}

	return buf.Bytes(), nil
}

// UnmarshalText implements encoding.TextUnmarshaler
func (s *stepper) UnmarshalText(text []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	values := strings.Split(string(text), stepperSeparator)
	steps := make([]uint32, len(values))
	for i, v := range values {
		step, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return err
		}
		steps[i] = uint32(step)
	}

	s.Steps = steps
	s.I = len(steps) - 1
	return nil
}
