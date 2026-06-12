package middleware

import (
	"context"

	"github.com/uigraph/app/internal/identity"
)

// principalKey is an unexported type that prevents collisions with other
// context keys set by third-party middleware.
type principalKey struct{}

// WithPrincipal returns a child context carrying p.
func WithPrincipal(ctx context.Context, p identity.Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFromCtx extracts the Principal injected by the auth middleware.
// Returns (Principal{}, false) when no principal has been set.
func PrincipalFromCtx(ctx context.Context) (identity.Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(identity.Principal)
	return p, ok
}
