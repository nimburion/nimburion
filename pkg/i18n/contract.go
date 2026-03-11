package i18n

import (
	"sort"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
)

// Params carries dynamic values used to interpolate a localized message template.
type Params = coreerrors.Params

// AppMessage is the transport-agnostic contract for application messages.
// Deprecated: use pkg/core/errors.Message as the canonical application message contract.
type AppMessage = coreerrors.Message

// AppError aliases the canonical application error contract.
// Deprecated: use pkg/core/errors.AppError as the canonical application error type.
type AppError = coreerrors.AppError

// NewMessage creates an AppMessage with the given code and params.
// Deprecated: use pkg/core/errors.NewMessage.
func NewMessage(code string, params Params) AppMessage {
	return coreerrors.NewMessage(code, params)
}

// NewError creates an AppError with a stable message code.
// Deprecated: use pkg/core/errors.New.
func NewError(code string, params Params, cause error) *AppError {
	return coreerrors.New(code, params, cause)
}

// Translator resolves a message key into a localized text.
type Translator interface {
	T(key string, args ...interface{}) string
}

// CanonicalParams returns a deterministic list of keys.
// Useful for tests, logging, and telemetry.
func CanonicalParams(params Params) []string {
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
