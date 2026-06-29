package figmaapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/uigraph/app/internal/crypto"
	"github.com/uigraph/app/internal/figma"
	"github.com/uigraph/app/internal/httputil"
	"github.com/uigraph/app/internal/identity"
	authmw "github.com/uigraph/app/internal/middleware"
)

const stateCookie = "uigraph_figma_state"

type store interface {
	GetFigmaAuth(ctx context.Context, userID string) (*identity.FigmaAuth, error)
	UpsertFigmaAuth(ctx context.Context, a identity.FigmaAuth) error
	DeleteFigmaAuth(ctx context.Context, userID string) error
}

type Handler struct {
	store       store
	client      *figma.Client
	cipher      *crypto.Cipher
	publicURL   string
	frontendURL string
}

func New(s store, client *figma.Client, cipher *crypto.Cipher, publicURL, frontendURL string) *Handler {
	return &Handler{
		store:       s,
		client:      client,
		cipher:      cipher,
		publicURL:   strings.TrimRight(publicURL, "/"),
		frontendURL: strings.TrimRight(frontendURL, "/"),
	}
}

func Register(mux *http.ServeMux, h *Handler, protected func(method, pattern string, hf http.HandlerFunc)) {
	protected("GET", "/api/v1/figma/connect", h.Connect)
	protected("GET", "/api/v1/figma/callback", h.Callback)
	protected("GET", "/api/v1/figma/status", h.Status)
	protected("POST", "/api/v1/figma/disconnect", h.Disconnect)
	protected("GET", "/api/v1/figma/node-info", h.NodeInfo)
}

func (h *Handler) frontendBase() string {
	if h.frontendURL != "" {
		return h.frontendURL
	}
	return h.publicURL
}

func (h *Handler) secureCookies() bool {
	return strings.HasPrefix(h.publicURL, "https://")
}

func (h *Handler) Connect(w http.ResponseWriter, r *http.Request) {
	if !h.client.Configured() {
		httputil.JSON(w, http.StatusServiceUnavailable, map[string]string{"code": "not_configured", "message": "Figma integration is not configured"})
		return
	}
	state := randomToken()
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookie,
		Value:    state,
		Path:     "/api/v1/figma",
		MaxAge:   600,
		HttpOnly: true,
		Secure:   h.secureCookies(),
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, h.client.AuthorizeURL(state), http.StatusFound)
}

func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}

	c, err := r.Cookie(stateCookie)
	if err != nil || c.Value == "" || c.Value != r.URL.Query().Get("state") {
		h.respondPopup(w, false, "invalid OAuth state")
		return
	}
	h.clearCookie(w)

	code := r.URL.Query().Get("code")
	if code == "" {
		h.respondPopup(w, false, "missing authorization code")
		return
	}

	tokens, err := h.client.ExchangeCode(r.Context(), code)
	if err != nil {
		h.respondPopup(w, false, "failed to exchange code")
		return
	}

	encAccess, err := h.cipher.Encrypt(tokens.AccessToken)
	if err != nil {
		h.respondPopup(w, false, "failed to store token")
		return
	}
	encRefresh, err := h.cipher.Encrypt(tokens.RefreshToken)
	if err != nil {
		h.respondPopup(w, false, "failed to store token")
		return
	}

	err = h.store.UpsertFigmaAuth(r.Context(), identity.FigmaAuth{
		UserID:       p.UserID,
		FigmaUserID:  tokens.UserID,
		AccessToken:  encAccess,
		RefreshToken: encRefresh,
		ExpiresAt:    time.Now().UTC().Add(time.Duration(tokens.ExpiresIn) * time.Second),
	})
	if err != nil {
		h.respondPopup(w, false, "failed to store token")
		return
	}

	h.respondPopup(w, true, "")
}

func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	if !h.client.Configured() {
		httputil.JSON(w, http.StatusOK, map[string]any{"connected": false, "configured": false})
		return
	}
	auth, err := h.store.GetFigmaAuth(r.Context(), p.UserID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"connected": auth != nil, "configured": true})
}

func (h *Handler) Disconnect(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	if err := h.store.DeleteFigmaAuth(r.Context(), p.UserID); err != nil {
		httputil.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) NodeInfo(w http.ResponseWriter, r *http.Request) {
	p, ok := authmw.PrincipalFromCtx(r.Context())
	if !ok {
		httputil.Unauthorized(w)
		return
	}
	figmaURL := r.URL.Query().Get("url")
	if figmaURL == "" {
		httputil.BadRequest(w, "url query parameter is required")
		return
	}
	fileKey, nodeID, err := figma.ParseURL(figmaURL)
	if err != nil {
		httputil.BadRequest(w, err.Error())
		return
	}

	auth, err := h.store.GetFigmaAuth(r.Context(), p.UserID)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if auth == nil {
		httputil.JSON(w, http.StatusForbidden, map[string]string{"code": "figma_not_connected", "message": "Figma account is not connected"})
		return
	}

	accessToken, err := h.cipher.Decrypt(auth.AccessToken)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}

	info, err := h.client.GetNodeInfo(r.Context(), accessToken, fileKey, nodeID)
	if err == figma.ErrUnauthorized {
		accessToken, err = h.refresh(r, p.UserID, auth)
		if err != nil {
			httputil.JSON(w, http.StatusForbidden, map[string]string{"code": "figma_not_connected", "message": "Figma session expired, please reconnect"})
			return
		}
		info, err = h.client.GetNodeInfo(r.Context(), accessToken, fileKey, nodeID)
	}
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	if info.ImageURL == "" {
		httputil.JSON(w, http.StatusNotFound, map[string]string{"code": "not_found", "message": "Figma component not found"})
		return
	}
	httputil.JSON(w, http.StatusOK, info)
}

func (h *Handler) refresh(r *http.Request, userID string, auth *identity.FigmaAuth) (string, error) {
	refreshToken, err := h.cipher.Decrypt(auth.RefreshToken)
	if err != nil {
		return "", err
	}
	tokens, err := h.client.RefreshToken(r.Context(), refreshToken)
	if err != nil {
		return "", err
	}
	newRefresh := tokens.RefreshToken
	if newRefresh == "" {
		newRefresh = refreshToken
	}
	encAccess, err := h.cipher.Encrypt(tokens.AccessToken)
	if err != nil {
		return "", err
	}
	encRefresh, err := h.cipher.Encrypt(newRefresh)
	if err != nil {
		return "", err
	}
	err = h.store.UpsertFigmaAuth(r.Context(), identity.FigmaAuth{
		UserID:       userID,
		FigmaUserID:  auth.FigmaUserID,
		AccessToken:  encAccess,
		RefreshToken: encRefresh,
		ExpiresAt:    time.Now().UTC().Add(time.Duration(tokens.ExpiresIn) * time.Second),
	})
	if err != nil {
		return "", err
	}
	return tokens.AccessToken, nil
}

func (h *Handler) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookie,
		Value:    "",
		Path:     "/api/v1/figma",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.secureCookies(),
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) respondPopup(w http.ResponseWriter, success bool, errMsg string) {
	origin := h.frontendBase()
	payload := map[string]string{"type": "figma-oauth-error", "error": errMsg}
	if success {
		payload = map[string]string{"type": "figma-oauth-connected"}
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, "failed to encode popup response", http.StatusInternalServerError)
		return
	}
	originJSON, err := json.Marshal(origin)
	if err != nil {
		http.Error(w, "failed to encode popup response", http.StatusInternalServerError)
		return
	}
	page := `<!doctype html><html><body><script>
try { if (window.opener) window.opener.postMessage(` + string(payloadJSON) + `, ` + string(originJSON) + `); } catch (e) {}
window.close();
</script></body></html>`
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(page))
}

func randomToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
