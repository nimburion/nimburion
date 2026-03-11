package status

import (
	"testing"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
	grpcvalidation "github.com/nimburion/nimburion/pkg/grpc/validation"
	"github.com/nimburion/nimburion/pkg/jobs"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func TestCodeMapsValidationLayers(t *testing.T) {
	if got := Code(grpcvalidation.NewError(grpcvalidation.LayerContract, "contract failed", nil)); got != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition, got %s", got)
	}
}

func TestErrorMapsAppError(t *testing.T) {
	err := coreerrors.New("resource.not_found", nil, nil).WithMessage("missing").WithHTTPStatus(404)

	got := Error(err)
	status, ok := grpcstatus.FromError(got)
	if !ok {
		t.Fatalf("expected grpc status error")
	}
	if status.Code() != codes.NotFound {
		t.Fatalf("expected NotFound, got %s", status.Code())
	}
	if status.Message() != "missing" {
		t.Fatalf("expected safe fallback message, got %q", status.Message())
	}
}

func TestErrorMapsFamilySentinel(t *testing.T) {
	got := Error(jobs.ErrConflict)
	status, ok := grpcstatus.FromError(got)
	if !ok {
		t.Fatalf("expected grpc status error")
	}
	if status.Code() != codes.AlreadyExists {
		t.Fatalf("expected AlreadyExists, got %s", status.Code())
	}
	if status.Message() != jobs.ErrConflict.Error() {
		t.Fatalf("expected jobs conflict message, got %q", status.Message())
	}
}
