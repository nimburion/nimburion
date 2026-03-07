package cache

import (
	"context"

	"github.com/nimburion/nimburion/pkg/observability/logger"
)

type noopLogger struct{}

func (noopLogger) Debug(string, ...any)        {}
func (noopLogger) Info(string, ...any)         {}
func (noopLogger) Warn(string, ...any)         {}
func (noopLogger) Error(string, ...any)        {}
func (n noopLogger) With(...any) logger.Logger { return n }
func (n noopLogger) WithContext(context.Context) logger.Logger {
	return n
}
