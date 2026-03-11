package config

import (
	"errors"
	"testing"

	"github.com/spf13/viper"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
)

func TestExtensionValidate_ReturnsAppError(t *testing.T) {
	ext := Extension{}
	ext.SecurityHeaders.STSSeconds = -1

	err := ext.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}

	var appErr *coreerrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != "validation.security_headers.sts_seconds.invalid" {
		t.Fatalf("Code = %q", appErr.Code)
	}
}

func TestExtensionBindEnv_BindsExtendedSecurityHeaderFields(t *testing.T) {
	t.Setenv("APP_SECURITY_HEADERS_CUSTOM_FRAME_OPTIONS", "SAMEORIGIN")
	t.Setenv("APP_SECURITY_HEADERS_CONTENT_SECURITY_POLICY", "default-src 'none'")
	t.Setenv("APP_SECURITY_HEADERS_PERMISSIONS_POLICY", "camera=()")
	t.Setenv("APP_SECURITY_HEADERS_REFERRER_POLICY", "no-referrer")
	t.Setenv("APP_SECURITY_HEADERS_IE_NO_OPEN", "false")
	t.Setenv("APP_SECURITY_HEADERS_X_DNS_PREFETCH_CONTROL", "on")
	t.Setenv("APP_SECURITY_HEADERS_CROSS_ORIGIN_OPENER_POLICY", "unsafe-none")
	t.Setenv("APP_SECURITY_HEADERS_CROSS_ORIGIN_RESOURCE_POLICY", "cross-origin")
	t.Setenv("APP_SECURITY_HEADERS_CROSS_ORIGIN_EMBEDDER_POLICY", "require-corp")

	v := viper.New()
	ext := Extension{}
	ext.ApplyDefaults(v)

	if err := ext.BindEnv(v, "APP"); err != nil {
		t.Fatalf("BindEnv() error = %v", err)
	}

	if got := v.GetString("security_headers.custom_frame_options"); got != "SAMEORIGIN" {
		t.Fatalf("custom_frame_options = %q, want SAMEORIGIN", got)
	}
	if got := v.GetString("security_headers.content_security_policy"); got != "default-src 'none'" {
		t.Fatalf("content_security_policy = %q", got)
	}
	if got := v.GetString("security_headers.permissions_policy"); got != "camera=()" {
		t.Fatalf("permissions_policy = %q", got)
	}
	if got := v.GetString("security_headers.referrer_policy"); got != "no-referrer" {
		t.Fatalf("referrer_policy = %q", got)
	}
	if got := v.GetBool("security_headers.ie_no_open"); got {
		t.Fatalf("ie_no_open = %v, want false", got)
	}
	if got := v.GetString("security_headers.x_dns_prefetch_control"); got != "on" {
		t.Fatalf("x_dns_prefetch_control = %q", got)
	}
	if got := v.GetString("security_headers.cross_origin_opener_policy"); got != "unsafe-none" {
		t.Fatalf("cross_origin_opener_policy = %q", got)
	}
	if got := v.GetString("security_headers.cross_origin_resource_policy"); got != "cross-origin" {
		t.Fatalf("cross_origin_resource_policy = %q", got)
	}
	if got := v.GetString("security_headers.cross_origin_embedder_policy"); got != "require-corp" {
		t.Fatalf("cross_origin_embedder_policy = %q", got)
	}
}
