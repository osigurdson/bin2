package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

type r2RegistryStorage struct {
	bucket    string
	uploadDir string
	client    *s3.Client
	uploader  *transfermanager.Client
}

const singlePartBlobUploadMaxSize int64 = 128 * 1024 * 1024

func newR2RegistryStorageFromEnv(dataDir string) (*r2RegistryStorage, error) {
	accountID := os.Getenv("R2_ACCOUNT_ID")
	endpoint := os.Getenv("R2_ENDPOINT")
	if endpoint == "" {
		if accountID == "" {
			return nil, errors.New("R2_ACCOUNT_ID or R2_ENDPOINT must be set")
		}
		endpoint = fmt.Sprintf(
			"https://%s.r2.cloudflarestorage.com",
			accountID,
		)
	}

	bucket := os.Getenv("R2_BUCKET")
	if bucket == "" {
		return nil, errors.New("R2_BUCKET must be set")
	}

	accessKeyID := os.Getenv("R2_ACCESS_KEY_ID")
	secretAccessKey := os.Getenv("R2_SECRET_ACCESS_KEY")
	if accessKeyID == "" || secretAccessKey == "" {
		return nil, errors.New(
			"R2_ACCESS_KEY_ID and R2_SECRET_ACCESS_KEY must be set",
		)
	}

	region := getenvDefault("R2_REGION", "auto")
	cfg, err := config.LoadDefaultConfig(
		context.Background(),
		config.WithRegion(region),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				accessKeyID,
				secretAccessKey,
				"",
			),
		),
	)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})

	return &r2RegistryStorage{
		bucket:    bucket,
		uploadDir: filepath.Join(dataDir, "uploads"),
		client:    client,
		uploader: transfermanager.New(client, func(o *transfermanager.Options) {
			o.PartSizeBytes = 64 * 1024 * 1024
		}),
	}, nil
}

func (r *r2RegistryStorage) Init() error {
	return os.MkdirAll(r.uploadDir, 0o755)
}

func (r *r2RegistryStorage) CreateUpload(_ context.Context, uuid string) error {
	path := r.uploadPath(uuid)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	return f.Close()
}

func (r *r2RegistryStorage) AppendUpload(
	_ context.Context,
	uuid string,
	body io.Reader,
) (int64, error) {
	path := r.uploadPath(uuid)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if errors.Is(err, os.ErrNotExist) {
		return 0, ErrUploadNotFound
	}
	if err != nil {
		return 0, err
	}
	defer f.Close()

	tag := uuid[len(uuid)-4:]

	log.Printf("fu: start %s", tag)
	start := time.Now()

	if _, err := io.Copy(f, body); err != nil {
		return 0, err
	}
	info, err := f.Stat()
	if err != nil {
		return 0, err
	}
	log.Printf("fu: end %s, time: %.2f, size: %d",
		tag, time.Since(start).Seconds(),
		info.Size()/1024)
	return info.Size(), nil
}

func (r *r2RegistryStorage) UploadSHA256(
	_ context.Context,
	uuid string,
) (string, error) {
	sum, err := fileSHA256(r.uploadPath(uuid))
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrUploadNotFound
	}
	return sum, err
}

func (r *r2RegistryStorage) DeleteUpload(_ context.Context, uuid string) error {
	err := os.Remove(r.uploadPath(uuid))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (r *r2RegistryStorage) BlobExists(
	ctx context.Context,
	digestHex string,
) (bool, error) {
	_, err := r.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(blobObjectKey(digestHex)),
	})
	if err == nil {
		return true, nil
	}
	if isR2NotFoundErr(err) {
		return false, nil
	}
	return false, err
}

func (r *r2RegistryStorage) BlobSize(
	ctx context.Context,
	digestHex string,
) (int64, error) {
	out, err := r.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(blobObjectKey(digestHex)),
	})
	if err != nil {
		if isR2NotFoundErr(err) {
			return 0, ErrBlobNotFound
		}
		return 0, err
	}
	if out.ContentLength == nil {
		return 0, fmt.Errorf("missing content length for blob %s", digestHex)
	}
	return *out.ContentLength, nil
}

func (r *r2RegistryStorage) GetBlob(
	ctx context.Context,
	digestHex string,
) (io.ReadCloser, int64, error) {
	out, err := r.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(blobObjectKey(digestHex)),
	})
	if err != nil {
		if isR2NotFoundErr(err) {
			return nil, 0, ErrBlobNotFound
		}
		return nil, 0, err
	}

	size := int64(-1)
	if out.ContentLength != nil {
		size = *out.ContentLength
	}
	return out.Body, size, nil
}

func (r *r2RegistryStorage) StoreBlobFromUpload(
	ctx context.Context,
	uuid string,
	digestHex string,
) (int64, error) {
	f, err := os.Open(r.uploadPath(uuid))
	if errors.Is(err, os.ErrNotExist) {
		return 0, ErrUploadNotFound
	}
	if err != nil {
		return 0, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return 0, err
	}

	tag := string(uuid[len(uuid)-4:])
	now := time.Now()
	log.Printf("r2ul: %s start... size=%d", tag, info.Size())
	if info.Size() < singlePartBlobUploadMaxSize {
		_, err = r.client.PutObject(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(r.bucket),
			Key:         aws.String(blobObjectKey(digestHex)),
			Body:        f,
			ContentType: aws.String(defaultBlobContentType),
		})
	} else {
		_, err = r.uploader.UploadObject(ctx, &transfermanager.UploadObjectInput{
			Bucket:      aws.String(r.bucket),
			Key:         aws.String(blobObjectKey(digestHex)),
			Body:        f,
			ContentType: aws.String(defaultBlobContentType),
		})
	}
	log.Printf("r2ul: %s done. time: %.1f", tag, time.Since(now).Seconds())
	if err != nil {
		return 0, err
	}
	if err := r.DeleteUpload(ctx, uuid); err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func (r *r2RegistryStorage) StoreManifest(
	ctx context.Context,
	repo string,
	reference string,
	manifest []byte,
	contentType string,
) error {
	_, err := r.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(r.bucket),
		Key:         aws.String(manifestObjectKey(repo, reference)),
		Body:        bytes.NewReader(manifest),
		ContentType: aws.String(manifestContentType(contentType)),
	})
	return err
}

func (r *r2RegistryStorage) GetManifest(
	ctx context.Context,
	repo string,
	reference string,
) ([]byte, string, error) {
	out, err := r.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(manifestObjectKey(repo, reference)),
	})
	if err != nil {
		if isR2NotFoundErr(err) {
			return nil, "", ErrManifestNotFound
		}
		return nil, "", err
	}
	defer out.Body.Close()

	manifestBytes, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, "", err
	}

	contentType := defaultManifestContentType
	if out.ContentType != nil {
		contentType = manifestContentType(*out.ContentType)
	}
	return manifestBytes, contentType, nil
}

func (r *r2RegistryStorage) uploadPath(uuid string) string {
	return filepath.Join(r.uploadDir, uuid)
}

func isR2NotFoundErr(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchKey", "NoSuchBucket":
			return true
		}
	}
	return false
}
