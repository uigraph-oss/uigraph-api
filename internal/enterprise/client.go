// Package enterprise is a thin client for the internal, shared-secret
// authenticated endpoints the private uigraph-enterprise service exposes.
// It's the mirror image of uigraph-enterprise's own internal/uigraphapi
// client (which calls back into this service) -- this is the first thing in
// uigraph-api that calls out to uigraph-enterprise. Only ever constructed
// when UIGRAPH_ENTERPRISE=true; self-hosted builds never import an HTTP
// client here at all, they just keep a nil *Client.
package enterprise

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL       string
	internalToken string
	http          *http.Client
}

func New(baseURL, internalToken string) *Client {
	return &Client{
		baseURL:       strings.TrimRight(baseURL, "/"),
		internalToken: internalToken,
		http:          &http.Client{Timeout: 5 * time.Second},
	}
}

// SeatLimitInfo is an org's current plan capacity, as known by
// uigraph-enterprise's billing state.
type SeatLimitInfo struct {
	Limit  int    // -1 = unlimited
	PlanID string // "" = free
}

// SeatLimit asks uigraph-enterprise for orgID's current seat limit and plan,
// called before adding a member (invite or SSO self-join) so uigraph-api can
// reject the add once the org is at capacity. Callers should treat any
// non-nil error as "unknown, fail open" -- a billing-service hiccup must
// never block a core workflow like adding a teammate.
func (c *Client) SeatLimit(ctx context.Context, orgID string) (SeatLimitInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/internal/v1/orgs/"+orgID+"/seat-limit", nil)
	if err != nil {
		return SeatLimitInfo{}, fmt.Errorf("enterprise: build request: %w", err)
	}
	req.Header.Set("X-Internal-Token", c.internalToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return SeatLimitInfo{}, fmt.Errorf("enterprise: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return SeatLimitInfo{}, fmt.Errorf("enterprise: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var out struct {
		SeatLimit int    `json:"seatLimit"`
		PlanID    string `json:"planId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return SeatLimitInfo{}, fmt.Errorf("enterprise: decode response: %w", err)
	}
	return SeatLimitInfo{Limit: out.SeatLimit, PlanID: out.PlanID}, nil
}
