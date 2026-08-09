// Package oauth implements a minimal OAuth2 / OIDC authorization-code flow for
// DB-configured providers (generic OIDC, Microsoft Entra ID, Okta).
//
// Identity is read from the OIDC userinfo endpoint and merged with the id_token
// payload, because group and role claims are frequently present only in the
// id_token and userinfo alone is not enough to drive role mapping.
//
// The id_token's signature is not verified against the provider's JWKS. That is
// sound in this flow specifically: the token arrives directly from the token
// endpoint over TLS, in response to a code this server just issued, which is the
// exception OIDC Core §3.1.3.7 permits. It would *not* be sound for a token
// received from the browser or any other third party.
package oauth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Provider type identifiers stored in auth_providers.type.
const (
	Generic = "generic"
	Entra   = "entra"
	Okta    = "okta"
)

// Provider holds the resolved endpoints and credentials for one OAuth provider.
type Provider struct {
	Name         string
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	UserinfoURL  string
	Scopes       string
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
// nonce binds the resulting id_token to this request and is echoed back in the
// token's nonce claim; pass "" only for plain OAuth2 providers that issue no
// id_token.
func (p Provider) AuthCodeURL(redirectURI, state, nonce string) string {
	q := url.Values{
		"client_id":     {p.ClientID},
		"redirect_uri":  {redirectURI},
		"response_type": {"code"},
		"scope":         {p.Scopes},
		"state":         {state},
	}
	if nonce != "" {
		q.Set("nonce", nonce)
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

// Token is the subset of the token endpoint's response this package uses.
// IDToken is empty for plain OAuth2 providers such as GitHub.
type Token struct {
	AccessToken string
	IDToken     string
}

// Exchange swaps an authorization code for an access token and, for OIDC
// providers, an id_token.
func Exchange(ctx context.Context, p Provider, code, redirectURI string) (Token, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {p.ClientID},
		"client_secret": {p.ClientSecret},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return Token{}, fmt.Errorf("oauth: build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return Token{}, fmt.Errorf("oauth: token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return Token{}, fmt.Errorf("oauth: token endpoint returned %d: %s", resp.StatusCode, body)
	}

	// Some providers (e.g. GitHub) return HTTP 200 with an error payload instead
	// of an HTTP error status, so check the decoded body for an error too.
	var tok struct {
		AccessToken      string `json:"access_token"`
		IDToken          string `json:"id_token"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &tok); err != nil {
		return Token{}, fmt.Errorf("oauth: decode token response: %w", err)
	}
	if tok.AccessToken == "" {
		if tok.Error != "" {
			return Token{}, fmt.Errorf("oauth: token endpoint error %q: %s", tok.Error, tok.ErrorDescription)
		}
		return Token{}, fmt.Errorf("oauth: token response missing access_token")
	}
	return Token{AccessToken: tok.AccessToken, IDToken: tok.IDToken}, nil
}

// IDTokenClaims decodes a JWT's payload without verifying its signature. See the
// package doc for why that is acceptable for a token taken straight from the
// token endpoint, and only there.
func IDTokenClaims(idToken string) (map[string]any, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("oauth: id_token is not a JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("oauth: decode id_token payload: %w", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("oauth: parse id_token payload: %w", err)
	}
	return claims, nil
}

// VerifyNonce checks that the id_token echoes the nonce sent on the
// authorization request, binding the token to that request.
//
// A provider that issues no id_token has nothing to bind, so there is nothing to
// check. A provider that issues one and drops the nonce is rejected: OIDC Core
// §3.1.3.7 requires it to be echoed whenever it was sent.
func VerifyNonce(claims map[string]any, want string) error {
	if want == "" {
		return fmt.Errorf("oauth: no nonce was issued for this login")
	}
	got, ok := claims["nonce"].(string)
	if !ok || got == "" {
		return fmt.Errorf("oauth: id_token is missing the nonce claim")
	}
	if got != want {
		return fmt.Errorf("oauth: id_token nonce does not match the login request")
	}
	return nil
}

// MergedClaims returns the id_token claims overlaid with the userinfo response.
//
// Both are needed: userinfo is authoritative for profile fields and is fetched
// live, while groups and roles are often only in the id_token. Userinfo wins on
// the keys it actually returns; id_token-only keys survive.
//
// When the provider issues an id_token, its nonce is verified against nonce
// before any of its claims are used.
func MergedClaims(ctx context.Context, p Provider, tok Token, nonce string) (map[string]any, error) {
	merged := map[string]any{}

	if tok.IDToken != "" {
		idClaims, err := IDTokenClaims(tok.IDToken)
		if err != nil {
			return nil, err
		}
		if err := VerifyNonce(idClaims, nonce); err != nil {
			return nil, err
		}
		for k, v := range idClaims {
			merged[k] = v
		}
	}

	userinfo, err := FetchUserInfo(ctx, p, tok.AccessToken)
	if err != nil {
		return nil, err
	}
	for k, v := range userinfo {
		merged[k] = v
	}
	return merged, nil
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
