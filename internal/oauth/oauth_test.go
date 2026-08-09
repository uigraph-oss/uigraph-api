package oauth

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func makeIDToken(t *testing.T, claims map[string]any) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"RS256"}`)) + "." + enc(payload) + ".signature"
}

func TestAuthCodeURLNonce(t *testing.T) {
	p := Provider{ClientID: "cid", AuthURL: "https://idp.example.com/authorize", Scopes: "openid email"}

	raw := p.AuthCodeURL("https://app.example.com/cb", "st4te", "n0nce")
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := u.Query().Get("nonce"); got != "n0nce" {
		t.Fatalf("nonce = %q, want n0nce", got)
	}
	if got := u.Query().Get("state"); got != "st4te" {
		t.Fatalf("state = %q, want st4te", got)
	}

	raw = p.AuthCodeURL("https://app.example.com/cb", "st4te", "")
	if strings.Contains(raw, "nonce") {
		t.Fatal("an empty nonce should not be sent at all")
	}

	p.AuthURL = "https://idp.example.com/authorize?tenant=acme"
	raw = p.AuthCodeURL("https://app.example.com/cb", "st4te", "n0nce")
	if !strings.Contains(raw, "?tenant=acme&") {
		t.Fatalf("an auth URL with an existing query should be appended to, got %q", raw)
	}
}

func TestIDTokenClaims(t *testing.T) {
	tok := makeIDToken(t, map[string]any{"sub": "u1", "groups": []any{"eng"}, "nonce": "n1"})

	claims, err := IDTokenClaims(tok)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims["sub"] != "u1" || claims["nonce"] != "n1" {
		t.Fatalf("claims not decoded: %#v", claims)
	}

	if _, err := IDTokenClaims("not-a-jwt"); err == nil {
		t.Fatal("expected an error for a non-JWT")
	}
	if _, err := IDTokenClaims("a.!!!.c"); err == nil {
		t.Fatal("expected an error for an undecodable payload")
	}
}

func TestVerifyNonce(t *testing.T) {
	if err := VerifyNonce(map[string]any{"nonce": "n1"}, "n1"); err != nil {
		t.Fatalf("matching nonce should pass: %v", err)
	}
	if err := VerifyNonce(map[string]any{"nonce": "n1"}, "n2"); err == nil {
		t.Fatal("a mismatched nonce must be rejected")
	}
	if err := VerifyNonce(map[string]any{"sub": "u1"}, "n1"); err == nil {
		t.Fatal("an id_token without a nonce claim must be rejected")
	}
	if err := VerifyNonce(map[string]any{"nonce": "n1"}, ""); err == nil {
		t.Fatal("a login that issued no nonce must not validate an id_token")
	}
}
