package storage

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	awshttp "github.com/aws/smithy-go/transport/http"
)

type s3Client struct {
	client  *s3.Client
	presign *s3.PresignClient
	bucket  string
	region  string
}

// newS3Client builds a Client backed by AWS S3 using the AWS SDK v2.
func newS3Client(cfg Config) (Client, error) {
	awsCfg, err := s3Credentials(cfg)
	if err != nil {
		return nil, err
	}

	opts := []func(*s3.Options){
		func(o *s3.Options) { o.UsePathStyle = cfg.ForcePathStyle },
	}
	if cfg.Endpoint != "" {
		awsCfg.BaseEndpoint = aws.String(cfg.Endpoint)
	}

	client := s3.NewFromConfig(awsCfg, opts...)
	return &s3Client{
		client:  client,
		presign: s3.NewPresignClient(client),
		bucket:  cfg.Bucket,
		region:  cfg.Region,
	}, nil
}

// s3Credentials prefers the pod's IAM role whenever one is available:
// AWS_ROLE_ARN is what EKS injects when the ServiceAccount carries the
// eks.amazonaws.com/role-arn annotation, and LoadDefaultConfig resolves it
// (via AWS_WEB_IDENTITY_TOKEN_FILE) along with every other standard AWS
// credential source. A pod can end up with both a role and static keys
// configured (e.g. the keys are also needed by a service that can't use
// IRSA) — the role wins in that case, since it's scoped and rotates
// automatically. Static keys are only used as the fallback, for endpoints
// IRSA can't reach at all (e.g. a non-AWS S3-compatible service).
func s3Credentials(cfg Config) (aws.Config, error) {
	if os.Getenv("AWS_ROLE_ARN") == "" && cfg.AccessKey != "" && cfg.SecretKey != "" {
		return aws.Config{
			Region:      cfg.Region,
			Credentials: credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		}, nil
	}
	return awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(cfg.Region))
}

func (c *s3Client) EnsureBucket(ctx context.Context) error {
	_, err := c.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(c.bucket)})
	if err == nil {
		return nil // already exists
	}
	var re *awshttp.ResponseError
	if !errors.As(err, &re) || re.HTTPStatusCode() != http.StatusNotFound {
		return err // real error (auth failure, network error, etc.) — surface it
	}
	input := &s3.CreateBucketInput{Bucket: aws.String(c.bucket)}
	// us-east-1 is the S3 default region; specifying it in CreateBucketConfiguration is an error.
	if c.region != "" && c.region != "us-east-1" {
		input.CreateBucketConfiguration = &s3types.CreateBucketConfiguration{
			LocationConstraint: s3types.BucketLocationConstraint(c.region),
		}
	}
	_, err = c.client.CreateBucket(ctx, input)
	return err
}

func (c *s3Client) Upload(ctx context.Context, key, contentType string, r io.Reader, size int64) error {
	input := &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		Body:        r,
		ContentType: aws.String(contentType),
	}
	if size >= 0 {
		input.ContentLength = aws.Int64(size)
	}
	_, err := c.client.PutObject(ctx, input)
	return err
}

func (c *s3Client) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := c.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	return out.Body, nil
}

func (c *s3Client) Delete(ctx context.Context, key string) error {
	_, err := c.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	return err
}

func (c *s3Client) PresignURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	req, err := c.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

func (c *s3Client) PresignPutURL(ctx context.Context, key string) (string, error) {
	req, err := c.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(15*time.Minute))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

// compile-time interface check
var _ Client = (*s3Client)(nil)
