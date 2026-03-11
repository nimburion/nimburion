// Package tenant provides tenant context helpers and contracts.
package tenant

import (
	"context"
	"errors"
	"strings"
)

// Identity identifies a tenant.
type Identity struct {
	ID string
}

// Validate checks that the tenant identity is usable.
func (i Identity) Validate() error {
	if strings.TrimSpace(i.ID) == "" {
		return errors.New("tenant id is required")
	}
	return nil
}

// Context carries tenant-scoped identity and metadata.
type Context struct {
	Tenant  Identity
	Subject string
	Attrs   map[string]string
}

// Validate checks that the tenant context contains a valid tenant identity.
func (c Context) Validate() error {
	return c.Tenant.Validate()
}

type contextKey struct{}

// WithContext stores tc in ctx.
func WithContext(ctx context.Context, tc Context) context.Context {
	return context.WithValue(ctx, contextKey{}, tc)
}

// FromContext extracts tenant context from ctx.
func FromContext(ctx context.Context) (Context, bool) {
	tc, ok := ctx.Value(contextKey{}).(Context)
	return tc, ok
}

// MustFromContext extracts and validates tenant context from ctx.
func MustFromContext(ctx context.Context) (Context, error) {
	tc, ok := FromContext(ctx)
	if !ok {
		return Context{}, errors.New("tenant context not found")
	}
	if err := tc.Validate(); err != nil {
		return Context{}, err
	}
	return tc, nil
}
