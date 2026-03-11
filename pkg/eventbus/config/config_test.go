package config

import (
	"errors"
	"testing"

	"github.com/spf13/viper"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
)

func TestExtensionValidate_ReturnsAppError(t *testing.T) {
	ext := Extension{EventBus: Config{Type: "invalid"}}

	err := ext.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}

	var appErr *coreerrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != "validation.eventbus.type.invalid" {
		t.Fatalf("Code = %q", appErr.Code)
	}
}

func TestExtensionBindEnv_BindsSQSTuningFields(t *testing.T) {
	t.Setenv("APP_EVENTBUS_WAIT_TIME_SECONDS", "11")
	t.Setenv("APP_EVENTBUS_MAX_MESSAGES", "7")
	t.Setenv("APP_EVENTBUS_VISIBILITY_TIMEOUT", "45")

	v := viper.New()
	ext := Extension{}
	ext.ApplyDefaults(v)

	if err := ext.BindEnv(v, "APP"); err != nil {
		t.Fatalf("BindEnv() error = %v", err)
	}

	if got := v.GetInt32("eventbus.wait_time_seconds"); got != 11 {
		t.Fatalf("wait_time_seconds = %d, want 11", got)
	}
	if got := v.GetInt32("eventbus.max_messages"); got != 7 {
		t.Fatalf("max_messages = %d, want 7", got)
	}
	if got := v.GetInt32("eventbus.visibility_timeout"); got != 45 {
		t.Fatalf("visibility_timeout = %d, want 45", got)
	}
}
