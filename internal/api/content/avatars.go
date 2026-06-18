package content

import (
	"net/http"

	authmw "github.com/uigraph/app/internal/middleware"

	"github.com/uigraph/app/internal/cache"
	"github.com/uigraph/app/internal/identity"
	"github.com/uigraph/app/internal/storage"
	"github.com/uigraph/app/internal/store"
)

// maxAvatarBytes caps an uploaded avatar at 5 MiB.
const maxAvatarBytes = 5 << 20

type AvatarHandler struct {
	store   store.Store
	storage storage.Client
	cache   cache.Client // may be nil
}

func NewAvatarHandler(s store.Store, st storage.Client, c cache.Client) *AvatarHandler {
	return &AvatarHandler{store: s, storage: st, cache: c}
}

// PutUserAvatar handles PUT /api/v1/users/me/avatar — the calling user uploads
// their own avatar (multipart field "file").
func (h *AvatarHandler) PutUserAvatar(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	if p.Kind != identity.PrincipalUser {
		writeErr(w, http.StatusForbidden, "only users can set a personal avatar")
		return
	}

	assetID := storage.UserAvatarAssetID(p.UserID)
	if !h.uploadAvatar(w, r, assetID) {
		return
	}
	if err := h.store.SetUserAvatar(r.Context(), p.UserID, &assetID); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to set avatar")
		return
	}
	h.bust(r, p.UserID, assetID)
	writeJSON(w, http.StatusOK, map[string]any{"assetId": assetID})
}

// DeleteUserAvatar handles DELETE /api/v1/users/me/avatar.
func (h *AvatarHandler) DeleteUserAvatar(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	if p.Kind != identity.PrincipalUser {
		writeErr(w, http.StatusForbidden, "only users can clear a personal avatar")
		return
	}

	assetID := storage.UserAvatarAssetID(p.UserID)
	_ = h.storage.Delete(r.Context(), storage.AssetKey(assetID))
	if err := h.store.SetUserAvatar(r.Context(), p.UserID, nil); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to clear avatar")
		return
	}
	h.bust(r, p.UserID, assetID)
	w.WriteHeader(http.StatusNoContent)
}

// PutServiceAccountAvatar handles
// PUT /api/v1/orgs/{orgID}/service-accounts/{saID}/avatar — an admin sets a
// service account's avatar (multipart field "file").
func (h *AvatarHandler) PutServiceAccountAvatar(w http.ResponseWriter, r *http.Request) {
	saID := r.PathValue("saID")

	sa, err := h.store.GetServiceAccount(r.Context(), saID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if sa == nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}

	assetID := storage.ServiceAccountAvatarAssetID(saID)
	if !h.uploadAvatar(w, r, assetID) {
		return
	}
	if err := h.store.SetServiceAccountAvatar(r.Context(), saID, &assetID); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to set avatar")
		return
	}
	h.bust(r, saID, assetID)
	writeJSON(w, http.StatusOK, map[string]any{"assetId": assetID})
}

// DeleteServiceAccountAvatar handles
// DELETE /api/v1/orgs/{orgID}/service-accounts/{saID}/avatar.
func (h *AvatarHandler) DeleteServiceAccountAvatar(w http.ResponseWriter, r *http.Request) {
	saID := r.PathValue("saID")

	sa, err := h.store.GetServiceAccount(r.Context(), saID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if sa == nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}

	assetID := storage.ServiceAccountAvatarAssetID(saID)
	_ = h.storage.Delete(r.Context(), storage.AssetKey(assetID))
	if err := h.store.SetServiceAccountAvatar(r.Context(), saID, nil); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to clear avatar")
		return
	}
	h.bust(r, saID, assetID)
	w.WriteHeader(http.StatusNoContent)
}

// uploadAvatar reads the multipart "file" and stores it under assets/<assetID>.
// It writes an error response and returns false on failure.
func (h *AvatarHandler) uploadAvatar(w http.ResponseWriter, r *http.Request, assetID string) bool {
	file, header, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "missing file")
		return false
	}
	defer file.Close()

	if header.Size > maxAvatarBytes {
		writeErr(w, http.StatusBadRequest, "avatar too large")
		return false
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	if err := h.storage.Upload(r.Context(), storage.AssetKey(assetID), contentType, file, header.Size); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to store avatar")
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
