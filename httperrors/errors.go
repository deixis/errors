package httperrors

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/deixis/errors"
	"github.com/deixis/pkg/httputil"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
)

// Marshal marshals `err` to the HTTP response writer
func Marshal(r *http.Request, w http.ResponseWriter, err error) error {
	status := Pack(err)
	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status.Code())

	h := w.Header()
	for k, v := range status.Header {
		for i := range v {
			h.Add(k, v[i])
		}
	}

	// TODO: Load encoder
	// TODO: Load Accept-Language

	enc := json.NewEncoder(w)
	return enc.Encode(struct {
		Error interface{} `json:"error"`
	}{
		Error: status.statusError,
	})
}

// Pack returns a Status representing err if it was produced from an
// `*errors.Error` struct.
func Pack(err error) *Status {
	s, _ := pack(err)
	return s
}

// Pack returns a Status representing err if it was produced from an
// `*errors.Error` struct. Otherwise, ok is false and a Status is returned
// with http.StatusInternalServerError and the original error message.
func pack(err error) (*Status, bool) {
	if err == nil {
		return New(http.StatusOK, ""), true
	}

	switch err {
	case context.Canceled, context.DeadlineExceeded:
		return New(http.StatusGatewayTimeout, err.Error()), true
	}

	switch err := err.(type) {
	case *errors.AvailabilityFailure:
		s := New(http.StatusServiceUnavailable, err.Error())
		httputil.FormatRetryAfter(s.Header, err.RetryInfo.RetryDelay)
		return s, true
	case *errors.PermissionFailure:
		return New(http.StatusForbidden, err.Error()), true
	case *errors.AuthenticationFailure:
		return New(http.StatusUnauthorized, err.Error()), true
	case *errors.MissingFailure:
		return New(http.StatusNotFound, err.Error()), true
	case *errors.BadRequest:
		s := New(http.StatusBadRequest, err.Error())
		detail := &errdetails.BadRequest{
			FieldViolations: make([]*errdetails.BadRequest_FieldViolation, len(err.Violations)),
		}
		for i, violation := range err.Violations {
			detail.FieldViolations[i] = &errdetails.BadRequest_FieldViolation{
				Field:       violation.Field,
				Description: violation.Description,
			}
		}
		s.Details = []interface{}{detail}
		return s, true
	case *errors.PreconditionFailure:
		s := New(http.StatusPreconditionFailed, err.Error())
		detail := &errdetails.PreconditionFailure{
			Violations: make([]*errdetails.PreconditionFailure_Violation, len(err.Violations)),
		}
		for i, violation := range err.Violations {
			detail.Violations[i] = &errdetails.PreconditionFailure_Violation{
				Type:        violation.Type,
				Subject:     violation.Subject,
				Description: violation.Description,
			}
		}
		s.Details = []interface{}{detail}
		return s, true
	case *errors.ConflictFailure:
		return New(http.StatusConflict, err.Error()), true
	case *errors.QuotaFailure:
		s := New(http.StatusTooManyRequests, err.Error())
		detail := &errdetails.QuotaFailure{
			Violations: make([]*errdetails.QuotaFailure_Violation, len(err.Violations)),
		}
		for i, violation := range err.Violations {
			detail.Violations[i] = &errdetails.QuotaFailure_Violation{
				Subject:     violation.Subject,
				Description: violation.Description,
			}
		}
		s.Details = []interface{}{detail}
		return s, true
	default:
		return New(http.StatusInternalServerError, err.Error()), false
	}
}

// Status represents an HTTP status code, message, and details. It is immutable
// and should be created with New, or Newf.
type Status struct {
	statusError
}

// Code returns the status code contained in s.
func (s *Status) Code() int {
	if s == nil {
		return http.StatusOK
	}
	return s.statusError.Code
}

// Message returns the message contained in s.
func (s *Status) Message() string {
	if s == nil {
		return ""
	}
	return s.statusError.Message
}

// Err returns an immutable error representing s; returns nil if s.Code() is OK.
func (s *Status) Err() error {
	if s.Code() == http.StatusOK {
		return nil
	}
	return s
}

// New returns a Status representing c and msg.
func New(code int, msg string) *Status {
	return &Status{statusError{Code: code, Message: msg, Header: http.Header{}}}
}

// Newf returns New(c, fmt.Sprintf(format, a...)).
func Newf(code int, format string, a ...interface{}) *Status {
	return New(code, fmt.Sprintf(format, a...))
}

type statusError struct {
	Code    int           `json:"-"`
	Header  http.Header   `json:"-"`
	Message string        `json:"message"`
	Details []interface{} `json:"details"`
}

func (se *statusError) Error() string {
	if se == nil {
		return ""
	}
	return fmt.Sprintf("http error: code = %d desc = %s", se.Code, se.Message)
}

func (se *statusError) HTTPStatus() *Status {
	if se == nil {
		return nil
	}
	return &Status{*se}
}
