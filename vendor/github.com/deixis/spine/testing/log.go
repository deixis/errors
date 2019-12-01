package testing

import (
	"bytes"
	"sync"
	"testing"

	"github.com/deixis/spine/log"
)

const (
	// TC is the TRACE log constant
	TC = "TRACE"
	// WN is the WARNING log constant
	WN = "WARN"
	// ER is the ERROR log constant
	ER = "ERRR"
)

// Logger is a simple Logger interface useful for tests
type Logger struct {
	mu sync.RWMutex
	t  *testing.T

	calldepth int
	lines     *counter
	fields    []log.Field
	strict    bool
}

// NewLogger creates a new logger
func NewLogger(t *testing.T, strict bool) log.Logger {
	return &Logger{
		t:         t,
		calldepth: 1,
		lines:     &counter{},
		strict:    strict,
	}
}

func (l *Logger) l(s, tag, msg string, args ...log.Field) {
	l.t.Log(s, format(tag, msg, args...))
	l.inc(s)
}

func (l *Logger) inc(s string) {
	l.lines.inc(s)
}

// Lines returns the number of log lines for the given severity
func (l *Logger) Lines(s string) int {
	return l.lines.count(s)
}

func (l *Logger) Trace(tag, msg string, fields ...log.Field)   { l.l(TC, tag, msg, fields...) }
func (l *Logger) Warning(tag, msg string, fields ...log.Field) { l.l(WN, tag, msg, fields...) }
func (l *Logger) Error(tag, msg string, fields ...log.Field) {
	l.l(ER, tag, msg, fields...)

	if l.strict {
		l.t.Error(format(tag, msg, fields...)) // Make the tests fail
	}
}
func (l *Logger) With(fields ...log.Field) log.Logger {
	return &Logger{
		t:         l.t,
		calldepth: l.calldepth,
		lines:     l.lines,
		fields:    append(l.fields, fields...),
		strict:    l.strict,
	}
}
func (l *Logger) AddCalldepth(n int) log.Logger {
	return &Logger{
		t:         l.t,
		calldepth: l.calldepth + n,
		lines:     l.lines,
		fields:    l.fields,
		strict:    l.strict,
	}
}
func (l *Logger) Close() error {
	return nil
}

func format(tag, msg string, fields ...log.Field) string {
	var b bytes.Buffer

	b.WriteString(tag)
	b.WriteString(" ")
	b.WriteString(msg)
	b.WriteString(" ")

	for _, f := range fields {
		k, v := f.KV()
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(v)
		b.WriteString(" ")
	}
	return b.String()
}

type counter struct {
	mu   sync.RWMutex
	smap sync.Map
}

func (c *counter) inc(s string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	i, ok := c.smap.Load(s)
	if ok {
		c.smap.Store(s, i.(int)+1)
	} else {
		c.smap.Store(s, 1)
	}
}

func (c *counter) count(s string) int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.smap.Load(s)
	if !ok {
		return 0
	}
	return v.(int)
}
