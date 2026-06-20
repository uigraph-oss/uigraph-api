package comment

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/uigraph/app/internal/comment"
	"github.com/uigraph/app/internal/httputil"
	authmw "github.com/uigraph/app/internal/middleware"
	storepkg "github.com/uigraph/app/internal/store"
)

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	resourceID := r.URL.Query().Get("resourceId")
	if resourceID == "" {
		httputil.BadRequest(w, "resourceId is required")
		return
	}
	comments, err := h.store.ListComments(r.Context(), r.PathValue("orgID"), resourceID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"comments": comments})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}

	var body struct {
		ResourceID      string  `json:"resourceId"`
		Text            string  `json:"text"`
		ParentCommentID *string `json:"parentCommentId"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.ResourceID == "" {
		httputil.BadRequest(w, "resourceId is required")
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
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, c)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	c, err := h.store.GetComment(r.Context(), r.PathValue("commentID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if c == nil || c.DeletedAt != nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}

	var body struct {
		Text *string `json:"text"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Text != nil {
		c.Text = *body.Text
	}
	c.UpdatedBy = &p.UserID
	if err := h.store.UpdateComment(r.Context(), *c); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, c)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	if err := h.store.SoftDeleteComment(r.Context(), r.PathValue("commentID"), p.UserID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"ok": true})
}
