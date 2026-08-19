package auth

import (
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/uigraph/app/internal/httputil"
	"github.com/uigraph/app/internal/identity"
	"github.com/uigraph/app/internal/org"
	"github.com/uigraph/app/internal/store"
)

// SignupHandler backs the internal, enterprise-only account-provisioning
// endpoint. It has no public HTTP surface of its own — the separate,
// private uigraph-enterprise service owns the actual signup UX (form,
// validation, rate limiting) and calls this to do the actual user/org
// creation, then sets the browser's session cookie itself using the token
// this returns. Self-hosted deployments never register this handler at all.
type SignupHandler struct {
	store         sessionStore
	internalToken string
}

func NewSignupHandler(s sessionStore, internalToken string) *SignupHandler {
	return &SignupHandler{store: s, internalToken: internalToken}
}

type provisionAccountRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
	OrgName  string `json:"orgName"`
}

type provisionAccountResponse struct {
	SessionToken string    `json:"sessionToken"`
	ExpiresAt    time.Time `json:"expiresAt"`
	UserID       string    `json:"userId"`
	OrgID        string    `json:"orgId"`
}

// ProvisionAccount creates a user, an org, and an admin membership for that
// user in one shot (the same sequence OrgHandler.Create runs for an
// already-logged-in user, prepended with user creation), then mints a
// session token for it. Authenticated via a shared X-Internal-Token header,
// not the normal session/API-key middleware — there is no session yet.
// POST /internal/v1/accounts
func (h *SignupHandler) ProvisionAccount(w http.ResponseWriter, r *http.Request) {
	if h.internalToken == "" || r.Header.Get("X-Internal-Token") != h.internalToken {
		httputil.Unauthorized(w)
		return
	}

	var req provisionAccountRequest
	if err := httputil.Decode(r, &req); err != nil {
		httputil.BadRequest(w, "invalid JSON")
		return
	}
	if req.Email == "" || req.Password == "" || req.Name == "" || req.OrgName == "" {
		httputil.BadRequest(w, "email, password, name, and orgName are required")
		return
	}

	existing, err := h.store.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if existing != nil {
		httputil.Error(w, r, store.ErrConflict)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	u := org.User{
		ID:           newID(),
		Email:        req.Email,
		Name:         req.Name,
		Login:        req.Email,
		PasswordHash: string(hash),
		Role:         "user",
	}
	if err := h.store.CreateUser(r.Context(), u); err != nil {
		httputil.Error(w, r, err)
		return
	}

	o := org.Org{ID: newUUID(), Name: req.OrgName}
	if err := h.store.CreateOrg(r.Context(), o); err != nil {
		httputil.Error(w, r, err)
		return
	}
	if err := h.store.AddMember(r.Context(), org.OrgMember{
		UserID: u.ID, OrgID: o.ID, Role: "admin", Source: "manual",
	}); err != nil {
		httputil.Error(w, r, err)
		return
	}
	sa := identity.NewSystemServiceAccount(o.ID, allScopeStrings())
	if err := h.store.CreateServiceAccount(r.Context(), sa); err != nil {
		httputil.Error(w, r, err)
		return
	}

	plaintext, tokenHash, err := generateSessionToken()
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	sess := identity.Session{
		ID:           newID(),
		UserID:       u.ID,
		TokenHash:    tokenHash,
		UserAgent:    r.UserAgent(),
		ClientIP:     r.RemoteAddr,
		CreatedAt:    time.Now().UTC(),
		ExpiresAt:    time.Now().UTC().Add(30 * 24 * time.Hour),
		LastActiveAt: time.Now().UTC(),
		RotatedAt:    time.Now().UTC(),
		AuthProvider: "password",
	}
	if err := h.store.CreateSession(r.Context(), sess); err != nil {
		httputil.Error(w, r, err)
		return
	}

	httputil.JSON(w, http.StatusCreated, provisionAccountResponse{
		SessionToken: plaintext,
		ExpiresAt:    sess.ExpiresAt,
		UserID:       u.ID,
		OrgID:        o.ID,
	})
}

type accountLookupResponse struct {
	UserID string `json:"userId"`
	Name   string `json:"name"`
	Email  string `json:"email"`
}

// LookupAccountByEmail backs uigraph-enterprise's self-service
// forgot-password flow: it needs to know whether an email has an account
// (and that account's id/name) before creating a reset token and sending
// mail, without exposing that lookup publicly itself. 404 (not an error
// body worth distinguishing further) when there's no match — the caller
// treats "not found" and "lookup succeeded, nothing to do" the same way.
// GET /internal/v1/accounts/lookup
func (h *SignupHandler) LookupAccountByEmail(w http.ResponseWriter, r *http.Request) {
	if h.internalToken == "" || r.Header.Get("X-Internal-Token") != h.internalToken {
		httputil.Unauthorized(w)
		return
	}

	email := r.URL.Query().Get("email")
	if email == "" {
		httputil.BadRequest(w, "email is required")
		return
	}

	u, err := h.store.GetUserByEmail(r.Context(), email)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if u == nil {
		httputil.Error(w, r, store.ErrNotFound)
		return
	}

	httputil.JSON(w, http.StatusOK, accountLookupResponse{UserID: u.ID, Name: u.Name, Email: u.Email})
}

type setAccountPasswordRequest struct {
	Password string `json:"password"`
}

// SetAccountPassword sets a user's password directly, by id. Called by
// uigraph-enterprise only after it has already validated and consumed its
// own password-reset token — by the time this runs, the request is already
// established as legitimate, so there's no token here, just the new value.
// POST /internal/v1/accounts/{userID}/password
func (h *SignupHandler) SetAccountPassword(w http.ResponseWriter, r *http.Request) {
	if h.internalToken == "" || r.Header.Get("X-Internal-Token") != h.internalToken {
		httputil.Unauthorized(w)
		return
	}

	var req setAccountPasswordRequest
	if err := httputil.Decode(r, &req); err != nil {
		httputil.BadRequest(w, "invalid JSON")
		return
	}
	if len(req.Password) < 8 {
		httputil.BadRequest(w, "password must be at least 8 characters")
		return
	}

	userID := r.PathValue("userID")
	u, err := h.store.GetUser(r.Context(), userID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if u == nil {
		httputil.Error(w, r, store.ErrNotFound)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	u.PasswordHash = string(hash)
	u.MustChangePassword = false // the user just chose this password themselves
	if err := h.store.UpdateUser(r.Context(), *u); err != nil {
		httputil.Error(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
