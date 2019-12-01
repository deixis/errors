package testing

import (
	"math/rand"
	"sync/atomic"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
	portSequence = &sequencer{n: 9800 + rand.Int31n(6800)}
}

// portSequence returns a sequence of port numbers. It should be used
// for test handlers in order to avoid port clashes
var portSequence *sequencer

type sequencer struct {
	n int32
}

func (s *sequencer) next() int32 {
	return atomic.AddInt32(&s.n, 1)
}

// NextPort returns the next supposedly available port number
func NextPort() int {
	return int(portSequence.next())
}
