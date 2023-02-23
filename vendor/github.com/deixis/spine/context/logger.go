package context

import (
	"context"

	"github.com/deixis/spine/log"
	"github.com/deixis/spine/stats"
	"github.com/opentracing/opentracing-go"
)

// WithLogger returns a copy of parent with a contextualised `log.Logger`
func WithLogger(ctx context.Context, l log.Logger) context.Context {
	return log.WithContext(ctx, &logger{
		Leg:  TransitFromContext(ctx),
		S:    stats.FromContext(ctx),
		Span: opentracing.SpanFromContext(ctx),
		Log:  l.AddCalldepth(1),
	})
}

// logger wraps a `log.Logger` to contextualise log messages
type logger struct {
	Leg  Leg
	S    stats.Stats
	Span opentracing.Span
	Log  log.Logger
}

func (l *logger) Trace(tag, msg string, fields ...log.Field) {
	l.incTag(tag)
	l.incLogLevelCount(log.LevelTrace, tag)
	if l.Span != nil {
		l.Span.LogEvent(tag)
	}

	// TODO: Use Step return by Tick and pass it to logFields to make sure steps are logged only once
	l.Leg.Tick()
	l.Log.Trace(tag, msg, l.logFields(fields)...)
}

func (l *logger) Warning(tag, msg string, fields ...log.Field) {
	l.incTag(tag)
	l.incLogLevelCount(log.LevelWarning, tag)
	if l.Span != nil {
		l.Span.LogEvent(tag)
	}

	l.Leg.Tick()
	l.Log.Warning(tag, msg, l.logFields(fields)...)
}

func (l *logger) Error(tag, msg string, fields ...log.Field) {
	l.incTag(tag)
	l.incLogLevelCount(log.LevelError, tag)
	if l.Span != nil {
		l.Span.LogEvent(tag)
	}

	l.Leg.Tick()
	l.Log.Error(tag, msg, l.logFields(fields)...)
}

func (l *logger) With(fields ...log.Field) log.Logger {
	return l.Log.With(fields...)
}

func (l *logger) AddCalldepth(n int) log.Logger {
	return l.Log.AddCalldepth(n)
}

func (l *logger) Close() error {
	return l.Log.Close()
}

func (l *logger) logFields(fields []log.Field) []log.Field {
	return log.JoinFields(
		[]log.Field{
			log.String("id", l.Leg.ShortID()),
			log.Stringer("step", l.Leg.Step()),
		},
		fields,
	)
}

func (l *logger) incTag(tag string) {
	l.S.Histogram(statsLog, 1, map[string]string{
		"tag": tag,
	})
}

func (l *logger) incLogLevelCount(lvl log.Level, tag string) {
	l.S.Histogram(statsLogLvl, 1, map[string]string{
		"level": lvl.String(),
		"tag":   tag,
	})
}

const (
	statsLog    = "log"
	statsLogLvl = "log.level"
)
