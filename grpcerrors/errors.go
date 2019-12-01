package grpcerrors

import (
	"context"

	"github.com/golang/protobuf/ptypes"
	"github.com/deixis/errors"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Unpack extracts a gRPC error
func Unpack(err error) error {
	status, ok := status.FromError(err)
	if !ok {
		return err
	}

	switch status.Code() {
	case codes.OK:
		return nil
	case codes.Canceled:
		return context.Canceled
	case codes.Unknown:
		return status.Err()
	case codes.InvalidArgument:
		for _, d := range status.Details() {
			failure, ok := d.(*errdetails.BadRequest)
			if !ok {
				continue
			}

			violations := make([]*errors.FieldViolation, len(failure.FieldViolations))
			for i, violation := range failure.FieldViolations {
				violations[i] = &errors.FieldViolation{
					Field:       violation.Field,
					Description: violation.Description,
				}
			}
			return errors.Bad(violations...)
		}
		return errors.Bad()
	case codes.DeadlineExceeded:
		return context.DeadlineExceeded
	case codes.NotFound:
		return errors.NotFound
	case codes.AlreadyExists:
		// TODO: Other error message?
		return errors.Aborted()
	case codes.PermissionDenied:
		return errors.PermissionDenied
	case codes.ResourceExhausted:
		for _, d := range status.Details() {
			failure, ok := d.(*errdetails.QuotaFailure)
			if !ok {
				continue
			}

			violations := make([]*errors.QuotaViolation, len(failure.Violations))
			for i, violation := range failure.Violations {
				violations[i] = &errors.QuotaViolation{
					Subject:     violation.Subject,
					Description: violation.Description,
				}
			}
			return errors.ResourceExhausted(violations...)
		}
		return errors.ResourceExhausted()
	case codes.FailedPrecondition:
		for _, d := range status.Details() {
			failure, ok := d.(*errdetails.PreconditionFailure)
			if !ok {
				continue
			}

			violations := make([]*errors.PreconditionViolation, len(failure.Violations))
			for i, violation := range failure.Violations {
				violations[i] = &errors.PreconditionViolation{
					Type:        violation.Type,
					Subject:     violation.Subject,
					Description: violation.Description,
				}
			}
			return errors.FailedPrecondition(violations...)
		}
		return errors.FailedPrecondition()
	case codes.Aborted:
		return errors.Aborted()
	case codes.OutOfRange:
		return status.Err()
	case codes.Unimplemented:
		return status.Err()
	case codes.Internal:
		return status.Err()
	case codes.Unavailable:
		for _, d := range status.Details() {
			info, ok := d.(*errdetails.RetryInfo)
			if !ok {
				continue
			}
			d, _ := ptypes.Duration(info.RetryDelay)
			return errors.Unavailable(d)
		}
		return errors.Unavailable(0)
	case codes.DataLoss:
		return status.Err()
	case codes.Unauthenticated:
		return errors.Unauthenticated
	default:
		return status.Err()
	}
}

// Pack returns a Status representing err if it was produced from an
// `*errors.Error` struct.
func Pack(err error) *status.Status {
	s, _ := pack(err)
	return s
}

// Pack returns a Status representing err if it was produced from an
// `*errors.Error` struct. Otherwise, ok is false and a Status is returned
// with codes.Unknown and the original error message.
func pack(err error) (*status.Status, bool) {
	if err == nil {
		return status.New(codes.OK, ""), true
	}

	switch err {
	case context.Canceled:
		return status.New(codes.Canceled, err.Error()), true
	case context.DeadlineExceeded:
		return status.New(codes.DeadlineExceeded, err.Error()), true
	}

	switch err := err.(type) {
	case *errors.AvailabilityFailure:
		s := status.New(codes.Unavailable, err.Error())
		detail := &errdetails.RetryInfo{
			RetryDelay: ptypes.DurationProto(err.RetryInfo.RetryDelay),
		}
		if s, err := s.WithDetails(detail); err == nil {
			return s, true
		}
		return s, true
	case *errors.PermissionFailure:
		return status.New(codes.PermissionDenied, err.Error()), true
	case *errors.AuthenticationFailure:
		return status.New(codes.Unauthenticated, err.Error()), true
	case *errors.MissingFailure:
		return status.New(codes.NotFound, err.Error()), true
	case *errors.BadRequest:
		s := status.New(codes.InvalidArgument, err.Error())
		detail := &errdetails.BadRequest{
			FieldViolations: make([]*errdetails.BadRequest_FieldViolation, len(err.Violations)),
		}
		for i, violation := range err.Violations {
			detail.FieldViolations[i] = &errdetails.BadRequest_FieldViolation{
				Field:       violation.Field,
				Description: violation.Description,
			}
		}
		if s, err := s.WithDetails(detail); err == nil {
			return s, true
		}
		return s, true
	case *errors.PreconditionFailure:
		s := status.New(codes.FailedPrecondition, err.Error())
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
		if s, err := s.WithDetails(detail); err == nil {
			return s, true
		}
		return s, true
	case *errors.ConflictFailure:
		return status.New(codes.Aborted, err.Error()), true
	case *errors.QuotaFailure:
		s := status.New(codes.ResourceExhausted, err.Error())
		detail := &errdetails.QuotaFailure{
			Violations: make([]*errdetails.QuotaFailure_Violation, len(err.Violations)),
		}
		for i, violation := range err.Violations {
			detail.Violations[i] = &errdetails.QuotaFailure_Violation{
				Subject:     violation.Subject,
				Description: violation.Description,
			}
		}
		if s, err := s.WithDetails(detail); err == nil {
			return s, true
		}
		return s, true
	default:
		return status.New(codes.Unknown, err.Error()), false
	}
}
