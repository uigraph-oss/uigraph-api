// Package asset provides an HTTP handler that resolves asset IDs to presigned URLs.
package asset

import (
	"errors"
	"io"
	"net/http"
	"strings"

	assetpkg "github.com/uigraph/app/internal/asset"
	"github.com/uigraph/app/internal/cache"
	"github.com/uigraph/app/internal/httputil"
	"github.com/uigraph/app/internal/storage"
)

const maxAssetIDs = 200

// createUploadRequest is optional: with no body (or no contentHash) the asset
// gets a fresh random ID. Supplying the sha256 of the content to be uploaded
// makes the ID deterministic, so a re-upload of identical bytes replaces the
// object rather than orphaning it — used by the CLI's test-case screenshot sync.
type createUploadRequest struct {
	ContentHash string `json:"contentHash"`
}

// Handler wraps asset.Resolver for HTTP.
type Handler struct {
	resolver *assetpkg.Resolver
	storage  storage.Client
}

// New constructs a Handler.
func New(st storage.Client, c cache.Client) *Handler {
	return &Handler{resolver: assetpkg.New(st, c), storage: st}
}

// Register wires the asset URL route into mux.
func Register(
	mux *http.ServeMux,
	st storage.Client,
	c cache.Client,
	protected func(method, pattern string, h http.HandlerFunc),
) {
	h := New(st, c)
	protected("GET", "/api/v1/orgs/{orgID}/assets/urls", h.Resolve)
	protected("POST", "/api/v1/orgs/{orgID}/assets", h.CreateUpload)
}

// CreateUpload
// @Summary  CreateUpload
// @Tags     assets
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/assets [post]
func (h *Handler) CreateUpload(w http.ResponseWriter, r *http.Request) {
	var req createUploadRequest
	switch err := httputil.Decode(r, &req); {
	case errors.Is(err, io.EOF):
		req.ContentHash = ""
	case err != nil:
		httputil.BadRequest(w, "invalid request body")
		return
	}

	var assetID string
	switch {
	case req.ContentHash == "":
		assetID = storage.NewFileAssetID()
	case storage.IsSHA256Hex(req.ContentHash):
		assetID = storage.FileAssetIDFromHash(req.ContentHash)
	default:
		httputil.BadRequest(w, "contentHash must be a 64-character lowercase sha256 hex string")
		return
	}

	url, err := h.storage.PresignPutURL(r.Context(), storage.AssetKey(assetID))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, map[string]any{
		"assetId":   assetID,
		"uploadUrl": url,
	})
}

// @Summary  Resolve
// @Tags     assets
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/assets/urls [get]
func (h *Handler) Resolve(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("ids")
	if raw == "" {
		httputil.BadRequest(w, "ids query parameter is required")
		return
	}
	var ids []string
	for _, part := range strings.Split(raw, ",") {
		if id := strings.TrimSpace(part); id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		httputil.BadRequest(w, "ids query parameter is required")
		return
	}
	if len(ids) > maxAssetIDs {
		httputil.BadRequest(w, "too many ids")
		return
	}
	urls, err := h.resolver.ResolveMany(r.Context(), ids)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"urls": urls})
}
