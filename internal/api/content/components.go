package content

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/uigraph/app/internal/componentcatalog"
	"github.com/uigraph/app/internal/store"
)

// ComponentHandler serves the focal-point component palette.
type ComponentHandler struct {
	store store.Store
}

func NewComponentHandler(s store.Store) *ComponentHandler {
	return &ComponentHandler{store: s}
}

// List handles GET /api/v1/orgs/{orgID}/components
func (h *ComponentHandler) List(w http.ResponseWriter, r *http.Request) {
	comps, err := h.store.ListComponentsByKind(r.Context(), componentcatalog.KindFocalPoint)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to list components")
		return
	}

	out := make([]componentcatalog.FocalPointComponent, len(comps))
	for i, c := range comps {
		out[i] = componentcatalog.ToFocalPointComponent(c, componentIconURL(r, c))
	}

	custom, err := h.store.ListCustomComponents(r.Context(), r.PathValue("orgID"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to list custom components")
		return
	}
	customOut := make([]componentcatalog.FocalPointComponent, len(custom))
	for i, c := range custom {
		customOut[i] = componentcatalog.ToFocalPointComponent(c, "")
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"components":       out,
		"customComponents": customOut,
	})
}

// Create handles POST /api/v1/orgs/{orgID}/components
func (h *ComponentHandler) Create(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	var body customComponentBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}

	c := body.toComponent(uuid.NewString(), orgID)
	if err := h.store.SaveCustomComponent(r.Context(), c); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	saved, err := h.store.GetComponent(r.Context(), c.ID)
	if err != nil || saved == nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, componentcatalog.ToFocalPointComponent(*saved, ""))
}

// Update handles PUT /api/v1/orgs/{orgID}/components/{componentID}
func (h *ComponentHandler) Update(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	id := r.PathValue("componentID")

	existing, err := h.store.GetComponent(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if existing == nil || existing.OrgID == nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}

	var body customComponentBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	c := body.toComponent(id, orgID)
	c.CreatedAt = existing.CreatedAt
	if err := h.store.SaveCustomComponent(r.Context(), c); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	saved, err := h.store.GetComponent(r.Context(), id)
	if err != nil || saved == nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, componentcatalog.ToFocalPointComponent(*saved, ""))
}

// Delete handles DELETE /api/v1/orgs/{orgID}/components/{componentID}
func (h *ComponentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.store.DeleteComponent(r.Context(), r.PathValue("componentID")); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
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

func (b customComponentBody) toComponent(id, orgID string) componentcatalog.Component {
	isActive := true
	if b.IsActive != nil {
		isActive = *b.IsActive
	}
	fields := make([]componentcatalog.ComponentField, len(b.ComponentFields))
	for i, f := range b.ComponentFields {
		fid := f.ComponentFieldID
		if fid == "" {
			fid = uuid.NewString()
		}
		fields[i] = componentcatalog.ComponentField{
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
	return componentcatalog.Component{
		ID:           id,
		OrgID:        &orgID,
		Kind:         componentcatalog.KindFocalPoint,
		Type:         "custom",
		Name:         b.Name,
		Slug:         componentcatalog.Slugify(b.Name),
		Description:  b.Description,
		CategoryName: b.Category,
		Tags:         b.Tags,
		IsActive:     isActive,
		Order:        b.Order,
		Fields:       fields,
	}
}

func componentIconURL(r *http.Request, c componentcatalog.Component) string {
	slug := componentcatalog.IconSlug(c)
	return "/api/v1/component-icons/" + slug
}
