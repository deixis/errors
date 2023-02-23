# Tracing package

Package tracing provides functions for reporting traces to a service.

### Code

```go
import (
  "context"

  "github.com/deixis/spine/tracing"
  "github.com/deixis/storage/kvdb"
  opentracing "github.com/opentracing/opentracing-go"
  olog "github.com/opentracing/opentracing-go/log"
)


func (c *store) ReadTransact(
	ctx context.Context,
	f func(kvdb.ReadTransaction) (interface{}, error),
) (interface{}, error) {
	var span opentracing.Span
	span, ctx = tracing.StartSpanFromContext(ctx, "storage.kvdb.readTx")
	defer span.Finish()
	span.LogFields(
		olog.String("type", "storage.kv"),
	)

	return c.s.ReadTransact(ctx, func(t kvdb.ReadTransaction) (interface{}, error) {
		return f(&readTransaction{t: t, ctx: ctx})
	})
}
```

### Config

```toml
[tracing.jaeger]
  service_name = "liquidator"

[tracing.jaeger.sampler]
  type = "probabilistic"
  param = 1.0

[tracing.jaeger.reporter]
  log_span = true

[tracing.jaeger.tags]
  node = "$HOSTNAME"
  version = "$VERSION"
```