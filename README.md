# errors pkg

Package errors is an advanced error handling library that can be used to propagate
errors across boundaries, such as gRPC, and HTTP.

```go
violations := []*errors.FieldViolation{
  {
    Field: "firstname",
    Description: "Field required",
  },
  {
    Field: "locality",
    Description: "Field required",
  },
}
err := errors.Bad(violations...)
```

## Failure types

  * PermissionFailure
  * AuthenticationFailure
  * MissingFailure
  * BadRequest
  * PreconditionFailure
  * ConflictFailure
  * AvailabilityFailure
  * QuotaFailure

## Wrapping

Just like with `pkg/errors` library, it is possible to wrap errors and thus
keep the root cause of a failure to simplify debugging.

```go
_, err := os.Stat(path)
if os.IsNotExist(err) {
  return errors.WithNotFound(err)
}

// Carry on...
```

## i18n

TODO

## Across the wire

### gRPC

```go
func (s *grpcServer) Denied(ctx context.Context, req *Request) (*Response, error) {
  return nil, grpcerrors.Pack(errors.PermissionDenied).Err()
}
```

This will return a `7 - codes.PermissionDenied` status.

### HTTP

```go
http.HandleFunc("/precondition", func(w http.ResponseWriter, r *http.Request) {
  err := errors.FailedPrecondition(&errors.PreconditionViolation{
    Type: "TOS",
    Subject: "foo.app",
    Description: "Terms of service not accepted",
  })
  httperrors.Marshal(r, w, err)
})
log.Fatal(http.ListenAndServe(":8080", nil))
```

This will return a `412 - http.StatusPreconditionFailed` status with the following payload (assuming the request expects a JSON response):

```json
{
  "error": {
    "message": "1 preconditions failed",
    "details": [
      {
        "type": "TOS",
        "subject": "foo.app",
        "description": "Terms of service not accepted"
      }
    ]
  }
}
```

## github.com/pkg/errors
This package wraps calls to the widely used `pkg/errors` library to simplify the import.

## TODO
 * Localised messages
 * Unpack HTTP errors
 * Custom HTTP encoder/decoder
# errors
