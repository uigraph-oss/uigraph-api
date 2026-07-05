package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/uigraph/app/internal/cache"
	"github.com/uigraph/app/internal/httputil"
	"github.com/uigraph/app/internal/identity"
	authmw "github.com/uigraph/app/internal/middleware"
	"github.com/uigraph/app/internal/org"
	"github.com/uigraph/app/internal/storage"
	"github.com/uigraph/app/internal/store"
)

// maxAvatarBytes caps an uploaded avatar at 5 MiB.
const maxAvatarBytes = 5 << 20

type avatarStore interface {
	SetUserAvatar(ctx context.Context, userID string, assetID *string) error
	GetServiceAccount(ctx context.Context, id string) (*identity.ServiceAccount, error)
	SetServiceAccountAvatar(ctx context.Context, saID string, assetID *string) error
	GetOrg(ctx context.Context, id string) (*org.Org, error)
	SetOrgLogo(ctx context.Context, id string, assetID *string) error
}

type AvatarHandler struct {
	store   avatarStore
	storage storage.Client
	cache   cache.Client // may be nil
}

func NewAvatarHandler(s avatarStore, st storage.Client, c cache.Client) *AvatarHandler {
	return &AvatarHandler{store: s, storage: st, cache: c}
}

// their own avatar (multipart field "file").
// @Summary  PutUserAvatar
// @Tags     users
// @Security BearerAuth
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /users/me/avatar [put]
func (h *AvatarHandler) PutUserAvatar(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	if p.Kind != identity.PrincipalUser {
		httputil.Forbidden(w)
		return
	}

	assetID := storage.UserAvatarAssetID(p.UserID)
	if !h.resolveAvatarAsset(w, r, assetID) {
		return
	}
	if err := h.store.SetUserAvatar(r.Context(), p.UserID, &assetID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	h.bust(r, p.UserID, assetID)
	httputil.JSON(w, http.StatusOK, map[string]any{"assetId": assetID})
}

// @Summary  DeleteUserAvatar
// @Tags     users
// @Security BearerAuth
// @Success  204  "No Content"
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /users/me/avatar [delete]
func (h *AvatarHandler) DeleteUserAvatar(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	if p.Kind != identity.PrincipalUser {
		httputil.Forbidden(w)
		return
	}

	assetID := storage.UserAvatarAssetID(p.UserID)
	_ = h.storage.Delete(r.Context(), storage.AssetKey(assetID))
	if err := h.store.SetUserAvatar(r.Context(), p.UserID, nil); err != nil {
		httputil.Error(w, r, err)
		return
	}
	h.bust(r, p.UserID, assetID)
	w.WriteHeader(http.StatusNoContent)
}

// PutServiceAccountAvatar handles
// PUT /api/v1/orgs/{orgID}/service-accounts/{saID}/avatar — an admin sets a
// service account's avatar (multipart field "file").
// @Summary  PutServiceAccountAvatar
// @Tags     service-accounts
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    saID  path  string  true  "saID"
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/service-accounts/{saID}/avatar [put]
func (h *AvatarHandler) PutServiceAccountAvatar(w http.ResponseWriter, r *http.Request) {
	saID := r.PathValue("saID")

	sa, err := h.store.GetServiceAccount(r.Context(), saID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if sa == nil {
		httputil.Error(w, r, store.ErrNotFound)
		return
	}

	assetID := storage.ServiceAccountAvatarAssetID(saID)
	if !h.resolveAvatarAsset(w, r, assetID) {
		return
	}
	if err := h.store.SetServiceAccountAvatar(r.Context(), saID, &assetID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	h.bust(r, saID, assetID)
	httputil.JSON(w, http.StatusOK, map[string]any{"assetId": assetID})
}

// DeleteServiceAccountAvatar handles
// DELETE /api/v1/orgs/{orgID}/service-accounts/{saID}/avatar.
// @Summary  DeleteServiceAccountAvatar
// @Tags     service-accounts
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    saID  path  string  true  "saID"
// @Success  204  "No Content"
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/service-accounts/{saID}/avatar [delete]
func (h *AvatarHandler) DeleteServiceAccountAvatar(w http.ResponseWriter, r *http.Request) {
	saID := r.PathValue("saID")

	sa, err := h.store.GetServiceAccount(r.Context(), saID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if sa == nil {
		httputil.Error(w, r, store.ErrNotFound)
		return
	}

	assetID := storage.ServiceAccountAvatarAssetID(saID)
	_ = h.storage.Delete(r.Context(), storage.AssetKey(assetID))
	if err := h.store.SetServiceAccountAvatar(r.Context(), saID, nil); err != nil {
		httputil.Error(w, r, err)
		return
	}
	h.bust(r, saID, assetID)
	w.WriteHeader(http.StatusNoContent)
}

// logo (multipart field "file").
// @Summary  PutOrgLogo
// @Tags     logo
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    body  body  object  false  "request body"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/logo [put]
// @Router   /server/orgs/{orgID}/logo [put]
func (h *AvatarHandler) PutOrgLogo(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")

	o, err := h.store.GetOrg(r.Context(), orgID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if o == nil {
		httputil.Error(w, r, store.ErrNotFound)
		return
	}

	assetID := storage.OrgLogoAssetID(orgID)
	if !h.resolveAvatarAsset(w, r, assetID) {
		return
	}
	if err := h.store.SetOrgLogo(r.Context(), orgID, &assetID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	h.bust(r, orgID, assetID)
	httputil.JSON(w, http.StatusOK, map[string]any{"assetId": assetID})
}

// @Summary  DeleteOrgLogo
// @Tags     logo
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Success  204  "No Content"
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/logo [delete]
// @Router   /server/orgs/{orgID}/logo [delete]
func (h *AvatarHandler) DeleteOrgLogo(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")

	o, err := h.store.GetOrg(r.Context(), orgID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if o == nil {
		httputil.Error(w, r, store.ErrNotFound)
		return
	}

	assetID := storage.OrgLogoAssetID(orgID)
	_ = h.storage.Delete(r.Context(), storage.AssetKey(assetID))
	if err := h.store.SetOrgLogo(r.Context(), orgID, nil); err != nil {
		httputil.Error(w, r, err)
		return
	}
	h.bust(r, orgID, assetID)
	w.WriteHeader(http.StatusNoContent)
}

// putPresignedImage handles the JSON {assetId, contentType} presigned-upload
// branch shared by avatars, logos and provider icons: the browser already PUT
// the bytes to assets/<assetId> via a presigned URL, so copy them into the
// canonical assets/<dstAssetID> key. handled reports whether the request was
// JSON; when handled, ok reports success and an error response was already
// written on failure.
func putPresignedImage(w http.ResponseWriter, r *http.Request, st storage.Client, dstAssetID string) (handled bool, ok bool) {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return false, false
	}
	var body struct {
		AssetID     string `json:"assetId"`
		ContentType string `json:"contentType"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return true, false
	}
	src := strings.TrimSpace(body.AssetID)
	if src == "" {
		httputil.BadRequest(w, "assetId is required")
		return true, false
	}
	contentType := strings.TrimSpace(body.ContentType)
	if contentType == "" {
		contentType = "image/png"
	}
	rc, err := st.Download(r.Context(), storage.AssetKey(src))
	if err != nil {
		httputil.Error(w, r, err)
		return true, false
	}
	defer rc.Close()
	if err := st.Upload(r.Context(), storage.AssetKey(dstAssetID), contentType, rc, -1); err != nil {
		httputil.Error(w, r, err)
		return true, false
	}
	_ = st.Delete(r.Context(), storage.AssetKey(src))
	return true, true
}

// resolveAvatarAsset stores the image under assets/<assetID> from either the
// presigned JSON flow or a multipart "file". It writes an error response and
// returns false on failure.
func (h *AvatarHandler) resolveAvatarAsset(w http.ResponseWriter, r *http.Request, assetID string) bool {
	if handled, ok := putPresignedImage(w, r, h.storage, assetID); handled {
		return ok
	}
	return h.uploadAvatar(w, r, assetID)
}

// uploadAvatar reads the multipart "file" and stores it under assets/<assetID>.
// It writes an error response and returns false on failure.
func (h *AvatarHandler) uploadAvatar(w http.ResponseWriter, r *http.Request, assetID string) bool {
	file, header, err := r.FormFile("file")
	if err != nil {
		httputil.BadRequest(w, "missing file")
		return false
	}
	defer file.Close()

	if header.Size > maxAvatarBytes {
		httputil.BadRequest(w, "avatar too large")
		return false
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	if err := h.storage.Upload(r.Context(), storage.AssetKey(assetID), contentType, file, header.Size); err != nil {
		httputil.Error(w, r, err)
		return false
	}
	return true
}

// bust clears the actor and asset-url caches so the new avatar is served
// immediately rather than after the cache TTLs expire.
func (h *AvatarHandler) bust(r *http.Request, actorID, assetID string) {
	if h.cache == nil {
		return
	}
	_ = h.cache.Del(r.Context(), cache.ActorKey(actorID))
	_ = h.cache.Del(r.Context(), cache.AssetURLKey(assetID))
}
