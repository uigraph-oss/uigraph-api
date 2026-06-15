// Package middleware provides net/http middleware for authenticating requests via
// Bearer tokens (SSO/OIDC users) or X-API-Key headers (service account tokens).
//
// On success the authenticated Principal is injected into the request context
// and the next handler is called. On failure a JSON 401 is written.
package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/uigraph/app/internal/authz"
	"github.com/uigraph/app/internal/identity"
)

// BearerVerifier validates a raw Bearer token and returns the principal.
// Implementations handle JWT verification and session lookup for SSO users.
// Return (Principal{}, authz.ErrForbidden) for invalid or expired tokens.
type BearerVerifier interface {
	VerifyBearer(token string) (identity.Principal, error)
}

// Middleware authenticates incoming HTTP requests.
type Middleware struct {
	bearer  BearerVerifier
	saStore identity.ServiceAccountStore
}

// New returns a Middleware. Either bearer or saStore may be nil if that
// authentication method is not required, but at least one must be non-nil.
func New(bearer BearerVerifier, saStore identity.ServiceAccountStore) *Middleware {
	return &Middleware{bearer: bearer, saStore: saStore}
}

// Handler wraps next, authenticating every request before calling it.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, err := m.authenticate(r)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, errorBody{
				Error: "unauthenticated",
				Code:  http.StatusUnauthorized,
			})
			return
		}
		next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), principal)))
	})
}

// sessionCookie is the HttpOnly cookie carrying the session token for browser
// clients (set by the OAuth callback and password login).
const sessionCookie = "uigraph_session"

// authenticate tries Bearer first, then the session cookie, then X-API-Key.
func (m *Middleware) authenticate(r *http.Request) (identity.Principal, error) {
	if bearer := extractBearer(r); bearer != "" && m.bearer != nil {
		return m.bearer.VerifyBearer(bearer)
	}
	if m.bearer != nil {
		if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
			return m.bearer.VerifyBearer(c.Value)
		}
	}
	if key := r.Header.Get("X-API-Key"); key != "" && m.saStore != nil {
		return m.verifyAPIKey(r, key)
	}
	return identity.Principal{}, authz.ErrForbidden
}

func (m *Middleware) verifyAPIKey(r *http.Request, plaintext string) (identity.Principal, error) {
	prefix := identity.Prefix(plaintext)
	tok, err := m.saStore.GetTokenByPrefix(r.Context(), prefix)
	if err != nil {
		return identity.Principal{}, err
	}
	if tok == nil || tok.Revoked {
		return identity.Principal{}, authz.ErrForbidden
	}
	if tok.ExpiresAt != nil && tok.ExpiresAt.Before(time.Now()) {
		return identity.Principal{}, authz.ErrForbidden
	}
	if !identity.Verify(plaintext, tok.Hash) {
		return identity.Principal{}, authz.ErrForbidden
	}

	// Best-effort: update last_used_at without blocking the request.
	_ = m.saStore.TouchToken(r.Context(), tok.ID)

	sa, err := m.saStore.GetServiceAccount(r.Context(), tok.ServiceAccountID)
	if err != nil || sa == nil || sa.Disabled {
		return identity.Principal{}, authz.ErrForbidden
	}

	return identity.Principal{
		Kind:             identity.PrincipalServiceAccount,
		UserID:           sa.ID,
		OrgID:            sa.OrgID,
		ServiceAccountID: sa.ID,
		Scopes:           sa.Scopes,
	}, nil
}

// extractBearer parses "Authorization: Bearer <token>".
func extractBearer(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

type errorBody struct {
	Error string `json:"error"`
	Code  int    `json:"code"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
