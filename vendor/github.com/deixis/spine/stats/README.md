# Stats package

Package stats provides functions for reporting metrics to a service.

### Code

```go
import (
	"context"
	"net/http"
	"runtime/debug"
	"strconv"
	"time"

	opentracing "github.com/opentracing/opentracing-go"
	olog "github.com/opentracing/opentracing-go/log"
	scontext "github.com/deixis/spine/context"
	"github.com/deixis/spine/log"
	"github.com/deixis/spine/stats"
	"github.com/deixis/spine/tracing"
)

func mwStats(next ServeFunc) ServeFunc {
	return func(ctx context.Context, w ResponseWriter, r *Request) {
		stats := stats.FromContext(ctx)
		tags := map[string]string{
			"method": r.method,
			"path":   r.path,
		}
		stats.Inc("http.conc", tags)

		// Next middleware
		next(ctx, w, r)

		tags["status"] = strconv.Itoa(w.Code())
		stats.Histogram("http.call", 1, tags)
		stats.Timing("http.time", time.Since(r.startTime), tags)
		stats.Dec("http.conc", tags)
	}
}
```

### Config

```toml
[stats.statsd]
  addr = "127.0.0.1"
  port = "8125"
  tags_format = "influxdb"

[stats.statsd.tags]
  foo = "bar"
```