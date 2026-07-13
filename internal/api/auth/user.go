package auth

import (
	"context"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/uigraph/app/internal/asset"
	"github.com/uigraph/app/internal/cache"
	"github.com/uigraph/app/internal/httputil"
	"github.com/uigraph/app/internal/org"
	"github.com/uigraph/app/internal/store"
)

type UserHandler struct {
	store  org.UserStore
	cache  cache.Client    // may be nil
	assets *asset.Resolver // presigns avatar URLs; may be nil
}

func NewUserHandler(s org.UserStore, c cache.Client, assets *asset.Resolver) *UserHandler {
	return &UserHandler{store: s, cache: c, assets: assets}
}

// ── Request / Response types ─────────────────────────────────────────────────

type userResponse struct {
	ID         string     `json:"id"`
	Email      string     `json:"email"`
	Name       string     `json:"name"`
	Login      string     `json:"login"`
	Disabled   bool       `json:"disabled"`
	Role       string     `json:"role"`
	AvatarURL  string     `json:"avatarUrl,omitempty"`
	LastSeenAt *time.Time `json:"lastSeenAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

// avatarURL presigns the avatar asset id, returning "" when there is no avatar
// or no resolver configured.
func (h *UserHandler) avatarURL(ctx context.Context, assetID *string) string {
	if assetID == nil || *assetID == "" || h.assets == nil {
		return ""
	}
	u, err := h.assets.Resolve(ctx, *assetID)
	if err != nil {
		return ""
	}
	return u
}

func (h *UserHandler) userToResponse(ctx context.Context, u org.User) userResponse {
	return userResponse{
		ID: u.ID, Email: u.Email, Name: u.Name, Login: u.Login,
		Disabled: u.Disabled, Role: u.Role,
		AvatarURL:  h.avatarURL(ctx, u.AvatarAssetID),
		LastSeenAt: u.LastSeenAt,
		CreatedAt:  u.CreatedAt, UpdatedAt: u.UpdatedAt,
	}
}

type createUserRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
	Role     string `json:"role,omitempty"`
}

type updateUserRequest struct {
	Name     string `json:"name,omitempty"`
	Role     string `json:"role,omitempty"`
	Disabled *bool  `json:"disabled,omitempty"`
}

// ── Handlers ─────────────────────────────────────────────────────────────────

// List returns all users globally (server admin only).
// GET /api/v1/users
// @Summary  List
// @Tags     users
// @Security BearerAuth
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /users [get]
func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	users, err := h.store.ListAllUsers(r.Context())
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	out := make([]userResponse, len(users))
	for i, u := range users {
		out[i] = h.userToResponse(r.Context(), u)
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"users": out})
}

// Create creates a new user (server admin only).
// POST /api/v1/users
// @Summary  Create
// @Tags     users
// @Security BearerAuth
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /users [post]
func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := httputil.Decode(r, &req); err != nil {
		httputil.BadRequest(w, "invalid JSON")
		return
	}
	if req.Email == "" || req.Name == "" || req.Password == "" {
		httputil.BadRequest(w, "email, name, and password are required")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	role := req.Role
	if role == "" {
		role = "user"
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
	now := time.Now().UTC()
	u := org.User{
		ID:           newID(),
		Email:        req.Email,
		Name:         req.Name,
		Login:        req.Email,
		PasswordHash: string(hash),
		Role:         role,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := h.store.CreateUser(r.Context(), u); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, h.userToResponse(r.Context(), u))
}

// Get returns a single user by ID.
// GET /api/v1/users/{userID}
// @Summary  Get
// @Tags     users
// @Security BearerAuth
// @Param    userID  path  string  true  "userID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /users/{userID} [get]
func (h *UserHandler) Get(w http.ResponseWriter, r *http.Request) {
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
	httputil.JSON(w, http.StatusOK, h.userToResponse(r.Context(), *u))
}

// Update changes a user's name, role, or disabled status.
// PUT /api/v1/users/{userID}
// @Summary  Update
// @Tags     users
// @Security BearerAuth
// @Param    userID  path  string  true  "userID"
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /users/{userID} [put]
func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userID")
	var req updateUserRequest
	if err := httputil.Decode(r, &req); err != nil {
		httputil.BadRequest(w, "invalid JSON")
		return
	}
	u, err := h.store.GetUser(r.Context(), userID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if u == nil {
		httputil.Error(w, r, store.ErrNotFound)
		return
	}
	if req.Name != "" {
		u.Name = req.Name
	}
	if req.Role != "" {
		u.Role = req.Role
	}
	if req.Disabled != nil {
		u.Disabled = *req.Disabled
	}
	if err := h.store.UpdateUser(r.Context(), *u); err != nil {
		httputil.Error(w, r, err)
		return
	}
	if h.cache != nil {
		_ = h.cache.Del(r.Context(), cache.ActorKey(userID))
	}
	httputil.JSON(w, http.StatusOK, h.userToResponse(r.Context(), *u))
}

// Disable marks a user as disabled.
// DELETE /api/v1/users/{userID}
// @Summary  Disable
// @Tags     users
// @Security BearerAuth
// @Param    userID  path  string  true  "userID"
// @Success  204  "No Content"
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /users/{userID} [delete]
func (h *UserHandler) Disable(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userID")
	if err := h.store.DisableUser(r.Context(), userID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	if h.cache != nil {
		_ = h.cache.Del(r.Context(), cache.ActorKey(userID))
	}
	w.WriteHeader(http.StatusNoContent)
}
