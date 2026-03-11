package httpsignature

import (
	"errors"
	"testing"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
	httpsignatureconfig "github.com/nimburion/nimburion/pkg/http/httpsignature/config"
)

func TestExtensionValidate_ReturnsAppError(t *testing.T) {
	ext := Extension{
		Security: httpsignatureconfig.SecurityConfig{
			HTTPSignature: httpsignatureconfig.Config{
				Enabled: true,
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
	if appErr.Code != "validation.security.http_signature.key_id_header.required" {
		t.Fatalf("Code = %q", appErr.Code)
	}
}
