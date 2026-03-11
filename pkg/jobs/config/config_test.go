package config

import (
	"errors"
	"testing"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
)

func TestExtensionValidate_ReturnsAppError(t *testing.T) {
	ext := Extension{
		Jobs: Config{
			Backend:      "invalid",
			DefaultQueue: "default",
			Worker: WorkerConfig{
				Concurrency:    1,
				LeaseTTL:       30,
				ReserveTimeout: 1,
				StopTimeout:    10,
			},
			Retry: RetryConfig{
				MaxAttempts:    5,
				InitialBackoff: 1,
				MaxBackoff:     60,
				AttemptTimeout: 30,
			},
			DLQ: DLQConfig{
				Enabled:     true,
				QueueSuffix: ".dlq",
			},
		},
	}

	err := ext.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}

	var appErr *coreerrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != "validation.jobs.backend.invalid" {
		t.Fatalf("Code = %q", appErr.Code)
	}
}
