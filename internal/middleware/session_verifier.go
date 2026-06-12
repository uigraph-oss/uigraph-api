package middleware

import (
	"context"

	"github.com/uigraph/app/internal/authz"
	"github.com/uigraph/app/internal/identity"
	"github.com/uigraph/app/internal/org"
)

// SessionVerifier implements BearerVerifier by looking up session tokens in the
// database. It is constructed in server.go and passed to the router.
type SessionVerifier struct {
	store  identity.SessionStore
	susers org.UserStore
}

// NewSessionVerifier returns a BearerVerifier backed by store.
func NewSessionVerifier(store identity.SessionStore, users org.UserStore) *SessionVerifier {
	return &SessionVerifier{store: store, susers: users}
}

// VerifyBearer looks up the token hash in user_sessions.
// Returns ErrForbidden when the session is missing, expired, or invalid.
func (v *SessionVerifier) VerifyBearer(token string) (identity.Principal, error) {
	hash := identity.Hash(token)
	sess, err := v.store.GetSessionByToken(context.Background(), hash)
	if err != nil {
		return identity.Principal{}, err
	}
	if sess == nil {
		return identity.Principal{}, authz.ErrForbidden
	}
	isServerAdmin := false
	u, err := v.susers.GetUser(context.Background(), sess.UserID)
	if err == nil && u != nil && u.Role == "server_admin" {
		isServerAdmin = true
	}
	return identity.Principal{
		Kind:          identity.PrincipalUser,
		UserID:        sess.UserID,
		IsServerAdmin: isServerAdmin,
		AuthProvider:  sess.AuthProvider,
	}, nil
}
