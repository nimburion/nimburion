package rabbitmq

import (
	"context"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/nimburion/nimburion/pkg/eventbus"
)

// Property 11: Close prevents subsequent operations
func TestProperty_ClosePreventsPublish(t *testing.T) {
	params := gopter.DefaultTestParameters()
	params.MinSuccessfulTests = 20
	properties := gopter.NewProperties(params)

	properties.Property("closed adapter always rejects publish", prop.ForAll(
		func(body string) bool {
			a := &Adapter{closed: true, subs: map[string]*subscription{}}
			msg := &eventbus.Message{ID: "id", Value: []byte(body), Timestamp: time.Now()}
			return a.Publish(context.Background(), "topic", msg) != nil
		},
		gen.AlphaString(),
	))

	properties.TestingRun(t)
}
