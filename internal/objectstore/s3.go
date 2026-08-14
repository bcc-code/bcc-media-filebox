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
// (default "send/"). Region and credentials resolve through the AWS SDK's
// standard chain — AWS_REGION / AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY,
// the shared config files, or an attached instance role — so nothing beyond
// the bucket name is FileBox-specific configuration.
//
// Returns (nil, nil) when S3_BUCKET is unset. A nil *Client means "S3 not
// configured", and every caller treats that as "keep using local targets",
// so an existing deploy behaves exactly as it did before this package existed.
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

// Key derives the object key for an upload. It is a pure function of values
// already persisted on the uploads row, which is why S3-backed shares need no
// extra schema: the download path recomputes the same key the upload path
// wrote. Namespacing by upload ID also means two files with the same name
// never collide, so this needs no equivalent of tus.uniquePath's numeric
// suffixing.
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

// PresignDownload returns a URL that grants anonymous GET access to key until
// expiry elapses. The bucket itself stays private (Block Public Access on);
// this signed URL is the only way a recipient reaches the bytes, and it is
// what lets downloads bypass this server entirely.
//
// filename is sent as a response-content-disposition override so the browser
// saves the file under its original name rather than the key's trailing
// segment. Interpolating it into the header unquoted is safe because every
// stored filename has already been through tus.SanitizeFilename, which admits
// only [A-Za-z0-9_-] plus a single dot.
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
