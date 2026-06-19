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
	mc        *minio.Client
	presignMC *minio.Client
	bucket    string
	backend   string
}

// newMinioClient builds a Client backed by MinIO (or any S3-compatible endpoint
// that the MinIO SDK supports). Called by New() when Backend != "s3".
func newMinioClient(cfg Config) (Client, error) {
	mc, err := buildMinio(cfg.Endpoint, cfg)
	if err != nil {
		return nil, err
	}

	presignMC := mc
	if cfg.PublicEndpoint != "" {
		presignMC, err = buildMinio(cfg.PublicEndpoint, cfg)
		if err != nil {
			return nil, err
		}
	}

	return &minioClient{mc: mc, presignMC: presignMC, bucket: cfg.Bucket, backend: cfg.Backend}, nil
}

// buildMinio creates a raw minio.Client for the given endpoint.
func buildMinio(endpoint string, cfg Config) (*minio.Client, error) {
	secure := !strings.Contains(endpoint, "http://")
	host := strings.TrimPrefix(endpoint, "https://")
	host = strings.TrimPrefix(host, "http://")

	if cfg.Backend == "s3" && endpoint == "" {
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
