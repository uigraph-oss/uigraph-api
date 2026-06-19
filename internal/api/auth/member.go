package auth

import (
	"net/http"
	"time"

	"github.com/uigraph/app/internal/httputil"
	"github.com/uigraph/app/internal/org"
)

type MemberHandler struct {
	store org.MemberStore
}

func NewMemberHandler(s org.MemberStore) *MemberHandler {
	return &MemberHandler{store: s}
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
	UserID string `json:"userId"`
	Role   string `json:"role"` // admin | editor | viewer
}

type updateMemberRoleRequest struct {
	Role string `json:"role"`
}

// ── Handlers ─────────────────────────────────────────────────────────────────

// List returns all members of an org with their roles.
// GET /api/v1/orgs/{orgID}/members
func (h *MemberHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	members, err := h.store.ListMembers(r.Context(), orgID)
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

// Add grants an existing user membership in an org.
// POST /api/v1/orgs/{orgID}/members
func (h *MemberHandler) Add(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	var req addMemberRequest
	if err := httputil.Decode(r, &req); err != nil {
		httputil.BadRequest(w, "invalid JSON")
		return
	}
	if req.UserID == "" || req.Role == "" {
		httputil.BadRequest(w, "userId and role are required")
		return
	}
	err := h.store.AddMember(r.Context(), org.OrgMember{
		UserID: req.UserID, OrgID: orgID, Role: req.Role, Source: "manual",
	})
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
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
	if err := h.store.UpdateMemberRole(r.Context(), userID, orgID, req.Role, "manual"); err != nil {
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
	if err := h.store.RemoveMember(r.Context(), userID, orgID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
