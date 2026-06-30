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
	FigmaRedirectURI  string

	// Migrations
	MigrationsDir string // path to SQL files; defaults to embedded
}

func Load() (*Config, error) {
	c := &Config{
		Host:                  env("HOST", ""),
		Port:                  env("PORT", ""),
		PostgresURL:           env("POSTGRES_URL", ""),
		RedisURL:              env("REDIS_URL", ""),
		StorageBackend:        env("STORAGE_BACKEND", "minio"),
		StorageBucket:         env("STORAGE_BUCKET", "uigraph"),
		StorageAccessKey:      env("STORAGE_ACCESS_KEY", ""),
		StorageSecretKey:      env("STORAGE_SECRET_KEY", ""),
		StorageEndpoint:       env("STORAGE_ENDPOINT", ""),
		StoragePublicEndpoint: env("STORAGE_PUBLIC_ENDPOINT", ""),
		StorageRegion:         env("STORAGE_REGION", "us-east-1"),
		StorageForcePathStyle: envBool("STORAGE_FORCE_PATH_STYLE", env("STORAGE_ENDPOINT", "") != ""),
		VectorBackend:         env("VECTOR_BACKEND", "qdrant"),
		QdrantURL:             env("QDRANT_URL", "http://qdrant:6333"),
		EmbeddingBackend:      env("EMBEDDING_BACKEND", "ollama"),
		EmbeddingModel:        env("EMBEDDING_MODEL", "nomic-embed-text"),
		EmbeddingURL:          env("EMBEDDING_URL", "http://ollama:11434"),
		AdminEmail:            env("UIGRAPH_ADMIN_EMAIL", "admin@uigraph.app"),
		AdminPassword:         env("UIGRAPH_ADMIN_PASSWORD", "admin"),
		SecretKey:             env("UIGRAPH_SECRET_KEY", ""),
		Domain:                env("UIGRAPH_DOMAIN", "localhost"),
		LicenseKey:            env("UIGRAPH_LICENSE_KEY", ""),
		PublicURL:             env("UIGRAPH_PUBLIC_URL", "http://localhost:8080"),
		FrontendURL:           env("UIGRAPH_FRONTEND_URL", ""),
		InternalFrontendURL:   env("UIGRAPH_INTERNAL_FRONTEND_URL", ""),
		ChromiumPath:          env("UIGRAPH_CHROMIUM_PATH", ""),
		FigmaClientID:         env("FIGMA_CLIENT_ID", ""),
		FigmaClientSecret:     env("FIGMA_CLIENT_SECRET", ""),
		FigmaRedirectURI:      env("FIGMA_REDIRECT_URI", ""),
		MigrationsDir:         env("MIGRATIONS_DIR", ""),
	}

	if c.PostgresURL == "" {
		return nil, fmt.Errorf("config: POSTGRES_URL is required")
	}
	if c.SecretKey == "" {
		return nil, fmt.Errorf("config: UIGRAPH_SECRET_KEY is required")
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
