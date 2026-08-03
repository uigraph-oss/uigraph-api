package billing

import (
	"encoding/json"
	"net/http"

	"github.com/uigraph/app/internal/billing"
	"github.com/uigraph/app/internal/httputil"
)

type createConnectionRequest struct {
	Provider    billing.Provider `json:"provider"`
	DisplayName string           `json:"displayName"`

	RoleARN    string `json:"roleArn,omitempty"`
	ExternalID string `json:"externalId,omitempty"`

	ServiceAccountJSON string `json:"serviceAccountJson,omitempty"`
	BillingDataset     string `json:"billingDataset,omitempty"`

	TenantID       string `json:"tenantId,omitempty"`
	ClientID       string `json:"clientId,omitempty"`
	ClientSecret   string `json:"clientSecret,omitempty"`
	SubscriptionID string `json:"subscriptionId,omitempty"`
}

func (req createConnectionRequest) toInput() billing.ConnectionInput {
	return billing.ConnectionInput{
		Provider:           req.Provider,
		DisplayName:        req.DisplayName,
		RoleARN:            req.RoleARN,
		ExternalID:         req.ExternalID,
		ServiceAccountJSON: req.ServiceAccountJSON,
		BillingDataset:     req.BillingDataset,
		TenantID:           req.TenantID,
		ClientID:           req.ClientID,
		ClientSecret:       req.ClientSecret,
		SubscriptionID:     req.SubscriptionID,
	}
}

// CreateConnection stores a new cloud billing connection. The credential
// payload is encrypted before it ever reaches the store — decrypted only
// in-process for TestConnection / the sync worker.
func (h *Handler) CreateConnection(w http.ResponseWriter, r *http.Request) {
	p, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}

	var req createConnectionRequest
	if err := httputil.Decode(r, &req); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	in := req.toInput()
	if err := in.Validate(); err != nil {
		writeErr(w, r, err)
		return
	}

	payload, err := json.Marshal(in)
	if err != nil {
		httputil.Error(w, r, err)
		return
	}
	encrypted, err := h.cipher.Encrypt(string(payload))
	if err != nil {
		httputil.Error(w, r, err)
		return
	}

	conn, err := h.store.CreateCloudConnection(r.Context(), orgID, p.UserID, in.Provider, in.DisplayName, encrypted)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, conn)
}

func (h *Handler) DeleteConnection(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	if err := h.store.DeleteCloudConnection(r.Context(), orgID, r.PathValue("connectionID")); err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// TestConnection decrypts the stored credential and verifies it can
// authenticate against the provider, without persisting a sync.
func (h *Handler) TestConnection(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	conn, in, err := h.loadDecryptedConnection(r, orgID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	adapter, err := h.adapterFor(conn.Provider)
	if err != nil {
		writeErr(w, r, err)
		return
	}

	if err := adapter.TestConnection(r.Context(), in); err != nil {
		_ = h.store.UpdateCloudConnectionStatus(r.Context(), conn.ID, billing.ConnectionStatusError, err.Error())
		httputil.JSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	_ = h.store.UpdateCloudConnectionStatus(r.Context(), conn.ID, billing.ConnectionStatusConnected, "")
	httputil.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

// TriggerSync runs a synchronous, on-demand sync for one connection — the
// same path the scheduled background worker calls on an interval.
func (h *Handler) TriggerSync(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	conn, encrypted, err := h.store.GetCloudConnectionAuth(r.Context(), orgID, r.PathValue("connectionID"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	adapter, err := h.adapterFor(conn.Provider)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	resourceCount, err := billing.RunSync(r.Context(), h.store, h.cipher, adapter, orgID, conn, encrypted)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"resourceCount": resourceCount})
}

func (h *Handler) loadDecryptedConnection(r *http.Request, orgID string) (*billing.Connection, billing.ConnectionInput, error) {
	conn, encrypted, err := h.store.GetCloudConnectionAuth(r.Context(), orgID, r.PathValue("connectionID"))
	if err != nil {
		return nil, billing.ConnectionInput{}, err
	}
	decrypted, err := h.cipher.Decrypt(encrypted)
	if err != nil {
		return nil, billing.ConnectionInput{}, err
	}
	var in billing.ConnectionInput
	if err := json.Unmarshal([]byte(decrypted), &in); err != nil {
		return nil, billing.ConnectionInput{}, err
	}
	return conn, in, nil
}

type createTagRuleRequest struct {
	TagKey   string `json:"tagKey"`
	TagValue string `json:"tagValue"`
}

func (h *Handler) CreateTagRule(w http.ResponseWriter, r *http.Request) {
	p, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	var req createTagRuleRequest
	if err := httputil.Decode(r, &req); err != nil || req.TagKey == "" || req.TagValue == "" {
		httputil.BadRequest(w, "tagKey and tagValue are required")
		return
	}
	rule, err := h.store.CreateTagRule(r.Context(), orgID, r.PathValue("serviceID"), p.UserID, req.TagKey, req.TagValue)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, rule)
}

func (h *Handler) DeleteTagRule(w http.ResponseWriter, r *http.Request) {
	_, orgID, ok := h.authorizeOrg(w, r)
	if !ok {
		return
	}
	err := h.store.DeleteTagRule(r.Context(), orgID, r.PathValue("serviceID"), r.PathValue("ruleID"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
