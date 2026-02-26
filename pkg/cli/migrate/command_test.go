package migrate

import (
	"context"
	"testing"

	"github.com/nimburion/nimburion/pkg/observability/logger"
)

type testLogger struct{}

func (testLogger) Debug(string, ...any)                      {}
func (testLogger) Info(string, ...any)                       {}
func (testLogger) Warn(string, ...any)                       {}
func (testLogger) Error(string, ...any)                      {}
func (testLogger) With(...any) logger.Logger                 { return testLogger{} }
func (testLogger) WithContext(context.Context) logger.Logger { return testLogger{} }

func TestParseArgs(t *testing.T) {
	subcommand, steps, err := ParseArgs([]string{"up"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if subcommand != "up" || steps != 1 {
		t.Fatalf("unexpected result: %s, %d", subcommand, steps)
	}
}

func TestRun(t *testing.T) {
	ops := Operations{
		Up:   func(context.Context) (int, error) { return 0, nil },
		Down: func(context.Context, int) (int, error) { return 0, nil },
		Status: func(context.Context) (*Status, error) {
			return &Status{}, nil
		},
	}
	opts := Options{
		ServiceName: "test",
		Path:        "migrations",
		Logger:      testLogger{},
	}
	err := Run([]string{"up"}, opts, ops)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
