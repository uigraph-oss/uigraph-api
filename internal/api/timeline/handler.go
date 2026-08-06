// Package timeline exposes REST endpoints for a service's Timeline events
// (releases, decisions, incidents), backed by internal/timeline.
package timeline

import (
	"errors"
	"net/http"
	"time"

	"github.com/uigraph/app/internal/httputil"
	"github.com/uigraph/app/internal/identity"
	authmw "github.com/uigraph/app/internal/middleware"
	"github.com/uigraph/app/internal/timeline"
)

type Handler struct {
	store timeline.Store
}

func New(s timeline.Store) *Handler {
	return &Handler{store: s}
}

func Register(
	mux *http.ServeMux,
	s timeline.Store,
	requireScope func(scope, method, pattern string, h http.HandlerFunc),
) {
	h := New(s)
	const base = "/api/v1/orgs/{orgID}/services/{serviceID}/timeline"

	requireScope("timeline:read", "GET", base, h.List)
	requireScope("timeline:write", "POST", base, h.Create)
	requireScope("timeline:write", "POST", base+"/sync", h.Sync)
	requireScope("timeline:write", "PUT", base+"/{eventID}", h.Update)
	requireScope("timeline:write", "DELETE", base+"/{eventID}", h.Delete)
}

func (h *Handler) authorizeOrg(w http.ResponseWriter, r *http.Request) (identity.Principal, string, bool) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return identity.Principal{}, "", false
	}
	orgID := r.PathValue("orgID")
	if p.Kind == identity.PrincipalServiceAccount && p.OrgID != orgID {
		httputil.Forbidden(w)
		return identity.Principal{}, "", false
	}
	return p, orgID, true
}

func writeErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, timeline.ErrEventNotFound):
		httputil.JSON(w, http.StatusNotFound, map[string]string{"code": "not_found", "message": err.Error()})
	case errors.Is(err, timeline.ErrTitleRequired), errors.Is(err, timeline.ErrUnknownEventType), errors.Is(err, timeline.ErrUnknownDecisionStatus):
		httputil.BadRequest(w, err.Error())
	default:
		httputil.Error(w, r, err)
	}
}

type touchRequest struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Kind  string `json:"kind"`
}

type eventRequest struct {
	Type               string         `json:"type"`
	Title              string         `json:"title"`
	Summary            string         `json:"summary"`
	EventDate          time.Time      `json:"eventDate"`
	Version            *string        `json:"version,omitempty"`
	ADRNumber          *string        `json:"adrNumber,omitempty"`
	DecisionStatus     *string        `json:"decisionStatus,omitempty"`
	SourceLabel        *string        `json:"sourceLabel,omitempty"`
	SourceURL          *string        `json:"sourceUrl,omitempty"`
	IsAgentSummarized  bool           `json:"isAgentSummarized"`
	Touches            []touchRequest `json:"touches"`
	AttachmentAssetID  *string        `json:"attachmentAssetId,omitempty"`
	AttachmentFileName *string        `json:"attachmentFileName,omitempty"`
	AttachmentFileType *string        `json:"attachmentFileType,omitempty"`
	// SourceRef is read by Sync only; Create and Update ignore it.
	SourceRef string `json:"sourceRef,omitempty"`
}

func (req eventRequest) toInput() timeline.Input {
	touches := make([]timeline.Touch, len(req.Touches))
	for i, t := range req.Touches {
		touches[i] = timeline.Touch{ID: t.ID, Label: t.Label, Kind: t.Kind}
	}

	var status *timeline.DecisionStatus
	if req.DecisionStatus != nil {
		s := timeline.DecisionStatus(*req.DecisionStatus)
		status = &s
	}

	return timeline.Input{
		Type:               timeline.EventType(req.Type),
		Title:              req.Title,
		Summary:            req.Summary,
		EventDate:          req.EventDate,
		Version:            req.Version,
		ADRNumber:          req.ADRNumber,
		DecisionStatus:     status,
		SourceLabel:        req.SourceLabel,
		SourceURL:          req.SourceURL,
		IsAgentSummarized:  req.IsAgentSummarized,
		Touches:            touches,
		AttachmentAssetID:  req.AttachmentAssetID,
		AttachmentFileName: req.AttachmentFileName,
		AttachmentFileType: req.AttachmentFileType,
		SourceRef:          req.SourceRef,
	}
}

// List
// @Summary  List timeline events for a service
// @Tags     timeline
// @Security BearerAuth
// @Param    orgID      path  string  true  "orgID"
// @Param    serviceID  path  string  true  "serviceID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services/{serviceID}/timeline [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	events, err := h.store.ListEventsForService(r.Context(), orgID, r.PathValue("serviceID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"events": events})
}

// Create
// @Summary  Create a timeline event
// @Tags     timeline
// @Security BearerAuth
// @Param    orgID      path  string  true  "orgID"
// @Param    serviceID  path  string  true  "serviceID"
// @Param    body  body  object  true  "request body"
// @Success  201  {object}  timeline.Event
// @Failure  400  {object}  httputil.errorBody
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services/{serviceID}/timeline [post]
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	p, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	var req eventRequest
	if err := httputil.Decode(r, &req); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	in := req.toInput()
	if err := in.Validate(); err != nil {
		writeErr(w, r, err)
		return
	}
	event, err := h.store.CreateEvent(r.Context(), orgID, r.PathValue("serviceID"), p.UserID, in)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, event)
}

// Sync
// @Summary  Upsert a repo-scanned timeline event by sourceRef
// @Tags     timeline
// @Security BearerAuth
// @Param    orgID      path  string  true  "orgID"
// @Param    serviceID  path  string  true  "serviceID"
// @Param    body  body  object  true  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  400  {object}  httputil.errorBody
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services/{serviceID}/timeline/sync [post]
func (h *Handler) Sync(w http.ResponseWriter, r *http.Request) {
	p, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	var reqBody struct {
		eventRequest
		CommitHash *string `json:"commitHash"`
	}
	if err := httputil.Decode(r, &reqBody); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	in := reqBody.toInput()
	if in.SourceRef == "" {
		httputil.BadRequest(w, "sourceRef is required")
		return
	}
	if err := in.Validate(); err != nil {
		writeErr(w, r, err)
		return
	}
	event, created, err := h.store.UpsertEventBySourceRef(r.Context(), orgID, r.PathValue("serviceID"), p.UserID, reqBody.CommitHash, in)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"event": event, "created": created})
}

// Update
// @Summary  Update a timeline event
// @Tags     timeline
// @Security BearerAuth
// @Param    orgID      path  string  true  "orgID"
// @Param    serviceID  path  string  true  "serviceID"
// @Param    eventID    path  string  true  "eventID"
// @Param    body  body  object  true  "request body"
// @Success  200  {object}  timeline.Event
// @Failure  400  {object}  httputil.errorBody
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services/{serviceID}/timeline/{eventID} [put]
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	p, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	var req eventRequest
	if err := httputil.Decode(r, &req); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	in := req.toInput()
	if err := in.Validate(); err != nil {
		writeErr(w, r, err)
		return
	}
	event, err := h.store.UpdateEvent(r.Context(), orgID, r.PathValue("serviceID"), r.PathValue("eventID"), p.UserID, in)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, event)
}

// Delete
// @Summary  Delete a timeline event
// @Tags     timeline
// @Security BearerAuth
// @Param    orgID      path  string  true  "orgID"
// @Param    serviceID  path  string  true  "serviceID"
// @Param    eventID    path  string  true  "eventID"
// @Success  204
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/services/{serviceID}/timeline/{eventID} [delete]
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	if err := h.store.DeleteEvent(r.Context(), orgID, r.PathValue("serviceID"), r.PathValue("eventID")); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
