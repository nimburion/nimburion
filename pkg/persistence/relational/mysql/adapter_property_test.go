package mysql

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Property 3: Close prevents subsequent operations
func TestProperty_ClosePreventsSubsequentOperations(t *testing.T) {
	params := gopter.DefaultTestParameters()
	params.MinSuccessfulTests = 25
	properties := gopter.NewProperties(params)

	properties.Property("exec after close always fails", prop.ForAll(
		func(query string) bool {
			db, _, err := sqlmock.New()
			if err != nil {
				return false
			}
			a := &Adapter{db: db, logger: &mockLogger{}}
			_ = a.Close()
			_, err = a.ExecContext(context.Background(), query)
			return err != nil
		},
		gen.AlphaString(),
	))

	properties.TestingRun(t)
}
