package migrate

import (
	"context"
	"errors"
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

func defaultOptions() Options {
	return Options{
		ServiceName: "test-service",
		Path:        "db/migrations",
		Logger:      testLogger{},
	}
}

func defaultOperations() Operations {
	return Operations{
		Up:   func(context.Context) (int, error) { return 1, nil },
		Down: func(context.Context, int) (int, error) { return 1, nil },
		Status: func(context.Context) (*Status, error) {
			return &Status{AppliedVersions: []int64{1}, Pending: []PendingMigration{}}, nil
		},
	}
}

func TestParseArgsDefaultsToUp(t *testing.T) {
	subcommand, steps, err := ParseArgs(nil)
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}
	if subcommand != "up" {
		t.Fatalf("expected up, got %q", subcommand)
	}
	if steps != 1 {
		t.Fatalf("expected steps 1, got %d", steps)
	}
}

func TestParseArgsInvalidSteps(t *testing.T) {
	_, _, err := ParseArgs([]string{"down", "bad"})
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestRunParsedInvalidCommandReturnsUsage(t *testing.T) {
	err := RunParsed("invalid", 1, defaultOptions(), defaultOperations())
	if err == nil {
		t.Fatal("expected usage error")
	}
}

func TestRunPropagatesOperationError(t *testing.T) {
	ops := defaultOperations()
	ops.Up = func(context.Context) (int, error) { return 0, errors.New("boom") }

	err := RunParsed("up", 1, defaultOptions(), ops)
	if err == nil || err.Error() != "boom" {
		t.Fatalf("expected boom error, got %v", err)
	}
}
