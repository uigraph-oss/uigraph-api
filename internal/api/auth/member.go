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
	teams   org.TeamStore
}

func NewMemberHandler(m org.MemberStore, u org.UserStore, t org.TeamStore) *MemberHandler {
	return &MemberHandler{members: m, users: u, teams: t}
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
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func memberToResponse(m org.OrgMember) memberResponse {
	return memberResponse{
		UserID: m.UserID, OrgID: m.OrgID, Role: m.Role, Source: m.Source,
		Email: m.Email, Name: m.Name, TeamID: m.TeamID,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

type addMemberRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"` // admin | editor | viewer
}

type updateMemberRequest struct {
	Name   string `json:"name"`
	Email  string `json:"email"`
	Role   string `json:"role"` // admin | editor | viewer
	TeamID string `json:"teamId"`
}

// ── Handlers ─────────────────────────────────────────────────────────────────

// List returns all members of an org with their roles.
// GET /api/v1/orgs/{orgID}/members
// @Summary  List
// @Tags     members
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/members [get]
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
// @Summary  Add
// @Tags     members
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/members [post]
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

// UpdateMember updates a member's name, email, org role, and team assignment.
// PUT /api/v1/orgs/{orgID}/members/{userID}
// @Summary  UpdateMember
// @Tags     members
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    userID  path  string  true  "userID"
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/members/{userID} [put]
func (h *MemberHandler) UpdateMember(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	userID := r.PathValue("userID")
	var req updateMemberRequest
	if err := httputil.Decode(r, &req); err != nil {
		httputil.BadRequest(w, "invalid JSON")
		return
	}
	if req.Name == "" || req.Email == "" || req.Role == "" {
		httputil.BadRequest(w, "name, email, and role are required")
		return
	}
	u, err := h.users.GetUser(r.Context(), userID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if u == nil {
		httputil.Error(w, r, store.ErrNotFound)
		return
	}
	if req.Email != u.Email {
		clash, err := h.users.GetUserByEmail(r.Context(), req.Email)
		if err != nil {
			httputil.Error(w, r, err)
			return
		}
		if clash != nil && clash.ID != userID {
			httputil.Error(w, r, store.ErrConflict)
			return
		}
	}
	u.Name = req.Name
	u.Email = req.Email
	u.Login = req.Email
	if err := h.users.UpdateUser(r.Context(), *u); err != nil {
		httputil.Error(w, r, err)
		return
	}
	if err := h.members.UpdateMemberRole(r.Context(), userID, orgID, req.Role, "manual"); err != nil {
		httputil.Error(w, r, err)
		return
	}
	if err := h.teams.RemoveUserFromOrgTeams(r.Context(), orgID, userID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	if req.TeamID != "" {
		if err := h.teams.AddTeamMember(r.Context(), org.TeamMember{
			TeamID: req.TeamID, UserID: userID, OrgID: orgID, Permission: "member",
		}); err != nil {
			httputil.Error(w, r, err)
			return
		}
	}
	m := org.OrgMember{
		UserID: userID, OrgID: orgID, Role: req.Role, Source: "manual",
		Email: u.Email, Name: u.Name,
	}
	if req.TeamID != "" {
		m.TeamID = &req.TeamID
	}
	httputil.JSON(w, http.StatusOK, memberToResponse(m))
}

// Remove revokes a user's membership.
// DELETE /api/v1/orgs/{orgID}/members/{userID}
// @Summary  Remove
// @Tags     members
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    userID  path  string  true  "userID"
// @Success  204  "No Content"
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/members/{userID} [delete]
func (h *MemberHandler) Remove(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	userID := r.PathValue("userID")
	if err := h.members.RemoveMember(r.Context(), userID, orgID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
