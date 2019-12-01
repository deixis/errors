package errors

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/text/language"
)

var (
	// PermissionDenied indicates the caller does not have permission to
	// execute the specified operation. It must not be used for rejections
	// caused by exhausting some resource (use ResourceExhausted
	// instead for those errors). It must not be
	// used if the caller cannot be identified (use Unauthenticated
	// instead for those errors).
	PermissionDenied error = &PermissionFailure{}

	// Unauthenticated indicates the request does not have valid
	// authentication credentials for the operation.
	Unauthenticated error = &AuthenticationFailure{}

	// NotFound means some requested entity (e.g., file or directory) was
	// not found.
	NotFound error = &MissingFailure{}
)

// WithPermissionDenied wraps `parent` with a `PermissionFailure`
func WithPermissionDenied(parent error) error {
	return &PermissionFailure{parent}
}

// WithUnauthenticated wraps `parent` with an `AuthenticationFailure`
func WithUnauthenticated(parent error) error {
	return &AuthenticationFailure{parent}
}

// WithNotFound wraps `parent` with a `MissingFailure`
func WithNotFound(parent error) error {
	return &MissingFailure{parent}
}

// WithBad wraps `parent` with a `BadRequest`
func WithBad(parent error, violations ...*FieldViolation) error {
	return &BadRequest{parent, violations}
}

// WithFailedPrecondition wraps `parent` with a `PreconditionFailure`
func WithFailedPrecondition(parent error, violations ...*PreconditionViolation) error {
	return &PreconditionFailure{parent, violations}
}

// WithAborted wraps `parent` with a `ConflictFailure`
func WithAborted(parent error, violations ...*ConflictViolation) error {
	return &ConflictFailure{parent, violations}
}

// WithUnavailable wraps `parent` with an `AvailabilityFailure`
func WithUnavailable(parent error, retryDelay time.Duration) error {
	return &AvailabilityFailure{parent, RetryInfo{RetryDelay: retryDelay}}
}

// WithResourceExhausted wraps `parent` with a `QuotaFailure`
func WithResourceExhausted(parent error, violations ...*QuotaViolation) error {
	return &QuotaFailure{parent, violations}
}

// Bad indicates client specified an invalid argument.
// Note that this differs from FailedPrecondition. It indicates arguments
// that are problematic regardless of the state of the system
// (e.g., a malformed file name).
func Bad(violations ...*FieldViolation) error {
	return &BadRequest{Violations: violations}
}

// FailedPrecondition indicates operation was rejected because the
// system is not in a state required for the operation's execution.
// For example, directory to be deleted may be non-empty, an rmdir
// operation is applied to a non-directory, etc.
//
// A litmus test that may help a service implementor in deciding
// between FailedPrecondition, Aborted, and Unavailable:
//  (a) Use Unavailable if the client can retry just the failing call.
//  (b) Use Aborted if the client should retry at a higher-level
//      (e.g., restarting a read-modify-write sequence).
//  (c) Use FailedPrecondition if the client should not retry until
//      the system state has been explicitly fixed. E.g., if an "rmdir"
//      fails because the directory is non-empty, FailedPrecondition
//      should be returned since the client should not retry unless
//      they have first fixed up the directory by deleting files from it.
//  (d) Use FailedPrecondition if the client performs conditional
//      REST Get/Update/Delete on a resource and the resource on the
//      server does not match the condition. E.g., conflicting
//      read-modify-write on the same resource.
func FailedPrecondition(violations ...*PreconditionViolation) error {
	return &PreconditionFailure{Violations: violations}
}

// Aborted indicates the operation was aborted, typically due to a
// concurrency issue like sequencer check failures, transaction aborts,
// etc.
//
// See litmus test above for deciding between FailedPrecondition,
// Aborted, and Unavailable.
func Aborted(violations ...*ConflictViolation) error {
	return &ConflictFailure{Violations: violations}
}

// Unavailable indicates the service is currently unavailable.
// This is a most likely a transient condition and may be corrected
// by retrying with a backoff.
//
// See litmus test above for deciding between FailedPrecondition,
// Aborted, and Unavailable.
func Unavailable(retryDelay time.Duration) error {
	return &AvailabilityFailure{RetryInfo: RetryInfo{RetryDelay: retryDelay}}
}

// ResourceExhausted indicates some resource has been exhausted, perhaps
// a per-user quota, or perhaps the entire file system is out of space.
func ResourceExhausted(violations ...*QuotaViolation) error {
	return &QuotaFailure{Violations: violations}
}

func IsPermissionDenied(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*PermissionFailure)
	return ok
}

func IsUnauthenticated(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*AuthenticationFailure)
	return ok
}

func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*MissingFailure)
	return ok
}

func IsBad(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*BadRequest)
	return ok
}

func IsFailedPrecondition(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*PreconditionFailure)
	return ok
}

func IsAborted(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*ConflictFailure)
	return ok
}

func IsUnavailable(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*AvailabilityFailure)
	return ok
}

func IsResourceExhausted(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*QuotaFailure)
	return ok
}

type AvailabilityFailure struct {
	error

	RetryInfo RetryInfo
}

func (e *AvailabilityFailure) Error() string {
	if e.RetryInfo.RetryDelay > 0 {
		return fmt.Sprintf("service temporarily unavailable, retry in %s", e.RetryInfo.RetryDelay)
	}
	return "service temporarily unavailable"
}

// Describes how a quota check failed.
//
// For example if a daily limit was exceeded for the calling project,
// a service could respond with a QuotaFailure detail containing the project
// id and the description of the quota limit that was exceeded.  If the
// calling project hasn't enabled the service in the developer console, then
// a service could respond with the project id and set `service_disabled`
// to true.
//
// Also see RetryDetail and Help types for other details about handling a
type QuotaFailure struct {
	error

	// Describes all quota violations.
	Violations []*QuotaViolation
}

func (e *QuotaFailure) Error() string {
	if len(e.Violations) == 0 {
		return maybeWrap(e.error, "quota failure").Error()
	}

	s := make([]string, len(e.Violations))
	for i := range e.Violations {
		s[i] = e.Violations[i].Description
	}
	return maybeWrap(e.error, strings.Join(s, ". ")).Error()
}

// A message type used to describe a single quota violation. For example, a
// daily quota or a custom quota that was exceeded.
type QuotaViolation struct {
	// The subject on which the quota check failed.
	// For example, "clientip:<ip address of client>" or "project:<Google
	// developer project id>".
	Subject string
	// A description of how the quota check failed. Clients can use this
	// description to find more about the quota configuration in the service's
	// public documentation, or find the relevant quota limit to adjust through
	// developer console.
	//
	// For example: "Service disabled" or "Daily Limit for read operations
	// exceeded".
	Description string
}

func (v *QuotaViolation) String() string {
	return strings.Join([]string{v.Subject, v.Description}, " - ")
}

// Describes what preconditions have failed.
//
// For example, if an RPC failed because it required the Terms of Service to be
// acknowledged, it could list the terms of service violation in the
// PreconditionFailure message.
type PreconditionFailure struct {
	error

	// Describes all precondition violations.
	Violations []*PreconditionViolation
}

func (e *PreconditionFailure) Error() string {
	if len(e.Violations) == 0 {
		return maybeWrap(e.error, "precondition failure").Error()
	}

	s := make([]string, len(e.Violations))
	for i := range e.Violations {
		s[i] = e.Violations[i].Description
	}
	return maybeWrap(e.error, strings.Join(s, ". ")).Error()
}

// A message type used to describe a single precondition failure.
type PreconditionViolation struct {
	// The type of PreconditionFailure. We recommend using a service-specific
	// enum type to define the supported precondition violation types. For
	// example, "TOS" for "Terms of Service violation".
	Type string
	// The subject, relative to the type, that failed.
	// For example, "google.com/cloud" relative to the "TOS" type would
	// indicate which terms of service is being referenced.
	Subject string
	// A description of how the precondition failed. Developers can use this
	// description to understand how to fix the failure.
	//
	// For example: "Terms of service not accepted".
	Description string
}

func (v *PreconditionViolation) String() string {
	return strings.Join([]string{v.Type, v.Subject, v.Description}, " - ")
}

// Describes violations in a client request. This error type focuses on the
// syntactic aspects of the request.
type BadRequest struct {
	error

	// Describes all violations in a client request.
	Violations []*FieldViolation
}

func (e *BadRequest) Error() string {
	if len(e.Violations) == 0 {
		return maybeWrap(e.error, "bad request").Error()
	}

	s := make([]string, len(e.Violations))
	for i := range e.Violations {
		s[i] = e.Violations[i].Description
	}
	return maybeWrap(e.error, strings.Join(s, ". ")).Error()
}

// A message type used to describe a single bad request field.
type FieldViolation struct {
	// A path leading to a field in the request body. The value will be a
	// sequence of dot-separated identifiers that identify a protocol buffer
	// field. E.g., "field_violations.field" would identify this field.
	Field string
	// A description of why the request element is bad.
	Description string
}

func (v *FieldViolation) String() string {
	return strings.Join([]string{v.Field, v.Description}, " - ")
}

// Describes violations in a client request. This error type focuses on the
// syntactic aspects of the request.
type ConflictFailure struct {
	error

	// Describes all violations in a client request.
	Violations []*ConflictViolation
}

func (e *ConflictFailure) Error() string {
	if len(e.Violations) == 0 {
		return maybeWrap(e.error, "conflict").Error()
	}

	s := make([]string, len(e.Violations))
	for i := range e.Violations {
		s[i] = e.Violations[i].Description
	}
	return maybeWrap(e.error, strings.Join(s, ". ")).Error()
}

type ConflictViolation struct {
	// resource on which the conflict occurred.
	// For example, "user:<uuid>" or "billing/invoice:<uuid>".
	Resource string
	// A description of why the request element is bad.
	Description string
}

func (v *ConflictViolation) String() string {
	return strings.Join([]string{v.Resource, v.Description}, " - ")
}

type MissingFailure struct {
	error
}

func (e *MissingFailure) Error() string {
	return "resource not found"
}

type PermissionFailure struct {
	error
}

func (e *PermissionFailure) Error() string {
	return "permission denied"
}

type AuthenticationFailure struct {
	error
}

func (e *AuthenticationFailure) Error() string {
	return "failed to authenticate request"
}

// RetryInfo describes when the clients can retry a failed request.
// Clients could ignore the recommendation here or retry when this information
// is missing from error responses.
//
// It's always recommended that clients should use exponential backoff when
// retrying.
//
// Clients should wait until `retry_delay` amount of time has passed since
// receiving the error response before retrying.  If retrying requests also
// fail, clients should use an exponential backoff scheme to gradually increase
// the delay between retries based on `retry_delay`, until either a maximum
// number of retires have been reached or a maximum retry delay cap has been
// reached.
type RetryInfo struct {
	// Clients should wait at least this long between retrying the same request.
	RetryDelay time.Duration
}

// Provides a localized error message that is safe to return to the user
// which can be attached to an RPC error.
type LocalisedMessage struct {
	// The locale used following the specification defined at
	// http://www.rfc-editor.org/rfc/bcp/bcp47.txt.
	// Examples are: "en-US", "fr-CH", "es-MX"
	Locale language.Tag
	// The localized error message in the above locale.
	Message string
}

// LocalisedString is a string that can contain multiple translations
//
// The locale used following the specification defined at
// http://www.rfc-editor.org/rfc/bcp/bcp47.txt.
// Examples are: "en-US", "fr-CH", "es-MX"
type LocalisedString map[string]string

// Match finds the best supported language based on the preferred list and
// the languages for which there exists translations
func (s LocalisedString) Match(locales ...string) string {
	if len(s) == 0 {
		return ""
	}

	available := make([]language.Tag, 0, len(s))
	for t := range s {
		available = append(available, language.MustParse(t))
	}

	preferred := make([]language.Tag, 0, len(locales))
	for t := range s {
		tag, err := language.Parse(t)
		if err != nil {
			panic(err)
		}
		preferred = append(preferred, tag)
	}

	t, _, _ := language.NewMatcher(available).Match(preferred...)
	return s[t.String()]
}

func (s LocalisedString) String() string {
	return s.Match(language.English.String())
}

func maybeWrap(err error, message string) error {
	if err != nil {
		return Wrap(err, message)
	}
	return New(message)
}
