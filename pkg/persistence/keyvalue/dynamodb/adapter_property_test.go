package dynamodb

import (
	"context"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/prop"
)

// Property 3: Close prevents subsequent operations
func TestProperty_ClosePreventsHealthCheck(t *testing.T) {
	params := gopter.DefaultTestParameters()
	params.MinSuccessfulTests = 20
	properties := gopter.NewProperties(params)

	properties.Property("closed adapter always fails healthcheck", prop.ForAll(
		func() bool {
			a := &Adapter{closed: true, logger: &mockLogger{}}
			return a.HealthCheck(context.Background()) != nil
		},
	))

	properties.TestingRun(t)
}
