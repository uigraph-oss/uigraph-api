package content

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/uigraph/app/internal/comment"
	authmw "github.com/uigraph/app/internal/middleware"
	"github.com/uigraph/app/internal/store"
)

// CommentHandler serves org-scoped resource comments.
type CommentHandler struct {
	store store.Store
}

func NewCommentHandler(s store.Store) *CommentHandler {
	return &CommentHandler{store: s}
}

func (h *CommentHandler) List(w http.ResponseWriter, r *http.Request) {
	resourceID := r.URL.Query().Get("resourceId")
	if resourceID == "" {
		writeErr(w, http.StatusBadRequest, "resourceId is required")
		return
	}
	comments, err := h.store.ListComments(r.Context(), r.PathValue("orgID"), resourceID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"comments": comments})
}

func (h *CommentHandler) Create(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	var body struct {
		ResourceID      string  `json:"resourceId"`
		Text            string  `json:"text"`
		ParentCommentID *string `json:"parentCommentId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.ResourceID == "" {
		writeErr(w, http.StatusBadRequest, "resourceId is required")
		return
	}

	now := time.Now().UTC()
	c := comment.Comment{
		ID:              uuid.NewString(),
		OrgID:           orgID,
		ResourceID:      body.ResourceID,
		ParentCommentID: body.ParentCommentID,
		Text:            body.Text,
		CreatedBy:       p.UserID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := h.store.CreateComment(r.Context(), c); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (h *CommentHandler) Update(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	c, err := h.store.GetComment(r.Context(), r.PathValue("commentID"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if c == nil || c.DeletedAt != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}

	var body struct {
		Text *string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Text != nil {
		c.Text = *body.Text
	}
	c.UpdatedBy = &p.UserID
	if err := h.store.UpdateComment(r.Context(), *c); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (h *CommentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	if err := h.store.SoftDeleteComment(r.Context(), r.PathValue("commentID"), p.UserID); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
