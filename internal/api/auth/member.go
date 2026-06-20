package auth

import (
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/uigraph/app/internal/httputil"
	"github.com/uigraph/app/internal/org"
	"github.com/uigraph/app/internal/store"
)

type MemberHandler struct {
	members org.MemberStore
	users   org.UserStore
}

func NewMemberHandler(m org.MemberStore, u org.UserStore) *MemberHandler {
	return &MemberHandler{members: m, users: u}
}

// ── Request / Response types ─────────────────────────────────────────────────

type memberResponse struct {
	UserID    string    `json:"userId"`
	OrgID     string    `json:"orgId"`
	Role      string    `json:"role"`
	Source    string    `json:"source"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	TeamID    *string   `json:"teamId,omitempty"`
	TeamName  *string   `json:"teamName,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func memberToResponse(m org.OrgMember) memberResponse {
	return memberResponse{
		UserID: m.UserID, OrgID: m.OrgID, Role: m.Role, Source: m.Source,
		Email: m.Email, Name: m.Name, TeamID: m.TeamID, TeamName: m.TeamName,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

type addMemberRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"` // admin | editor | viewer
}

type updateMemberRoleRequest struct {
	Role string `json:"role"`
}

// ── Handlers ─────────────────────────────────────────────────────────────────

// List returns all members of an org with their roles.
// GET /api/v1/orgs/{orgID}/members
func (h *MemberHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	members, err := h.members.ListMembers(r.Context(), orgID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	out := make([]memberResponse, len(members))
	for i, m := range members {
		out[i] = memberToResponse(m)
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"members": out})
}

// Add creates a new user and grants them membership in an org.
// POST /api/v1/orgs/{orgID}/members
func (h *MemberHandler) Add(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	var req addMemberRequest
	if err := httputil.Decode(r, &req); err != nil {
		httputil.BadRequest(w, "invalid JSON")
		return
	}
	if req.Name == "" || req.Email == "" || req.Password == "" || req.Role == "" {
		httputil.BadRequest(w, "name, email, password, and role are required")
		return
	}
	existing, err := h.users.GetUserByEmail(r.Context(), req.Email)
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
	if err := h.users.CreateUser(r.Context(), u); err != nil {
		httputil.Error(w, r, err)
		return
	}
	m := org.OrgMember{UserID: u.ID, OrgID: orgID, Role: req.Role, Source: "manual"}
	if err := h.members.AddMember(r.Context(), m); err != nil {
		httputil.Error(w, r, err)
		return
	}
	m.Email = u.Email
	m.Name = u.Name
	httputil.JSON(w, http.StatusCreated, memberToResponse(m))
}

// UpdateRole changes a member's org-level role.
// PUT /api/v1/orgs/{orgID}/members/{userID}
func (h *MemberHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	userID := r.PathValue("userID")
	var req updateMemberRoleRequest
	if err := httputil.Decode(r, &req); err != nil {
		httputil.BadRequest(w, "invalid JSON")
		return
	}
	if req.Role == "" {
		httputil.BadRequest(w, "role is required")
		return
	}
	if err := h.members.UpdateMemberRole(r.Context(), userID, orgID, req.Role, "manual"); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Remove revokes a user's membership.
// DELETE /api/v1/orgs/{orgID}/members/{userID}
func (h *MemberHandler) Remove(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	userID := r.PathValue("userID")
	if err := h.members.RemoveMember(r.Context(), userID, orgID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
