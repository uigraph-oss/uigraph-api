package storage

import (
	"context"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type minioClient struct {
	mc     *minio.Client
	bucket string
}

// Config is the minimal configuration needed to create a storage client.
type Config struct {
	Backend   string // "minio" | "s3"
	Endpoint  string
	Bucket    string
	AccessKey string
	SecretKey string
}

// New creates a storage client for the given backend.
// Both "minio" and "s3" backends use the MinIO SDK (S3-compatible).
func New(cfg Config) (Client, error) {
	endpoint := cfg.Endpoint
	// Strip scheme — the minio client wants host[:port] only.
	endpoint = strings.TrimPrefix(endpoint, "https://")
	endpoint = strings.TrimPrefix(endpoint, "http://")

	secure := !strings.Contains(cfg.Endpoint, "http://")
	if cfg.Backend == "s3" && cfg.Endpoint == "" {
		// AWS S3: use virtual-hosted-style; endpoint is inferred from bucket/region.
		endpoint = "s3.amazonaws.com"
		secure = true
	}

	mc, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: secure,
	})
	if err != nil {
		return nil, err
	}
	return &minioClient{mc: mc, bucket: cfg.Bucket}, nil
}

func (c *minioClient) EnsureBucket(ctx context.Context) error {
	exists, err := c.mc.BucketExists(ctx, c.bucket)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return c.mc.MakeBucket(ctx, c.bucket, minio.MakeBucketOptions{})
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

func (c *minioClient) PresignURL(ctx context.Context, key string) (string, error) {
	u, err := c.mc.PresignedGetObject(ctx, c.bucket, key, 15*time.Minute, url.Values{})
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
