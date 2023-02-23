package context

import (
	"bytes"
	"context"
	"encoding"

	"github.com/deixis/spine/context/contextpb"
	"github.com/golang/protobuf/proto"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

// TransitFactory creates empty Transit instances
var TransitFactory = func() Transit {
	return &transit{
		ID:      uuid.New().String(),
		Stepper: newStepper(),
	}
}

// A Transit is request context that goes beyond a process. It is composed of
// multiple `Leg`s.
type Transit interface {
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler
	encoding.TextMarshaler
	encoding.TextUnmarshaler

	Leg

	// Transmit returns a new instance of `Transmit` that can be serialised onto
	// an outbound request.
	Transmit() Transit
	// Inject injects `Transit` for propagation within `carrier`.
	// The actual type of `carrier` depends on the value of `format`.
	// Inject(format interface{}, carrier interface{}) error
	// Extract extracts `Transit` data from `carrier`.
	// Extract(format interface{}, carrier interface{}) error
}

type transit struct {
	ID      string
	Stepper *stepper

	// Origin - Inbound node
}

// UUID returns the transit UUID
func (t *transit) UUID() string {
	if t == nil {
		return ""
	}
	return t.ID
}

// ShortID returns a partial representation of the transit ID (first 8 chars).
func (t *transit) ShortID() string {
	if t == nil || len(t.ID) < 8 {
		return ""
	}
	return string(t.ID[:8])
}

func (t *transit) Tick() uint {
	return t.Stepper.Inc()
}

func (t *transit) Step() Step {
	return t.Stepper
}

func (t *transit) Transmit() Transit {
	return &transit{
		ID:      t.ID,
		Stepper: t.Stepper.Child(),
	}
}

func (t *transit) Inject(format interface{}, carrier interface{}) error {
	return nil
}

func (t *transit) Extract(format interface{}, carrier interface{}) error {
	return nil
}

func (t *transit) MarshalBinary() (data []byte, err error) {
	return proto.Marshal(&contextpb.Context{
		ID:      t.ID,
		Stepper: t.Stepper.String(),
	})
}

func (t *transit) UnmarshalBinary(data []byte) error {
	pb := &contextpb.Context{}
	if err := proto.Unmarshal(data, pb); err != nil {
		return ErrInvalidTransitBinary
	}
	t.ID = pb.ID
	return t.Stepper.UnmarshalText([]byte(pb.Stepper))
}

var (
	textMarshallerSep = []byte("#")

	// ErrInvalidTransitBinary occurs when UnmarshalBinary is called on Transit
	// with an invalid binary representation
	ErrInvalidTransitBinary = errors.New("invalid transit binary representation")

	// ErrInvalidTransitText occurs when UnmarshalText is called on Transit
	// with an invalid textual representation
	ErrInvalidTransitText = errors.New("invalid transit textual representation")
)

func (t *transit) MarshalText() (text []byte, err error) {
	id := []byte(t.ID)
	stepper, err := t.Stepper.MarshalText()
	if err != nil {
		return nil, err
	}

	return bytes.Join([][]byte{
		id,
		stepper,
	}, textMarshallerSep), nil
}

func (t *transit) UnmarshalText(text []byte) error {
	r := bytes.Split(text, textMarshallerSep)
	if len(r) < 2 {
		return ErrInvalidTransitText
	}
	t.ID = string(r[0])
	t.Stepper = newStepper()
	if err := t.Stepper.UnmarshalText(r[1]); err != nil {
		return ErrInvalidTransitText
	}
	return nil
}

type transitKey struct{}

var activeTransitKey = transitKey{}

// TransitFromContext extracts `Transit` from context and returns `nil` when
// no instance of `Transit` can be found
func TransitFromContext(ctx context.Context) Transit {
	val := ctx.Value(activeTransitKey)
	if o, ok := val.(Transit); ok {
		return o
	}
	return nil
}

// TransitWithContext injects `Transit` to context
func TransitWithContext(ctx context.Context, t Transit) context.Context {
	return context.WithValue(ctx, activeTransitKey, t)
}

// NewTransitWithContext injects a new `Transit` to context
func NewTransitWithContext(ctx context.Context) (context.Context, Transit) {
	tr := TransitFactory()
	return context.WithValue(ctx, activeTransitKey, tr), tr
}
