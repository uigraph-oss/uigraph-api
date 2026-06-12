package auth

import (
	"net/http"
	"time"

	"github.com/uigraph/app/internal/httputil"
	"github.com/uigraph/app/internal/org"
)

type InvitationHandler struct {
	store org.InvitationStore
}

func NewInvitationHandler(s org.InvitationStore) *InvitationHandler {
	return &InvitationHandler{store: s}
}

// ── Request / Response types ─────────────────────────────────────────────────

type invitationResponse struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"orgId"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	InvitedBy string    `json:"invitedBy,omitempty"`
	ExpiresAt time.Time `json:"expiresAt"`
	CreatedAt time.Time `json:"createdAt"`
}

func inviteToResponse(inv org.Invitation) invitationResponse {
	return invitationResponse{
		ID: inv.ID, OrgID: inv.OrgID, Email: inv.Email, Role: inv.Role,
		Status: inv.Status, InvitedBy: inv.InvitedBy,
		ExpiresAt: inv.ExpiresAt, CreatedAt: inv.CreatedAt,
	}
}

type createInvitationRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"` // admin | editor | viewer
}

// ── Handlers ─────────────────────────────────────────────────────────────────

// List returns all invitations for an org (pending, accepted, revoked).
// GET /api/v1/orgs/{orgID}/invitations
func (h *InvitationHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	invs, err := h.store.ListInvitations(r.Context(), orgID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	out := make([]invitationResponse, len(invs))
	for i, inv := range invs {
		out[i] = inviteToResponse(inv)
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"invitations": out})
}

// Create sends an invitation to a new or existing user.
// POST /api/v1/orgs/{orgID}/invitations
func (h *InvitationHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createInvitationRequest
	if err := httputil.Decode(r, &req); err != nil {
		httputil.BadRequest(w, "invalid JSON")
		return
	}
	if req.Email == "" || req.Role == "" {
		httputil.BadRequest(w, "email and role are required")
		return
	}
	// TODO: generate code, persist, enqueue email job
	httputil.NotImplemented(w)
}

// Revoke cancels a pending invitation.
// DELETE /api/v1/orgs/{orgID}/invitations/{inviteID}
func (h *InvitationHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	inviteID := r.PathValue("inviteID")
	if err := h.store.RevokeInvitation(r.Context(), inviteID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Resend re-sends the invitation email for a pending invitation.
// POST /api/v1/orgs/{orgID}/invitations/{inviteID}/resend
func (h *InvitationHandler) Resend(w http.ResponseWriter, r *http.Request) {
	// TODO: look up invitation, verify still pending, enqueue email job
	httputil.NotImplemented(w)
}
