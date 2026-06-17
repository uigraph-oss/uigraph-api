package auth

import (
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/uigraph/app/internal/cache"
	"github.com/uigraph/app/internal/httputil"
	"github.com/uigraph/app/internal/org"
	"github.com/uigraph/app/internal/store"
)

type UserHandler struct {
	store org.UserStore
	cache cache.Client // may be nil
}

func NewUserHandler(s org.UserStore, c cache.Client) *UserHandler {
	return &UserHandler{store: s, cache: c}
}

// ── Request / Response types ─────────────────────────────────────────────────

type userResponse struct {
	ID         string     `json:"id"`
	Email      string     `json:"email"`
	Name       string     `json:"name"`
	Login      string     `json:"login"`
	Disabled   bool       `json:"disabled"`
	Role       string     `json:"role"`
	LastSeenAt *time.Time `json:"lastSeenAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

func userToResponse(u org.User) userResponse {
	return userResponse{
		ID: u.ID, Email: u.Email, Name: u.Name, Login: u.Login,
		Disabled: u.Disabled, Role: u.Role,
		LastSeenAt: u.LastSeenAt,
		CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt,
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
func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	users, err := h.store.ListAllUsers(r.Context())
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	out := make([]userResponse, len(users))
	for i, u := range users {
		out[i] = userToResponse(u)
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"users": out})
}

// Create creates a new user (server admin only).
// POST /api/v1/users
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
	u := org.User{
		ID:           newID(),
		Email:        req.Email,
		Name:         req.Name,
		Login:        req.Email,
		PasswordHash: string(hash),
		Role:         role,
	}
	if err := h.store.CreateUser(r.Context(), u); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, userToResponse(u))
}

// Get returns a single user by ID.
// GET /api/v1/users/{userID}
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
	httputil.JSON(w, http.StatusOK, userToResponse(*u))
}

// Update changes a user's name, role, or disabled status.
// PUT /api/v1/users/{userID}
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
	httputil.JSON(w, http.StatusOK, userToResponse(*u))
}

// Disable marks a user as disabled.
// DELETE /api/v1/users/{userID}
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
