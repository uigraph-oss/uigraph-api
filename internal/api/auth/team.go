package auth

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/uigraph/app/internal/httputil"
	"github.com/uigraph/app/internal/org"
	"github.com/uigraph/app/internal/store"
)

type TeamHandler struct {
	store org.TeamStore
}

func NewTeamHandler(s org.TeamStore) *TeamHandler {
	return &TeamHandler{store: s}
}

// ── Request / Response types ─────────────────────────────────────────────────

type teamResponse struct {
	ID         string    `json:"id"`
	OrgID      string    `json:"orgId"`
	Name       string    `json:"name"`
	Email      string    `json:"email,omitempty"`
	ExternalID string    `json:"externalId,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func teamToResponse(t org.Team) teamResponse {
	return teamResponse{
		ID: t.ID, OrgID: t.OrgID, Name: t.Name, Email: t.Email,
		ExternalID: t.ExternalID, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}
}

type teamMemberResponse struct {
	TeamID     string    `json:"teamId"`
	UserID     string    `json:"userId"`
	Permission string    `json:"permission"`
	CreatedAt  time.Time `json:"createdAt"`
}

type createTeamRequest struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
}

type updateTeamRequest struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
}

type addTeamMemberRequest struct {
	UserID     string `json:"userId"`
	Permission string `json:"permission"` // member | admin
}

// ── Handlers ─────────────────────────────────────────────────────────────────

// List returns all teams in an org.
// GET /api/v1/orgs/{orgID}/teams
func (h *TeamHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	teams, err := h.store.ListTeams(r.Context(), orgID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	out := make([]teamResponse, len(teams))
	for i, t := range teams {
		out[i] = teamToResponse(t)
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"teams": out})
}

// Create adds a new team to an org.
// POST /api/v1/orgs/{orgID}/teams
func (h *TeamHandler) Create(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	var req createTeamRequest
	if err := httputil.Decode(r, &req); err != nil {
		httputil.BadRequest(w, "invalid JSON")
		return
	}
	if req.Name == "" {
		httputil.BadRequest(w, "name is required")
		return
	}
	t := org.Team{
		ID:    uuid.NewString(),
		OrgID: orgID,
		Name:  req.Name,
		Email: req.Email,
	}
	if err := h.store.CreateTeam(r.Context(), t); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, teamToResponse(t))
}

// Get returns a single team.
// GET /api/v1/orgs/{orgID}/teams/{teamID}
func (h *TeamHandler) Get(w http.ResponseWriter, r *http.Request) {
	teamID := r.PathValue("teamID")
	t, err := h.store.GetTeam(r.Context(), teamID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if t == nil {
		httputil.Error(w, r, store.ErrNotFound)
		return
	}
	httputil.JSON(w, http.StatusOK, teamToResponse(*t))
}

// Update changes a team's name or email.
// PUT /api/v1/orgs/{orgID}/teams/{teamID}
func (h *TeamHandler) Update(w http.ResponseWriter, r *http.Request) {
	teamID := r.PathValue("teamID")
	var req updateTeamRequest
	if err := httputil.Decode(r, &req); err != nil {
		httputil.BadRequest(w, "invalid JSON")
		return
	}
	if req.Name == "" {
		httputil.BadRequest(w, "name is required")
		return
	}
	t, err := h.store.GetTeam(r.Context(), teamID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if t == nil {
		httputil.Error(w, r, store.ErrNotFound)
		return
	}
	t.Name = req.Name
	t.Email = req.Email
	if err := h.store.UpdateTeam(r.Context(), *t); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, teamToResponse(*t))
}

// Delete removes a team and its memberships.
// DELETE /api/v1/orgs/{orgID}/teams/{teamID}
func (h *TeamHandler) Delete(w http.ResponseWriter, r *http.Request) {
	teamID := r.PathValue("teamID")
	if err := h.store.DeleteTeam(r.Context(), teamID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListMembers returns all members of a team.
// GET /api/v1/orgs/{orgID}/teams/{teamID}/members
func (h *TeamHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	teamID := r.PathValue("teamID")
	members, err := h.store.ListTeamMembers(r.Context(), teamID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	out := make([]teamMemberResponse, len(members))
	for i, m := range members {
		out[i] = teamMemberResponse{
			TeamID: m.TeamID, UserID: m.UserID,
			Permission: m.Permission, CreatedAt: m.CreatedAt,
		}
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"members": out})
}

// AddMember adds a user to a team.
// POST /api/v1/orgs/{orgID}/teams/{teamID}/members
func (h *TeamHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	teamID := r.PathValue("teamID")
	var req addTeamMemberRequest
	if err := httputil.Decode(r, &req); err != nil {
		httputil.BadRequest(w, "invalid JSON")
		return
	}
	if req.UserID == "" {
		httputil.BadRequest(w, "userId is required")
		return
	}
	perm := req.Permission
	if perm == "" {
		perm = "member"
	}
	err := h.store.AddTeamMember(r.Context(), org.TeamMember{
		TeamID: teamID, UserID: req.UserID, OrgID: orgID, Permission: perm,
	})
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

// RemoveMember removes a user from a team.
// DELETE /api/v1/orgs/{orgID}/teams/{teamID}/members/{userID}
func (h *TeamHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	teamID := r.PathValue("teamID")
	userID := r.PathValue("userID")
	if err := h.store.RemoveTeamMember(r.Context(), teamID, userID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
