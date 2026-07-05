package auth

import (
	"context"
	"net/http"

	"github.com/uigraph/app/internal/cache"
	"github.com/uigraph/app/internal/httputil"
	"github.com/uigraph/app/internal/identity"
	authmw "github.com/uigraph/app/internal/middleware"
	"github.com/uigraph/app/internal/org"
	"github.com/uigraph/app/internal/storage"
	"github.com/uigraph/app/internal/store"
)

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

// presignUpload returns a presigned PUT URL for the canonical assets/<assetID>
// key so the browser uploads image bytes straight to it, overwriting any prior
// image in place.
func (h *AvatarHandler) presignUpload(w http.ResponseWriter, r *http.Request, assetID string) {
	url, err := h.storage.PresignPutURL(r.Context(), storage.AssetKey(assetID))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{
		"assetId":   assetID,
		"uploadUrl": url,
	})
}

// PrepareUserAvatarUpload presigns an upload to the caller's own avatar key.
// @Summary  PrepareUserAvatarUpload
// @Tags     users
// @Security BearerAuth
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /users/me/avatar/prepare [post]
func (h *AvatarHandler) PrepareUserAvatarUpload(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	if p.Kind != identity.PrincipalUser {
		httputil.Forbidden(w)
		return
	}
	h.presignUpload(w, r, storage.UserAvatarAssetID(p.UserID))
}

// PrepareServiceAccountAvatarUpload presigns an upload to a service account's
// avatar key.
// @Summary  PrepareServiceAccountAvatarUpload
// @Tags     service-accounts
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Param    saID  path  string  true  "saID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/service-accounts/{saID}/avatar/prepare [post]
func (h *AvatarHandler) PrepareServiceAccountAvatarUpload(w http.ResponseWriter, r *http.Request) {
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
	h.presignUpload(w, r, storage.ServiceAccountAvatarAssetID(saID))
}

// PrepareOrgLogoUpload presigns an upload to an org's logo key.
// @Summary  PrepareOrgLogoUpload
// @Tags     logo
// @Security BearerAuth
// @Param    orgID  path  string  true  "orgID"
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  httputil.errorBody
// @Failure  403  {object}  httputil.errorBody
// @Failure  404  {object}  httputil.errorBody
// @Failure  500  {object}  httputil.errorBody
// @Router   /orgs/{orgID}/logo/prepare [post]
// @Router   /server/orgs/{orgID}/logo/prepare [post]
func (h *AvatarHandler) PrepareOrgLogoUpload(w http.ResponseWriter, r *http.Request) {
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
	h.presignUpload(w, r, storage.OrgLogoAssetID(orgID))
}

// PutUserAvatar records the caller's own avatar after the bytes were uploaded
// to the canonical key via the prepared presigned URL.
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
// PutServiceAccountAvatar records a service account's avatar after the bytes
// were uploaded to the canonical key via the prepared presigned URL.
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

// PutOrgLogo records an org's logo after the bytes were uploaded to the
// canonical key via the prepared presigned URL.
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

// bust clears the actor and asset-url caches so the new avatar is served
// immediately rather than after the cache TTLs expire.
func (h *AvatarHandler) bust(r *http.Request, actorID, assetID string) {
	if h.cache == nil {
		return
	}
	_ = h.cache.Del(r.Context(), cache.ActorKey(actorID))
	_ = h.cache.Del(r.Context(), cache.AssetURLKey(assetID))
}
