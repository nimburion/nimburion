package sqs

import (
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Property 13: Batch publish chunking precondition helpers preserve attributes
func TestProperty_MessageAttributesRoundTrip(t *testing.T) {
	params := gopter.DefaultTestParameters()
	params.MinSuccessfulTests = 20
	properties := gopter.NewProperties(params)

	properties.Property("headers roundtrip through SQS attribute conversion", prop.ForAll(
		func(k, v string) bool {
			if k == "" {
				k = "k"
			}
			in := map[string]string{k: v}
			out := fromSQSAttributes(toSQSAttributes(in))
			return out[k] == v
		},
		gen.AlphaString(),
		gen.AlphaString(),
	))

	properties.TestingRun(t)
}
