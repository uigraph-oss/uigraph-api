package storage

import "fmt"

// Config is the minimal configuration needed to create a storage client.
type Config struct {
	Backend   string // "minio" | "s3"
	Endpoint  string
	Bucket    string
	AccessKey string
	SecretKey string
	Region    string
	// PublicEndpoint is the browser-reachable host used when signing presigned
	// GET/PUT URLs. When empty, Endpoint is used. Only relevant for MinIO.
	PublicEndpoint string
}

// New creates a storage Client for the given backend.
func New(cfg Config) (Client, error) {
	switch cfg.Backend {
	case "s3":
		return nil, fmt.Errorf("storage: s3 backend not yet implemented")
	default: // "minio" and anything else falls through to MinIO
		return newMinioClient(cfg)
	}
}
