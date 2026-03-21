package registrystorage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

// R2BlobDeleter deletes blobs from a Cloudflare R2 (S3-compatible) bucket.
// S3 DeleteObject is idempotent: it returns success even when the key is absent.
type R2BlobDeleter struct {
	bucket string
	client *s3.Client
}

// NewFromEnv constructs an R2BlobDeleter from environment variables:
//
//	R2_ACCOUNT_ID or R2_ENDPOINT (endpoint takes precedence)
//	R2_BUCKET
//	R2_ACCESS_KEY_ID
//	R2_SECRET_ACCESS_KEY
//	R2_REGION (optional, defaults to "auto")
func NewFromEnv() (*R2BlobDeleter, error) {
	endpoint := os.Getenv("R2_ENDPOINT")
	if endpoint == "" {
		accountID := os.Getenv("R2_ACCOUNT_ID")
		if accountID == "" {
			return nil, errors.New("R2_ACCOUNT_ID or R2_ENDPOINT must be set")
		}
		endpoint = fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)
	}

	bucket := os.Getenv("R2_BUCKET")
	if bucket == "" {
		return nil, errors.New("R2_BUCKET must be set")
	}

	accessKeyID := os.Getenv("R2_ACCESS_KEY_ID")
	secretAccessKey := os.Getenv("R2_SECRET_ACCESS_KEY")
	if accessKeyID == "" || secretAccessKey == "" {
		return nil, errors.New("R2_ACCESS_KEY_ID and R2_SECRET_ACCESS_KEY must be set")
	}

	region := os.Getenv("R2_REGION")
	if region == "" {
		region = "auto"
	}

	cfg, err := config.LoadDefaultConfig(
		context.Background(),
		config.WithRegion(region),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("load R2 config: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})

	return &R2BlobDeleter{bucket: bucket, client: client}, nil
}

// DeleteBlob removes a blob from R2 storage.  The operation is idempotent:
// deleting a key that does not exist is treated as success.
func (r *R2BlobDeleter) DeleteBlob(ctx context.Context, digestHex string) error {
	_, err := r.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(blobObjectKey(digestHex)),
	})
	if err != nil && !isNotFoundErr(err) {
		return fmt.Errorf("delete blob %s: %w", digestHex, err)
	}
	return nil
}

func blobObjectKey(digestHex string) string {
	return path.Join("blobs", "sha256", digestHex[:2], digestHex)
}

func isNotFoundErr(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchKey", "NoSuchBucket":
			return true
		}
	}
	return false
}
