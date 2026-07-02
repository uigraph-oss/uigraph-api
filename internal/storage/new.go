package storage

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
	// ForcePathStyle uses path-style bucket addressing (required by MinIO, R2,
	// and most S3-compatible providers). AWS S3 uses virtual-hosted style.
	ForcePathStyle bool
}

// New creates a storage Client for the given backend.
func New(cfg Config) (Client, error) {
	switch cfg.Backend {
	case "s3":
		return newS3Client(cfg)
	default: // "minio" and anything else falls through to MinIO
		return newMinioClient(cfg)
	}
}
