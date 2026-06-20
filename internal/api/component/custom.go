package component

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/uigraph/app/internal/componentlib"
	"github.com/uigraph/app/internal/httputil"
	storepkg "github.com/uigraph/app/internal/store"
)

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	var body customComponentBody
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.Name == "" {
		httputil.BadRequest(w, "name is required")
		return
	}

	c := body.toComponent(uuid.NewString(), orgID)
	if err := h.store.SaveCustomComponent(r.Context(), c); err != nil {
		httputil.Error(w, r, err)
		return
	}
	saved, err := h.store.GetComponent(r.Context(), c.ID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if saved == nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	httputil.JSON(w, http.StatusCreated, componentlib.ToFocalPointComponent(*saved, ""))
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	id := r.PathValue("componentID")

	existing, err := h.store.GetComponent(r.Context(), id)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if existing == nil || existing.OrgID == nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}

	var body customComponentBody
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}

	c := body.toComponent(id, orgID)
	c.CreatedAt = existing.CreatedAt
	if err := h.store.SaveCustomComponent(r.Context(), c); err != nil {
		httputil.Error(w, r, err)
		return
	}
	saved, err := h.store.GetComponent(r.Context(), id)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if saved == nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}
	httputil.JSON(w, http.StatusOK, componentlib.ToFocalPointComponent(*saved, ""))
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.store.DeleteComponent(r.Context(), r.PathValue("componentID")); err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

type customComponentBody struct {
	Name            string                     `json:"name"`
	Description     string                     `json:"description"`
	Category        string                     `json:"category"`
	Tags            []string                   `json:"tags"`
	IsActive        *bool                      `json:"isActive"`
	Order           int                        `json:"order"`
	ComponentFields []customComponentFieldBody `json:"componentFields"`
}

type customComponentFieldBody struct {
	ComponentFieldID string   `json:"componentFieldId"`
	Label            string   `json:"label"`
	Type             string   `json:"type"`
	Required         bool     `json:"required"`
	Readonly         *bool    `json:"readonly"`
	Options          []string `json:"options"`
	Order            int      `json:"order"`
}

func (b customComponentBody) toComponent(id, orgID string) componentlib.Component {
	isActive := true
	if b.IsActive != nil {
		isActive = *b.IsActive
	}
	fields := make([]componentlib.ComponentField, len(b.ComponentFields))
	for i, f := range b.ComponentFields {
		fid := f.ComponentFieldID
		if fid == "" {
			fid = uuid.NewString()
		}
		fields[i] = componentlib.ComponentField{
			ID:          fid,
			ComponentID: id,
			Label:       f.Label,
			Type:        f.Type,
			Required:    f.Required,
			Readonly:    f.Readonly,
			Options:     f.Options,
			Order:       f.Order,
		}
	}
	return componentlib.Component{
		ID:           id,
		OrgID:        &orgID,
		Kind:         componentlib.KindFocalPoint,
		Type:         "custom",
		Name:         b.Name,
		Slug:         componentlib.Slugify(b.Name),
		Description:  b.Description,
		CategoryName: b.Category,
		Tags:         b.Tags,
		IsActive:     isActive,
		Order:        b.Order,
		Fields:       fields,
	}
}
