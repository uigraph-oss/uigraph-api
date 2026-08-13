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
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/uigraph/app/internal/httputil"
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
// uigraph-enterprise's billing state. Despite the name (kept for backward
// compatibility -- this originally only carried the seat cap), it now
// carries every free-tier cap/gate uigraph-api enforces: resource counts
// (-1 = unlimited) and feature flags.
type SeatLimitInfo struct {
	Limit         int    // -1 = unlimited
	PlanID        string // "" = free
	MaxDiagrams   int    // -1 = unlimited
	MaxServices   int    // -1 = unlimited
	MaxMaps       int    // -1 = unlimited
	AssistEnabled bool
}

// SeatLimit asks uigraph-enterprise for orgID's current plan capacity --
// seat limit plus every other free-tier cap/gate -- called before adding a
// member or creating a diagram/service/map/chat-session, so uigraph-api can
// reject the action once the org is at capacity or the feature is gated.
// Callers should treat any non-nil error as "unknown, fail open" -- a
// billing-service hiccup must never block a core workflow.
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
		SeatLimit     int    `json:"seatLimit"`
		PlanID        string `json:"planId"`
		MaxDiagrams   int    `json:"maxDiagrams"`
		MaxServices   int    `json:"maxServices"`
		MaxMaps       int    `json:"maxMaps"`
		AssistEnabled bool   `json:"assistEnabled"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return SeatLimitInfo{}, fmt.Errorf("enterprise: decode response: %w", err)
	}
	return SeatLimitInfo{
		Limit: out.SeatLimit, PlanID: out.PlanID,
		MaxDiagrams: out.MaxDiagrams, MaxServices: out.MaxServices, MaxMaps: out.MaxMaps,
		AssistEnabled: out.AssistEnabled,
	}, nil
}

// ResourceLimitReached checks orgID's current count of some resource
// (diagrams, services, maps) against its plan limit before creating another
// one, writing a 409 with an upgrade message if at or over capacity.
// Returns true once it has written a response. A nil c (self-hosted) always
// returns false without any network call. Any error reaching
// uigraph-enterprise fails open -- a billing-service hiccup must never block
// a core workflow like creating a diagram -- and is logged for visibility.
// limit extracts the relevant cap (e.g. info.MaxDiagrams) from the full
// entitlement response; a negative value means unlimited.
func ResourceLimitReached(w http.ResponseWriter, r *http.Request, c *Client, orgID, resourceName string, currentCount int, limit func(SeatLimitInfo) int) bool {
	if c == nil {
		return false
	}
	info, err := c.SeatLimit(r.Context(), orgID)
	if err != nil {
		slog.WarnContext(r.Context(), resourceName+" limit check failed, allowing create (fail open)", "err", err, "orgId", orgID)
		return false
	}
	max := limit(info)
	if max < 0 || currentCount < max {
		return false
	}
	plan := info.PlanID
	if plan == "" {
		plan = "free"
	}
	httputil.JSON(w, http.StatusConflict, map[string]string{
		"code":    resourceName + "_limit_reached",
		"message": "Your organization has reached its " + resourceName + " limit on the " + plan + " plan. Upgrade to create more.",
	})
	return true
}
