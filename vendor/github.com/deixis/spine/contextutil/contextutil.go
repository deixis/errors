package contextutil

// ValueContext requires given contexts to only implements Value.
// This is particarily useful for functions that return attached values to
// a context without caring about other context functions.
//
// This was implemented for more flexibility when given contexts slightly differ
// from the standard go context interface.
//
// It is the case of Temporal workflow context.
// See: https://github.com/temporalio/temporal
type ValueContext interface {
	Value(key interface{}) interface{}
}
