package tenant

import (
	"context"
	"errors"
	"strings"
)

type Identity struct {
	ID string
}

func (i Identity) Validate() error {
	if strings.TrimSpace(i.ID) == "" {
		return errors.New("tenant id is required")
	}
	return nil
}

type Context struct {
	Tenant  Identity
	Subject string
	Attrs   map[string]string
}

func (c Context) Validate() error {
	return c.Tenant.Validate()
}

type contextKey struct{}

func WithContext(ctx context.Context, tc Context) context.Context {
	return context.WithValue(ctx, contextKey{}, tc)
}

func FromContext(ctx context.Context) (Context, bool) {
	tc, ok := ctx.Value(contextKey{}).(Context)
	return tc, ok
}

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
