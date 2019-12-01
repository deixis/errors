package log

import (
	"context"
	"fmt"
)

// Logger is an interface for app loggers
type Logger interface {
	// Trace level logs are to follow the code executio step by step
	Trace(tag, msg string, fields ...Field)
	// Warning level logs are meant to draw attention above a certain threshold
	// e.g. wrong credentials, 404 status code returned, upstream node down
	Warning(tag, msg string, fields ...Field)
	// Error level logs need immediate attention
	// The 2AM rule applies here, which means that if you are on call, this log line will wake you up at 2AM
	// e.g. all critical upstream nodes are down, disk space is full
	Error(tag, msg string, fields ...Field)

	// With returns a child logger, and optionally add some context to that logger
	With(fields ...Field) Logger

	// AddCalldepth adds the given value to calldepth
	// Calldepth is the count of the number of
	// frames to skip when computing the file name and line number
	AddCalldepth(n int) Logger

	// Close implements the Closer interface
	Close() error
}

// Formatter converts a log line to a specific format, such as JSON
type Formatter interface {
	// Format formats the given log line
	Format(ctx *Ctx, tag, msg string, fields ...Field) (string, error)
}

// Printer outputs a log line somewhere, such as stdout, syslog, 3rd party service
type Printer interface {
	// Print prints the given log line
	Print(ctx *Ctx, s string) error

	// Close implements the Closer interface
	Close() error
}

// Level defines log severity
type Level int

// ParseLevel parses a string representation of a log level
func ParseLevel(s string) Level {
	switch s {
	case "trace":
		return LevelTrace
	case "warning":
		return LevelWarning
	case "error":
		return LevelError
	}
	return LevelTrace
}

const (
	// LevelTrace displays logs with trace level (and above)
	LevelTrace Level = iota
	// LevelWarning displays logs with warning level (and above)
	LevelWarning
	// LevelError displays only logs with error level
	LevelError
)

// String returns a string representation of the given level
func (l Level) String() string {
	switch l {
	case LevelTrace:
		return "TR"
	case LevelWarning:
		return "WN"
	case LevelError:
		return "ER"
	default:
		panic(fmt.Sprintf("unknown level <%d>", l))
	}
}

// Ctx carries the log line context (level, timestamp, ...)
type Ctx struct {
	Level     string
	Timestamp string
	Service   string
	File      string
}

// Trace calls `Trace` on the context `Logger`
func Trace(ctx context.Context, tag, msg string, fields ...Field) {
	FromContext(ctx).Trace(tag, msg, fields...)
}

// Warn calls `Warning` on the context `Logger`
func Warn(ctx context.Context, tag, msg string, fields ...Field) {
	FromContext(ctx).Warning(tag, msg, fields...)
}

// Err calls `Error` on the context `Logger`
func Err(ctx context.Context, tag, msg string, fields ...Field) {
	FromContext(ctx).Error(tag, msg, fields...)
}

type contextKey struct{}

var activeContextKey = contextKey{}

// FromContext returns a `Logger` instance associated with `ctx`, or
// `NopLogger` if no `Logger` instance could be found.
func FromContext(ctx context.Context) Logger {
	val := ctx.Value(activeContextKey)
	if o, ok := val.(Logger); ok {
		return o
	}
	return NopLogger()
}

// WithContext returns a copy of parent in which the `Logger` is stored
func WithContext(ctx context.Context, l Logger) context.Context {
	return context.WithValue(ctx, activeContextKey, l)
}

// NopLogger returns a no-op `Logger`
func NopLogger() Logger {
	return &nopLogger{}
}

type nopLogger struct{}

func (l *nopLogger) Trace(tag, msg string, fields ...Field)   {}
func (l *nopLogger) Warning(tag, msg string, fields ...Field) {}
func (l *nopLogger) Error(tag, msg string, fields ...Field)   {}
func (l *nopLogger) With(fields ...Field) Logger              { return &nopLogger{} }
func (l *nopLogger) AddCalldepth(n int) Logger                { return &nopLogger{} }
func (l *nopLogger) Close() error                             { return nil }
