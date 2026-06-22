package diagram

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	diagrampkg "github.com/uigraph/app/internal/diagram"
	"github.com/uigraph/app/internal/httputil"
	authmw "github.com/uigraph/app/internal/middleware"
	storepkg "github.com/uigraph/app/internal/store"
	"github.com/uigraph/app/internal/storage"
)

func (h *Handler) ListImages(w http.ResponseWriter, r *http.Request) {
	images, err := h.store.ListDiagramImages(r.Context(), r.PathValue("diagramID"))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"images": images})
}

func (h *Handler) CreateImage(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	diagramID := r.PathValue("diagramID")
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}

	dg, err := h.store.GetDiagram(r.Context(), diagramID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if dg == nil || dg.DeletedAt != nil {
		httputil.Error(w, r, storepkg.ErrNotFound)
		return
	}

	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "application/json") {
		h.createImageFromAsset(w, r, orgID, diagramID, p.UserID)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		httputil.BadRequest(w, "missing file")
		return
	}
	defer file.Close()

	if contentType == "" {
		contentType = header.Header.Get("Content-Type")
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	order := 0
	if v := r.FormValue("order"); v != "" {
		if n, convErr := strconv.Atoi(v); convErr == nil {
			order = n
		}
	}

	var fileName *string
	if v := r.FormValue("fileName"); v != "" {
		fileName = &v
	} else if header.Filename != "" {
		name := header.Filename
		fileName = &name
	}

	assetID := storage.NewFileAssetID()
	if err := h.storage.Upload(r.Context(), storage.AssetKey(assetID), contentType, file, header.Size); err != nil {
		httputil.Error(w, r, err)
		return
	}

	img, err := h.saveDiagramImage(r, orgID, diagramID, p.UserID, assetID, fileName, order)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}

	httputil.JSON(w, http.StatusCreated, img)
}

func (h *Handler) createImageFromAsset(w http.ResponseWriter, r *http.Request, orgID, diagramID, userID string) {
	var body struct {
		AssetID  string  `json:"assetId"`
		FileName *string `json:"fileName"`
		Order    *int    `json:"order"`
	}
	if err := httputil.Decode(r, &body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if body.AssetID == "" {
		httputil.BadRequest(w, "assetId is required")
		return
	}

	order := 0
	if body.Order != nil {
		order = *body.Order
	}

	img, err := h.saveDiagramImage(r, orgID, diagramID, userID, body.AssetID, body.FileName, order)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}

	httputil.JSON(w, http.StatusCreated, img)
}

func (h *Handler) saveDiagramImage(
	r *http.Request,
	orgID, diagramID, userID, assetID string,
	fileName *string,
	order int,
) (diagrampkg.Image, error) {
	img := diagrampkg.Image{
		ID:        uuid.NewString(),
		DiagramID: diagramID,
		OrgID:     orgID,
		AssetID:   assetID,
		FileName:  fileName,
		Order:     order,
		CreatedBy: userID,
		CreatedAt: time.Now().UTC(),
	}
	if err := h.store.CreateDiagramImage(r.Context(), img); err != nil {
		return diagrampkg.Image{}, err
	}
	return img, nil
}
