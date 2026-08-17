package config

import (
	"fmt"
	"os"
)

type Config struct {
	// HTTP
	Host string
	Port string

	// Postgres
	PostgresURL string

	// Redis
	RedisURL string

	// Object storage
	StorageBackend   string // minio | s3 | gcs
	StorageBucket    string
	StorageAccessKey string
	StorageSecretKey string
	StorageEndpoint  string
	// StoragePublicEndpoint is the browser-reachable host used to sign presigned
	// asset URLs (e.g. http://localhost:9000). Empty falls back to StorageEndpoint.
	StoragePublicEndpoint string
	StorageRegion         string
	// StorageForcePathStyle controls bucket addressing. Path-style is required by
	// MinIO, Cloudflare R2, and most S3-compatible providers; AWS S3 uses
	// virtual-hosted style. Defaults to true whenever StorageEndpoint is set.
	StorageForcePathStyle bool

	// Vector store
	VectorBackend string // qdrant | s3vectors
	QdrantURL     string

	// Embeddings
	EmbeddingBackend string // ollama | bedrock | openai
	EmbeddingModel   string
	EmbeddingURL     string

	// Bootstrap
	AdminEmail    string // default admin user email; defaults to admin@uigraph.app
	AdminPassword string // default admin user password; defaults to admin

	// App
	SecretKey  string // AES-256-GCM key for encrypting tokens at rest
	Domain     string
	LicenseKey string

	// PublicURL is the externally reachable base URL (scheme + host[:port]).
	// Used to build OAuth redirect URIs and the post-login SPA redirect.
	PublicURL string

	// FrontendURL is the SPA base URL the backend redirects to after handling an
	// OAuth callback. When empty it falls back to PublicURL (same-origin prod).
	FrontendURL string

	GatewayURL string

	// CookieDomain sets the session cookie's Domain attribute, e.g.
	// ".example.com" to share it across every subdomain (the app, a
	// marketing/billing site on a different subdomain, etc). Empty (the
	// default) makes the cookie host-only, scoped to whichever exact host
	// issued it — correct for self-hosted's single-shared-URL model.
	CookieDomain string

	// InternalFrontendURL is the SPA base URL the screenshot worker's headless
	// browser navigates to from inside the network. When empty it falls back to
	// FrontendURL. Set this when the browser-facing URL is not reachable from the
	// backend (e.g. localhost in docker-compose).
	InternalFrontendURL string

	// ChromiumPath overrides the headless browser binary the screenshot worker uses.
	// Empty lets chromedp auto-detect chromium/chrome on PATH.
	ChromiumPath string

	FigmaClientID     string
	FigmaClientSecret string

	GitHubAppConfigured       bool
	GitHubAppID               int64
	GitHubAppSlug             string
	GitHubAppClientID         string
	GitHubAppClientSecret     string
	GitHubAppPrivateKeyBase64 string
	GitHubWebhookSecret       string
	GitHubAPIURL              string
	GitHubWebURL              string

	// Enterprise gates the managed-SaaS integration seam: internal endpoints
	// used by the separate, private uigraph-enterprise service (signup
	// provisioning, session introspection, seat-limit checks). Self-hosted
	// deployments leave this unset and never register or call any of it.
	Enterprise              bool
	EnterpriseServiceURL    string
	EnterpriseInternalToken string
	// EnterpriseBillingURL is the managed billing settings page (served by a
	// separate frontend, not this binary) — exposed via the public
	// /api/v1/instance-info endpoint so the frontend knows whether to show a
	// "Billing" link at all. Empty (self-hosted default) hides it entirely.
	EnterpriseBillingURL string

	// Migrations
	MigrationsDir string // path to SQL files; defaults to embedded
}

func Load() (*Config, error) {
	c := &Config{
		Host:                      env("HOST", ""),
		Port:                      env("PORT", ""),
		PostgresURL:               env("POSTGRES_URL", ""),
		RedisURL:                  env("REDIS_URL", ""),
		StorageBackend:            env("STORAGE_BACKEND", "minio"),
		StorageBucket:             env("STORAGE_BUCKET", "uigraph"),
		StorageAccessKey:          env("STORAGE_ACCESS_KEY", ""),
		StorageSecretKey:          env("STORAGE_SECRET_KEY", ""),
		StorageEndpoint:           env("STORAGE_ENDPOINT", ""),
		StoragePublicEndpoint:     env("STORAGE_PUBLIC_ENDPOINT", ""),
		StorageRegion:             env("STORAGE_REGION", "us-east-1"),
		StorageForcePathStyle:     envBool("STORAGE_FORCE_PATH_STYLE", env("STORAGE_ENDPOINT", "") != ""),
		VectorBackend:             env("VECTOR_BACKEND", "qdrant"),
		QdrantURL:                 env("QDRANT_URL", "http://qdrant:6333"),
		EmbeddingBackend:          env("EMBEDDING_BACKEND", "ollama"),
		EmbeddingModel:            env("EMBEDDING_MODEL", "nomic-embed-text"),
		EmbeddingURL:              env("EMBEDDING_URL", "http://ollama:11434"),
		AdminEmail:                env("UIGRAPH_ADMIN_EMAIL", "admin@uigraph.app"),
		AdminPassword:             env("UIGRAPH_ADMIN_PASSWORD", "admin"),
		SecretKey:                 env("UIGRAPH_SECRET_KEY", ""),
		Domain:                    env("UIGRAPH_DOMAIN", "localhost"),
		LicenseKey:                env("UIGRAPH_LICENSE_KEY", ""),
		PublicURL:                 env("UIGRAPH_PUBLIC_URL", "http://localhost:8080"),
		FrontendURL:               env("UIGRAPH_FRONTEND_URL", ""),
		GatewayURL:                env("UIGRAPH_GATEWAY_URL", ""),
		CookieDomain:              env("UIGRAPH_COOKIE_DOMAIN", ""),
		InternalFrontendURL:       env("UIGRAPH_INTERNAL_FRONTEND_URL", ""),
		ChromiumPath:              env("UIGRAPH_CHROMIUM_PATH", ""),
		FigmaClientID:             env("FIGMA_CLIENT_ID", ""),
		FigmaClientSecret:         env("FIGMA_CLIENT_SECRET", ""),
		GitHubAppID:               envInt64("GITHUB_APP_ID", 0),
		GitHubAppSlug:             env("GITHUB_APP_SLUG", ""),
		GitHubAppClientID:         env("GITHUB_APP_CLIENT_ID", ""),
		GitHubAppClientSecret:     env("GITHUB_APP_CLIENT_SECRET", ""),
		GitHubAppPrivateKeyBase64: env("GITHUB_APP_PRIVATE_KEY_BASE64", ""),
		GitHubWebhookSecret:       env("GITHUB_WEBHOOK_SECRET", ""),
		GitHubAPIURL:              env("GITHUB_API_URL", "https://api.github.com/"),
		GitHubWebURL:              env("GITHUB_WEB_URL", "https://github.com"),

		Enterprise:              envBool("UIGRAPH_ENTERPRISE", false),
		EnterpriseServiceURL:    env("UIGRAPH_ENTERPRISE_SERVICE_URL", ""),
		EnterpriseInternalToken: env("UIGRAPH_ENTERPRISE_INTERNAL_TOKEN", ""),
		EnterpriseBillingURL:    env("UIGRAPH_ENTERPRISE_BILLING_URL", ""),

		MigrationsDir: env("MIGRATIONS_DIR", ""),
	}

	if c.PostgresURL == "" {
		return nil, fmt.Errorf("config: POSTGRES_URL is required")
	}
	if c.SecretKey == "" {
		return nil, fmt.Errorf("config: UIGRAPH_SECRET_KEY is required")
	}
	if c.Enterprise && c.EnterpriseInternalToken == "" {
		return nil, fmt.Errorf("config: UIGRAPH_ENTERPRISE_INTERNAL_TOKEN is required when UIGRAPH_ENTERPRISE is true")
	}
	if c.Enterprise && c.EnterpriseServiceURL == "" {
		return nil, fmt.Errorf("config: UIGRAPH_ENTERPRISE_SERVICE_URL is required when UIGRAPH_ENTERPRISE is true")
	}
	c.GitHubAppConfigured = c.GitHubAppID != 0 && c.GitHubAppSlug != "" && c.GitHubAppClientID != "" &&
		c.GitHubAppClientSecret != "" && c.GitHubAppPrivateKeyBase64 != ""
	githubPartial := c.GitHubAppID != 0 || c.GitHubAppSlug != "" || c.GitHubAppClientID != "" ||
		c.GitHubAppClientSecret != "" || c.GitHubAppPrivateKeyBase64 != "" || c.GitHubWebhookSecret != ""
	if !c.GitHubAppConfigured && githubPartial {
		return nil, fmt.Errorf("config: partial GitHub App configuration; GITHUB_APP_ID, GITHUB_APP_SLUG, GITHUB_APP_CLIENT_ID, GITHUB_APP_CLIENT_SECRET, and GITHUB_APP_PRIVATE_KEY_BASE64 are all required")
	}

	return c, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	if v == "true" {
		return true
	}
	if v == "false" {
		return false
	}
	return fallback
}

func envInt64(key string, fallback int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	var parsed int64
	if _, err := fmt.Sscan(v, &parsed); err != nil {
		return fallback
	}
	return parsed
}
