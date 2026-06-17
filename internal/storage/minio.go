package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type minioClient struct {
	// mc is the internal client the server uses for uploads/downloads.
	mc *minio.Client
	// presignMC signs presigned URLs against the browser-reachable host.
	// Equals mc when no public endpoint is configured.
	presignMC *minio.Client
	bucket    string
	backend   string
}

// Config is the minimal configuration needed to create a storage client.
type Config struct {
	Backend   string // "minio" | "s3"
	Endpoint  string
	Bucket    string
	AccessKey string
	SecretKey string
	Region    string
	// PublicEndpoint is the browser-reachable host used to sign presigned URLs.
	// Endpoint is the internal host the server uses for uploads/downloads (e.g.
	// "http://minio:9000"); a presigned URL signed against it is unusable from a
	// browser because the host is part of the V4 signature. When set, presigning
	// is done against PublicEndpoint (e.g. "http://localhost:9000"). When empty,
	// presigning falls back to Endpoint.
	PublicEndpoint string
}

// New creates a storage client for the given backend.
// Both "minio" and "s3" backends use the MinIO SDK (S3-compatible).
func New(cfg Config) (Client, error) {
	mc, err := newMinio(cfg.Endpoint, cfg)
	if err != nil {
		return nil, err
	}

	presignMC := mc
	if cfg.PublicEndpoint != "" {
		presignMC, err = newMinio(cfg.PublicEndpoint, cfg)
		if err != nil {
			return nil, err
		}
	}

	return &minioClient{mc: mc, presignMC: presignMC, bucket: cfg.Bucket, backend: cfg.Backend}, nil
}

// newMinio builds a minio client for endpoint using cfg's credentials/region.
func newMinio(endpoint string, cfg Config) (*minio.Client, error) {
	secure := !strings.Contains(endpoint, "http://")
	// Strip scheme — the minio client wants host[:port] only.
	host := strings.TrimPrefix(endpoint, "https://")
	host = strings.TrimPrefix(host, "http://")

	if cfg.Backend == "s3" && endpoint == "" {
		// AWS S3: use virtual-hosted-style; endpoint is inferred from bucket/region.
		host = "s3.amazonaws.com"
		secure = true
	}

	return minio.New(host, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: secure,
		Region: cfg.Region,
	})
}

func (c *minioClient) EnsureBucket(ctx context.Context) error {
	exists, err := c.mc.BucketExists(ctx, c.bucket)
	if err != nil {
		return err
	}
	if !exists {
		if err := c.mc.MakeBucket(ctx, c.bucket, minio.MakeBucketOptions{}); err != nil {
			return err
		}
	}

	// Keep the bucket private: assets are reachable only via presigned URLs.
	// Clear any public policy a previous version may have set on assets/*.
	if c.backend == "minio" {
		if err := c.mc.SetBucketPolicy(ctx, c.bucket, ""); err != nil {
			return fmt.Errorf("storage: clear bucket policy: %w", err)
		}
	}
	return nil
}

func (c *minioClient) Upload(ctx context.Context, key, contentType string, r io.Reader, size int64) error {
	_, err := c.mc.PutObject(ctx, c.bucket, key, r, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

func (c *minioClient) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	return c.mc.GetObject(ctx, c.bucket, key, minio.GetObjectOptions{})
}

func (c *minioClient) Delete(ctx context.Context, key string) error {
	return c.mc.RemoveObject(ctx, c.bucket, key, minio.RemoveObjectOptions{})
}

func (c *minioClient) PresignURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	u, err := c.presignMC.PresignedGetObject(ctx, c.bucket, key, ttl, url.Values{})
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (c *minioClient) PresignPutURL(ctx context.Context, key string) (string, error) {
	u, err := c.mc.PresignedPutObject(ctx, c.bucket, key, 15*time.Minute)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}
