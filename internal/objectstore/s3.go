// Package objectstore backs the Send feature's file storage with AWS S3
// instead of a local target directory. Send never lets the sender pick a
// target (see internal/server's preUploadCreate), so there is exactly one
// implicit destination — this package is it.
package objectstore

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// TargetName is the reserved upload `target` metadata value that routes an
// upload to S3 rather than to a local target directory. It is deliberately
// never a row in the targets table, so the admin UI can't create, rename, or
// delete it out from under the Send flow.
const TargetName = "s3"

const defaultKeyPrefix = "send/"

// Client uploads to and presigns objects in a single S3 bucket.
type Client struct {
	uploader  *manager.Uploader
	presigner *s3.PresignClient
	bucket    string
	prefix    string
}

// NewFromEnv builds a Client from S3_BUCKET plus an optional S3_KEY_PREFIX
// (default "send/"). Region and credentials come from the AWS SDK's standard
// chain (env vars, shared config, or an instance role).
//
// Returns (nil, nil) when S3_BUCKET is unset; callers treat a nil Client as
// "keep using local targets", so deploys without S3 are unaffected.
func NewFromEnv(ctx context.Context) (*Client, error) {
	bucket := os.Getenv("S3_BUCKET")
	if bucket == "" {
		return nil, nil
	}

	prefix := os.Getenv("S3_KEY_PREFIX")
	if prefix == "" {
		prefix = defaultKeyPrefix
	} else if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}
	if cfg.Region == "" {
		return nil, fmt.Errorf("S3_BUCKET is set but no AWS region is configured (set AWS_REGION)")
	}

	cli := s3.NewFromConfig(cfg)
	return &Client{
		uploader:  manager.NewUploader(cli),
		presigner: s3.NewPresignClient(cli),
		bucket:    bucket,
		prefix:    prefix,
	}, nil
}

// Bucket returns the configured bucket name, for logging.
func (c *Client) Bucket() string { return c.bucket }

// Key derives the object key from values already on the uploads row, so the
// download path recomputes what the upload path wrote — which is why S3-backed
// shares need no extra schema. Namespacing by upload ID also rules out
// same-name collisions, so no equivalent of tus.uniquePath is needed.
func (c *Client) Key(uploadID, filename string) string {
	return c.prefix + uploadID + "/" + filename
}

// Upload streams the file at path into the bucket under key, switching to a
// multipart upload automatically for large files.
func (c *Client) Upload(ctx context.Context, key, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	if _, err := c.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
		Body:   f,
	}); err != nil {
		return fmt.Errorf("upload to s3://%s/%s: %w", c.bucket, key, err)
	}
	return nil
}

// PresignDownload returns a URL granting anonymous GET access to key until
// expiry. The bucket stays private, so this is the only way a recipient reaches
// the bytes — and what keeps download traffic off this server.
//
// filename overrides Content-Disposition so the browser saves the original
// name. Interpolating it unquoted is safe: stored filenames have been through
// tus.SanitizeFilename ([A-Za-z0-9_-] plus one dot).
func (c *Client) PresignDownload(ctx context.Context, key, filename string, expiry time.Duration) (string, error) {
	req, err := c.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket:                     aws.String(c.bucket),
		Key:                        aws.String(key),
		ResponseContentDisposition: aws.String(fmt.Sprintf("attachment; filename=%q", filename)),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("presign s3://%s/%s: %w", c.bucket, key, err)
	}
	return req.URL, nil
}
