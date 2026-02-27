package mongodb

import (
	"context"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/prop"
)

// Property 3: Close prevents subsequent operations
func TestProperty_ClosePreventsPing(t *testing.T) {
	params := gopter.DefaultTestParameters()
	params.MinSuccessfulTests = 20
	properties := gopter.NewProperties(params)

	properties.Property("closed adapter always fails ping", prop.ForAll(
		func() bool {
			a := &Adapter{closed: true}
			return a.Ping(context.Background()) != nil
		},
	))

	properties.TestingRun(t)
}
