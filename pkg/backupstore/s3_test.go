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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewFromEnvDisabledWithoutBucket(t *testing.T) {
	t.Setenv(BucketEnv, "")

	store, err := NewFromEnv(context.Background())
	if err != nil {
		t.Fatalf("NewFromEnv() error = %v", err)
	}
	if store != nil {
		t.Fatal("NewFromEnv() returned a store when S3 is disabled")
	}
}

func TestNewFromEnvRejectsInvalidTTL(t *testing.T) {
	t.Setenv(BucketEnv, "test-bucket")
	t.Setenv(PresignTTLEnv, "not-a-duration")

	_, err := NewFromEnv(context.Background())
	if err == nil {
		t.Fatal("NewFromEnv() accepted an invalid presigned URL TTL")
	}
}

func TestNewS3StoreRejectsLongTTL(t *testing.T) {
	_, err := NewS3Store(context.Background(), S3Config{
		Bucket:     "test-bucket",
		PresignTTL: 25 * time.Hour,
	})
	if err == nil {
		t.Fatal("NewS3Store() accepted a TTL longer than 24 hours")
	}
}

func TestObjectKey(t *testing.T) {
	key, err := objectKey("krkn-operator/backups", "backup.tar.gz")
	if err != nil {
		t.Fatalf("objectKey() error = %v", err)
	}
	if key != "krkn-operator/backups/backup.tar.gz" {
		t.Fatalf("objectKey() = %q", key)
	}

	for _, invalid := range []string{"", ".", "..", "nested/backup.tar.gz", "/backup.tar.gz", `backup\\archive.tar.gz`} {
		if _, err := objectKey("backups", invalid); err == nil {
			t.Errorf("objectKey(%q) accepted an invalid key", invalid)
		}
	}
}

func TestParseS3URI(t *testing.T) {
	ref, err := ParseS3URI("s3://backup-bucket/krkn-operator/backups/archive.tar.gz.enc")
	if err != nil {
		t.Fatalf("ParseS3URI() error = %v", err)
	}
	if ref.Bucket != "backup-bucket" {
		t.Errorf("bucket = %q, want %q", ref.Bucket, "backup-bucket")
	}
	if ref.Key != "krkn-operator/backups/archive.tar.gz.enc" {
		t.Errorf("key = %q", ref.Key)
	}

	for _, invalid := range []string{
		"",
		"https://backup-bucket/archive.tar.gz",
		"s3://backup-bucket",
		"s3://backup-bucket/archive.tar.gz?download=1",
		"s3://backup-bucket/../archive.tar.gz",
		"s3://backup-bucket/%zz",
	} {
		if _, err := ParseS3URI(invalid); err == nil {
			t.Errorf("ParseS3URI(%q) accepted an invalid URI", invalid)
		}
	}
}

func TestNewS3StoreRejectsUnsafePrefix(t *testing.T) {
	for _, prefix := range []string{"../backups", "/backups", ".."} {
		_, err := NewS3Store(context.Background(), S3Config{
			Bucket: "test-bucket",
			Prefix: prefix,
		})
		if err == nil {
			t.Errorf("NewS3Store() accepted unsafe prefix %q", prefix)
		}
	}
}

func TestNewS3StoreSuccess(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")

	store, err := NewS3Store(context.Background(), S3Config{
		Bucket:     "my-bucket",
		Prefix:     "custom/prefix",
		Region:     "eu-west-1",
		PresignTTL: 10 * time.Minute,
		KMSKeyID:   "arn:aws:kms:eu-west-1:123:key/abc",
	})
	if err != nil {
		t.Fatalf("NewS3Store() error = %v", err)
	}
	if store == nil {
		t.Fatal("NewS3Store() returned nil store")
	}
	if store.bucket != "my-bucket" {
		t.Errorf("bucket = %q, want %q", store.bucket, "my-bucket")
	}
	if store.prefix != "custom/prefix" {
		t.Errorf("prefix = %q, want %q", store.prefix, "custom/prefix")
	}
	if store.presignTTL != 10*time.Minute {
		t.Errorf("presignTTL = %v, want %v", store.presignTTL, 10*time.Minute)
	}
	if store.kmsKeyID != "arn:aws:kms:eu-west-1:123:key/abc" {
		t.Errorf("kmsKeyID = %q", store.kmsKeyID)
	}
}

func TestNewS3StoreDefaults(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")

	store, err := NewS3Store(context.Background(), S3Config{
		Bucket: "my-bucket",
	})
	if err != nil {
		t.Fatalf("NewS3Store() error = %v", err)
	}
	if store.prefix != DefaultS3Prefix {
		t.Errorf("default prefix = %q, want %q", store.prefix, DefaultS3Prefix)
	}
	if store.presignTTL != DefaultPresignTTL {
		t.Errorf("default presignTTL = %v, want %v", store.presignTTL, DefaultPresignTTL)
	}
}

func TestNewS3StoreRejectsEmptyBucket(t *testing.T) {
	_, err := NewS3Store(context.Background(), S3Config{Bucket: "  "})
	if err == nil {
		t.Fatal("NewS3Store() accepted empty bucket")
	}
}

func TestNewS3StoreRejectsInvalidEndpoint(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")

	for _, endpoint := range []string{"ftp://bucket.example.com", "://missing-scheme", "no-scheme-at-all"} {
		_, err := NewS3Store(context.Background(), S3Config{
			Bucket:   "test-bucket",
			Endpoint: endpoint,
		})
		if err == nil {
			t.Errorf("NewS3Store() accepted invalid endpoint %q", endpoint)
		}
	}
}

func TestNewS3StoreAcceptsHTTPEndpoint(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")

	store, err := NewS3Store(context.Background(), S3Config{
		Bucket:   "test-bucket",
		Endpoint: "http://localhost:9000",
	})
	if err != nil {
		t.Fatalf("NewS3Store() should accept http endpoint for local testing, got: %v", err)
	}
	if store == nil {
		t.Fatal("NewS3Store() returned nil")
	}
}

func TestS3StorePutSuccess(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")

	var capturedMethod string
	var capturedPath string
	var capturedBody []byte
	var capturedContentType string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		capturedContentType = r.Header.Get("Content-Type")
		if r.Body != nil {
			capturedBody, _ = io.ReadAll(r.Body)
		}
		w.Header().Set("ETag", `"d41d8cd98f00b204e9800998ecf8427e"`)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	store, err := NewS3Store(context.Background(), S3Config{
		Bucket:         "test-bucket",
		Prefix:         "backups",
		Region:         "us-east-1",
		Endpoint:       ts.URL,
		ForcePathStyle: true,
		PresignTTL:     5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("NewS3Store() error = %v", err)
	}

	data := "fake-archive-content"
	loc, err := store.Put(context.Background(), "test-backup.tar.gz", strings.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if capturedMethod != "PUT" {
		t.Errorf("S3 request method = %q, want PUT", capturedMethod)
	}
	if !strings.Contains(capturedPath, "backups/test-backup.tar.gz") {
		t.Errorf("S3 object path = %q, want to contain %q", capturedPath, "backups/test-backup.tar.gz")
	}
	if capturedContentType != "application/gzip" {
		t.Errorf("content type = %q, want %q", capturedContentType, "application/gzip")
	}
	if string(capturedBody) != data {
		t.Errorf("uploaded body = %q, want %q", capturedBody, data)
	}
	if loc.Key != "backups/test-backup.tar.gz" {
		t.Errorf("location key = %q, want %q", loc.Key, "backups/test-backup.tar.gz")
	}
	if loc.DownloadURL == "" {
		t.Error("Put() returned empty download URL")
	}
	if !strings.Contains(loc.DownloadURL, "test-backup.tar.gz") {
		t.Errorf("download URL %q does not contain the object key", loc.DownloadURL)
	}
}

func TestS3StorePutEncryptedContentType(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")

	var capturedContentType string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedContentType = r.Header.Get("Content-Type")
		w.Header().Set("ETag", `"abc"`)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	store, err := NewS3Store(context.Background(), S3Config{
		Bucket:         "test-bucket",
		Endpoint:       ts.URL,
		ForcePathStyle: true,
	})
	if err != nil {
		t.Fatalf("NewS3Store() error = %v", err)
	}

	_, err = store.Put(context.Background(), "backup.tar.gz.enc", strings.NewReader("data"), 4)
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if capturedContentType != "application/octet-stream" {
		t.Errorf("encrypted content type = %q, want %q", capturedContentType, "application/octet-stream")
	}
}

func TestS3StorePutRejectsNegativeSize(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	store, err := NewS3Store(context.Background(), S3Config{
		Bucket:         "test-bucket",
		Endpoint:       ts.URL,
		ForcePathStyle: true,
	})
	if err != nil {
		t.Fatalf("NewS3Store() error = %v", err)
	}

	_, err = store.Put(context.Background(), "test.tar.gz", strings.NewReader(""), -1)
	if err == nil || !strings.Contains(err.Error(), "negative") {
		t.Fatalf("expected negative size error, got: %v", err)
	}
}

func TestS3StorePutRejectsNilStore(t *testing.T) {
	var store *S3Store
	_, err := store.Put(context.Background(), "test.tar.gz", strings.NewReader(""), 0)
	if err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Fatalf("expected not initialized error, got: %v", err)
	}
}

func TestS3StorePutUploadFailure(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>AccessDenied</Code><Message>Access Denied</Message></Error>`))
	}))
	defer ts.Close()

	store, err := NewS3Store(context.Background(), S3Config{
		Bucket:         "test-bucket",
		Endpoint:       ts.URL,
		ForcePathStyle: true,
	})
	if err != nil {
		t.Fatalf("NewS3Store() error = %v", err)
	}

	_, err = store.Put(context.Background(), "test.tar.gz", strings.NewReader("data"), 4)
	if err == nil {
		t.Fatal("expected error for upload failure")
	}
	if !strings.Contains(err.Error(), "failed to upload") {
		t.Fatalf("expected upload error, got: %v", err)
	}
}
