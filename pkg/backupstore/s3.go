/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package backupstore

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const (
	BucketEnv         = "KRKN_BACKUP_S3_BUCKET"
	PrefixEnv         = "KRKN_BACKUP_S3_PREFIX"
	RegionEnv         = "KRKN_BACKUP_S3_REGION"
	EndpointEnv       = "KRKN_BACKUP_S3_ENDPOINT"
	PathStyleEnv      = "KRKN_BACKUP_S3_FORCE_PATH_STYLE"
	PresignTTLEnv     = "KRKN_BACKUP_S3_PRESIGN_TTL"
	KMSKeyIDEnv       = "KRKN_BACKUP_S3_KMS_KEY_ID"
	DefaultS3Prefix   = "krkn-operator/backups"
	DefaultPresignTTL = 15 * time.Minute
)

// S3Config configures an S3 or S3-compatible archive store.
type S3Config struct {
	Bucket         string
	Prefix         string
	Region         string
	Endpoint       string
	ForcePathStyle bool
	PresignTTL     time.Duration
	KMSKeyID       string
}

// ObjectReference identifies an S3 object without embedding credentials in
// the reference. The object is read using the caller's AWS credential chain.
type ObjectReference struct {
	Bucket string
	Key    string
}

// S3Store stores archives in a private S3 bucket and returns short-lived
// presigned GET URLs. Credentials are loaded through the AWS SDK default
// credential chain, including workload identity and IRSA.
type S3Store struct {
	client     *s3.Client
	presigner  *s3.PresignClient
	bucket     string
	prefix     string
	presignTTL time.Duration
	kmsKeyID   string
}

// NewS3Store creates an S3-backed archive store.
func NewS3Store(ctx context.Context, config S3Config) (*S3Store, error) {
	config.Bucket = strings.TrimSpace(config.Bucket)
	if config.Bucket == "" {
		return nil, fmt.Errorf("S3 bucket must not be empty")
	}
	if config.PresignTTL <= 0 {
		config.PresignTTL = DefaultPresignTTL
	}
	if config.PresignTTL > 24*time.Hour {
		return nil, fmt.Errorf("S3 presigned URL TTL must not exceed 24h")
	}
	config.Prefix = strings.TrimSpace(config.Prefix)
	if config.Prefix == "" {
		config.Prefix = DefaultS3Prefix
	}
	if err := validatePrefix(config.Prefix); err != nil {
		return nil, err
	}
	config.Prefix = strings.Trim(config.Prefix, "/")

	var loadOptions []func(*awsconfig.LoadOptions) error
	if config.Region != "" {
		loadOptions = append(loadOptions, awsconfig.WithRegion(config.Region))
	}
	awsConfig, err := awsconfig.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}
	if awsConfig.Region == "" {
		awsConfig.Region = "us-east-1"
	}

	clientOptions := []func(*s3.Options){
		func(options *s3.Options) {
			options.UsePathStyle = config.ForcePathStyle
		},
	}
	if config.Endpoint != "" {
		endpoint := strings.TrimRight(strings.TrimSpace(config.Endpoint), "/")
		parsed, err := url.Parse(endpoint)
		if err != nil || parsed.Host == "" {
			return nil, fmt.Errorf("invalid S3 endpoint URL: %q", endpoint)
		}
		if parsed.Scheme != "https" && parsed.Scheme != "http" {
			return nil, fmt.Errorf("S3 endpoint must use http or https scheme, got %q", parsed.Scheme)
		}
		clientOptions = append(clientOptions, func(options *s3.Options) {
			options.BaseEndpoint = aws.String(endpoint)
		})
	}

	client := s3.NewFromConfig(awsConfig, clientOptions...)
	return &S3Store{
		client:     client,
		presigner:  s3.NewPresignClient(client),
		bucket:     config.Bucket,
		prefix:     config.Prefix,
		presignTTL: config.PresignTTL,
		kmsKeyID:   strings.TrimSpace(config.KMSKeyID),
	}, nil
}

// NewFromEnv returns a configured S3 store when KRKN_BACKUP_S3_BUCKET is set.
// An unset bucket leaves S3 persistence disabled.
func NewFromEnv(ctx context.Context) (Store, error) {
	bucket := strings.TrimSpace(os.Getenv(BucketEnv))
	if bucket == "" {
		return nil, nil
	}
	return NewS3StoreForBucketFromEnv(ctx, bucket)
}

// NewS3StoreForBucketFromEnv creates a store for bucket using the AWS/S3
// connection settings from the environment. The bucket argument is used for
// direct object reads where the bucket is supplied by an s3:// reference.
func NewS3StoreForBucketFromEnv(ctx context.Context, bucket string) (*S3Store, error) {
	bucket = strings.TrimSpace(bucket)
	if bucket == "" {
		return nil, fmt.Errorf("S3 bucket must not be empty")
	}

	presignTTL := DefaultPresignTTL
	if raw := strings.TrimSpace(os.Getenv(PresignTTLEnv)); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid %s: %w", PresignTTLEnv, err)
		}
		presignTTL = parsed
	}

	forcePathStyle := false
	if raw := strings.TrimSpace(os.Getenv(PathStyleEnv)); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid %s: %w", PathStyleEnv, err)
		}
		forcePathStyle = parsed
	}

	return NewS3Store(ctx, S3Config{
		Bucket:         bucket,
		Prefix:         os.Getenv(PrefixEnv),
		Region:         os.Getenv(RegionEnv),
		Endpoint:       os.Getenv(EndpointEnv),
		ForcePathStyle: forcePathStyle,
		PresignTTL:     presignTTL,
		KMSKeyID:       os.Getenv(KMSKeyIDEnv),
	})
}

// ParseS3URI validates and parses an s3://bucket/key reference. Query
// parameters and fragments are rejected so restore cannot silently interpret
// an expiring presigned URL as an object reference.
func ParseS3URI(raw string) (ObjectReference, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "s3" || parsed.Host == "" || parsed.User != nil {
		return ObjectReference{}, fmt.Errorf("invalid S3 URI")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return ObjectReference{}, fmt.Errorf("S3 URI must not contain query parameters or fragments")
	}
	key, err := url.PathUnescape(strings.TrimPrefix(parsed.EscapedPath(), "/"))
	if err != nil {
		return ObjectReference{}, fmt.Errorf("invalid S3 object key: %w", err)
	}
	if err := validateReadableObjectKey(key); err != nil {
		return ObjectReference{}, err
	}
	return ObjectReference{Bucket: parsed.Host, Key: key}, nil
}

// Put uploads an archive and returns a short-lived presigned download URL.
// The key determines the content type and disposition: keys ending with ".enc"
// are treated as encrypted archives (application/octet-stream), while all
// others are treated as plain gzip archives (application/gzip).
func (s *S3Store) Put(ctx context.Context, key string, source io.Reader, size int64) (Location, error) {
	if s == nil || s.client == nil || s.presigner == nil {
		return Location{}, fmt.Errorf("S3 store is not initialized")
	}
	if size < 0 {
		return Location{}, fmt.Errorf("archive size must not be negative")
	}
	objectKey, err := objectKey(s.prefix, key)
	if err != nil {
		return Location{}, err
	}

	contentType := "application/gzip"
	if strings.HasSuffix(key, ".enc") {
		contentType = "application/octet-stream"
	}

	putInput := &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(objectKey),
		Body:          source,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(contentType),
		// Archives are immutable backup artifacts; do not silently replace one
		// if an object key collision ever occurs.
		IfNoneMatch:          aws.String("*"),
		ServerSideEncryption: types.ServerSideEncryptionAes256,
	}
	if s.kmsKeyID != "" {
		putInput.ServerSideEncryption = types.ServerSideEncryptionAwsKms
		putInput.SSEKMSKeyId = aws.String(s.kmsKeyID)
	}
	if _, err := s.client.PutObject(ctx, putInput); err != nil {
		return Location{}, fmt.Errorf("failed to upload backup archive: %w", err)
	}

	presigned, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket:                     aws.String(s.bucket),
		Key:                        aws.String(objectKey),
		ResponseContentDisposition: aws.String(fmt.Sprintf("attachment; filename=%q", path.Base(objectKey))),
		ResponseContentType:        aws.String(contentType),
	}, func(options *s3.PresignOptions) {
		options.Expires = s.presignTTL
	})
	if err != nil {
		return Location{}, fmt.Errorf("failed to create backup download URL: %w", err)
	}

	return Location{Key: objectKey, DownloadURL: presigned.URL}, nil
}

// Get reads an archive by its complete S3 object key. It does not create a
// presigned URL or expose credentials to the caller.
func (s *S3Store) Get(ctx context.Context, key string) (io.ReadCloser, int64, error) {
	if s == nil || s.client == nil {
		return nil, 0, fmt.Errorf("S3 store is not initialized")
	}
	if err := validateReadableObjectKey(key); err != nil {
		return nil, 0, err
	}

	object, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to download backup archive: %w", err)
	}
	return object.Body, aws.ToInt64(object.ContentLength), nil
}

func objectKey(prefix, key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" || strings.ContainsAny(key, `/\\`) || path.Base(key) != key || key == "." || key == ".." {
		return "", fmt.Errorf("invalid backup object key")
	}
	return path.Join(prefix, key), nil
}

func validateReadableObjectKey(key string) error {
	if key == "" || strings.HasPrefix(key, "/") || strings.Contains(key, `\`) {
		return fmt.Errorf("invalid S3 object key")
	}
	for _, segment := range strings.Split(key, "/") {
		if segment == "." || segment == ".." {
			return fmt.Errorf("invalid S3 object key")
		}
	}
	return nil
}

func validatePrefix(prefix string) error {
	cleaned := path.Clean(prefix)
	if cleaned == "." || path.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return fmt.Errorf("invalid S3 object prefix")
	}
	return nil
}
