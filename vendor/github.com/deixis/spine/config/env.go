package config

import (
	"os"
	"strings"
)

const prefix = "$"

// ValueOf extracts the environment variable name given or the plain string given
//
// e.g. foo -> foo
//      $DATABASE_URL -> http://foo.bar:8083
func ValueOf(s string) string {
	if strings.HasPrefix(s, prefix) && len(s) > 1 {
		return os.Getenv(s[1:])
	}
	return s
}

// ValuesOf extracts the environment variable(s) from v
func ValuesOf(v interface{}) interface{} {
	switch v := v.(type) {
	case string:
		if strings.HasPrefix(v, prefix) && len(v) > 1 {
			return os.Getenv(v[1:])
		}
	case []interface{}:
		r := make([]interface{}, len(v))
		for i := range v {
			r[i] = ValuesOf(v[i])
		}
		return r
	}
	return v
}
