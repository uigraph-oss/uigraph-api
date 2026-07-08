// Package oauth implements a minimal OAuth2 / OIDC authorization-code flow for
// DB-configured providers (generic OIDC, Microsoft Entra ID, Okta).
//
// Token exchange uses the provider token endpoint; user identity is read from
// the OIDC userinfo endpoint. No id_token JWT/JWKS validation is performed —
// the access token is exchanged over TLS and immediately used against userinfo.
package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Provider type identifiers stored in oauth_provider_config.type.
const (
	Generic = "generic"
	Entra   = "entra"
	Okta    = "okta"
)

// Provider holds the resolved endpoints and credentials for one OAuth provider.
type Provider struct {
	Name           string
	ClientID       string
	ClientSecret   string
	AuthURL        string
	TokenURL       string
	UserinfoURL    string
	Scopes         string
	AllowedDomains string // comma-separated; empty = unrestricted
}

// EntraEndpoints derives the Microsoft Entra ID (Azure AD) authorization, token,
// and userinfo URLs from a directory (tenant) ID.
func EntraEndpoints(tenantID string) (authURL, tokenURL, userinfoURL string) {
	base := "https://login.microsoftonline.com/" + tenantID + "/oauth2/v2.0"
	return base + "/authorize", base + "/token", "https://graph.microsoft.com/oidc/userinfo"
}

// OktaEndpoints derives the Okta authorization, token, and userinfo URLs from an
// Okta domain (e.g. company.okta.com).
func OktaEndpoints(domain string) (authURL, tokenURL, userinfoURL string) {
	base := "https://" + domain + "/oauth2/v1"
	return base + "/authorize", base + "/token", base + "/userinfo"
}

// AuthCodeURL builds the provider authorization URL to redirect the browser to.
func (p Provider) AuthCodeURL(redirectURI, state string) string {
	q := url.Values{
		"client_id":     {p.ClientID},
		"redirect_uri":  {redirectURI},
		"response_type": {"code"},
		"scope":         {p.Scopes},
		"state":         {state},
	}
	sep := "?"
	if strings.Contains(p.AuthURL, "?") {
		sep = "&"
	}
	return p.AuthURL + sep + q.Encode()
}

var httpClient = &http.Client{Timeout: 15 * time.Second}

// userAgent is sent on all provider requests; GitHub's API rejects requests
// without one (HTTP 403).
const userAgent = "uigraph"

// Exchange swaps an authorization code for an access token.
func Exchange(ctx context.Context, p Provider, code, redirectURI string) (string, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {p.ClientID},
		"client_secret": {p.ClientSecret},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("oauth: build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("oauth: token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("oauth: token endpoint returned %d: %s", resp.StatusCode, body)
	}

	// Some providers (e.g. GitHub) return HTTP 200 with an error payload instead
	// of an HTTP error status, so check the decoded body for an error too.
	var tok struct {
		AccessToken      string `json:"access_token"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", fmt.Errorf("oauth: decode token response: %w", err)
	}
	if tok.AccessToken == "" {
		if tok.Error != "" {
			return "", fmt.Errorf("oauth: token endpoint error %q: %s", tok.Error, tok.ErrorDescription)
		}
		return "", fmt.Errorf("oauth: token response missing access_token")
	}
	return tok.AccessToken, nil
}

// FetchUserInfo calls the provider userinfo endpoint and returns the raw claims.
func FetchUserInfo(ctx context.Context, p Provider, accessToken string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.UserinfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("oauth: build userinfo request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oauth: userinfo request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oauth: userinfo endpoint returned %d: %s", resp.StatusCode, body)
	}

	var claims map[string]any
	if err := json.Unmarshal(body, &claims); err != nil {
		return nil, fmt.Errorf("oauth: decode userinfo response: %w", err)
	}

	// GitHub is not OIDC: its /user endpoint returns a null email when the user
	// keeps the address private. Detect the GitHub userinfo host and backfill the
	// email from /user/emails so callers can treat the response like any other
	// OIDC userinfo payload. Requires the user:email scope on the access token.
	if isGitHubHost(p.UserinfoURL) {
		if s, _ := claims["email"].(string); s == "" {
			email, err := fetchGitHubPrimaryEmail(ctx, accessToken)
			if err != nil {
				return nil, err
			}
			if email != "" {
				claims["email"] = email
			}
		}
	}

	return claims, nil
}

// isGitHubHost reports whether rawURL points at GitHub's API host.
func isGitHubHost(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "api.github.com" || host == "github.com"
}

// fetchGitHubPrimaryEmail returns the user's primary verified email from GitHub's
// /user/emails endpoint, falling back to the first verified address.
func fetchGitHubPrimaryEmail(ctx context.Context, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/emails", nil)
	if err != nil {
		return "", fmt.Errorf("oauth: build github emails request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("oauth: github emails request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("oauth: github emails endpoint returned %d: %s", resp.StatusCode, body)
	}

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.Unmarshal(body, &emails); err != nil {
		return "", fmt.Errorf("oauth: decode github emails: %w", err)
	}
	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}
	for _, e := range emails {
		if e.Verified {
			return e.Email, nil
		}
	}
	return "", nil
}

// EmailAllowed reports whether email passes the provider's allowed-domain filter.
// An empty AllowedDomains permits any domain.
func (p Provider) EmailAllowed(email string) bool {
	if p.AllowedDomains == "" {
		return true
	}
	at := strings.LastIndex(email, "@")
	if at < 0 {
		return false
	}
	domain := strings.ToLower(email[at+1:])
	for _, d := range strings.Split(p.AllowedDomains, ",") {
		if strings.ToLower(strings.TrimSpace(d)) == domain {
			return true
		}
	}
	return false
}
