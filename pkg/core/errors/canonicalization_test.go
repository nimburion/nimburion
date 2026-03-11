package errors_test

import (
	stderrors "errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/nimburion/nimburion/pkg/cache"
	"github.com/nimburion/nimburion/pkg/coordination"
	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
	httpsession "github.com/nimburion/nimburion/pkg/http/session"
	"github.com/nimburion/nimburion/pkg/jobs"
	"github.com/nimburion/nimburion/pkg/scheduler"
	"github.com/nimburion/nimburion/pkg/session"
)

func TestAsAppErrorCanonicalizesRegisteredFamilies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantCode   string
		wantStatus int
	}{
		{name: "jobs validation", err: stderrors.Join(fmt.Errorf("context"), jobs.ErrValidation), wantCode: "validation.jobs", wantStatus: http.StatusBadRequest},
		{name: "scheduler conflict", err: scheduler.ErrConflict, wantCode: "scheduler.conflict", wantStatus: http.StatusConflict},
		{name: "coordination retryable", err: coordination.ErrRetryable, wantCode: coreerrors.CodeRetryable, wantStatus: http.StatusServiceUnavailable},
		{name: "cache miss", err: cache.ErrCacheMiss, wantCode: "cache.not_found", wantStatus: http.StatusNotFound},
		{name: "session missing", err: session.ErrNotFound, wantCode: "session.not_found", wantStatus: http.StatusNotFound},
		{name: "http session missing", err: httpsession.ErrNoSession, wantCode: coreerrors.CodeNotInitialized, wantStatus: http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			appErr, ok := coreerrors.AsAppError(tt.err)
			if !ok {
				t.Fatalf("expected canonical app error for %T", tt.err)
			}
			if appErr.Code != tt.wantCode {
				t.Fatalf("Code = %q, want %q", appErr.Code, tt.wantCode)
			}
			if appErr.HTTPStatus != tt.wantStatus {
				t.Fatalf("HTTPStatus = %d, want %d", appErr.HTTPStatus, tt.wantStatus)
			}
		})
	}
}
