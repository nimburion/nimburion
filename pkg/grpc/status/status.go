package status

import (
	"errors"

	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
	grpcvalidation "github.com/nimburion/nimburion/pkg/grpc/validation"
)

// Code returns the gRPC code for one framework error.
func Code(err error) codes.Code {
	if err == nil {
		return codes.OK
	}

	var validationErr *grpcvalidation.Error
	if errors.As(err, &validationErr) {
		switch validationErr.Layer {
		case grpcvalidation.LayerTransport:
			return codes.InvalidArgument
		case grpcvalidation.LayerContract:
			return codes.FailedPrecondition
		case grpcvalidation.LayerDomain:
			return codes.InvalidArgument
		}
	}

	if appErr, ok := coreerrors.AsAppError(err); ok {
		switch appErr.HTTPStatus {
		case 400:
			return codes.InvalidArgument
		case 401:
			return codes.Unauthenticated
		case 403:
			return codes.PermissionDenied
		case 404:
			return codes.NotFound
		case 409:
			return codes.AlreadyExists
		case 429:
			return codes.ResourceExhausted
		case 501:
			return codes.Unimplemented
		case 503:
			return codes.Unavailable
		default:
			if appErr.Code == "validation.failed" {
				return codes.InvalidArgument
			}
		}
	}

	return codes.Internal
}

// Error converts one framework error into a gRPC status error.
func Error(err error) error {
	if err == nil {
		return nil
	}
	return grpcstatus.Error(Code(err), SafeMessage(err))
}

// SafeMessage returns a transport-safe message for one framework error.
func SafeMessage(err error) string {
	if err == nil {
		return ""
	}

	var validationErr *grpcvalidation.Error
	if errors.As(err, &validationErr) && validationErr.Message != "" {
		return validationErr.Message
	}

	if appErr, ok := coreerrors.AsAppError(err); ok {
		if appErr.FallbackMessage != "" {
			return appErr.FallbackMessage
		}
		if appErr.Code != "" {
			return appErr.Code
		}
	}
	return "internal error"
}
