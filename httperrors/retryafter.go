package httperrors

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"
)

const retryAfter = "Retry-After"

// Now returns the current time
var Now = time.Now

// parseRetryAfter parses the `Retry-After` header and returns its duration.
// If it does not exist or can't be parsed, it will return 0. The values
// returned are guaranteed to greater or equal to 0.
func parseRetryAfter(h http.Header) (time.Duration, bool) {
	s := h.Get(retryAfter)
	if seconds, err := strconv.ParseInt(s, 10, 32); err == nil {
		if seconds < 0 {
			return 0, true
		}
		return time.Duration(seconds) * time.Second, true
	}
	if t, err := time.Parse(http.TimeFormat, s); err == nil {
		d := Now().Sub(t)
		if d < 0 {
			return 0, true
		}
		return d, true
	}
	return 0, false
}

// formatRetryAfter formats the `Retry-After` response header
func formatRetryAfter(h http.Header, d time.Duration) {
	if d < 0 {
		d = 0
	}
	h.Set(retryAfter, fmt.Sprintf("%d", int(math.Round(d.Seconds()))))
}
