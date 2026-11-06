package backup

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

type staticArchiveReader struct {
	data []byte
}

type testRoundTripper struct {
	handler http.HandlerFunc
}

func (t testRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	recorder := httptest.NewRecorder()
	t.handler(recorder, req)
	return recorder.Result(), nil
}

func setupDownloadTest(t *testing.T, handler http.HandlerFunc) string {
	t.Helper()
	oldTransport := defaultTransport
	oldLookupIP := lookupIP
	defaultTransport = testRoundTripper{handler: handler}
	lookupIP = func(_ context.Context, host string) ([]net.IPAddr, error) {
		switch host {
		case "public.backup.test":
			return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
		case "private.backup.test":
			return []net.IPAddr{{IP: net.ParseIP("10.0.0.1")}}, nil
		case "loopback.backup.test":
			return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
		default:
			return nil, fmt.Errorf("unknown test host %q", host)
		}
	}
	t.Cleanup(func() {
		defaultTransport = oldTransport
		lookupIP = oldLookupIP
	})
	return "https://public.backup.test/backup.tar.gz"
}

func (s staticArchiveReader) Get(context.Context, string) (io.ReadCloser, int64, error) {
	return io.NopCloser(bytes.NewReader(s.data)), int64(len(s.data)), nil
}

func TestDownloadArchiveRejectsHTTP(t *testing.T) {
	_, _, err := DownloadArchive(context.Background(), "http://example.com/backup.tar.gz", 0, 0)
	if err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("expected HTTPS error, got: %v", err)
	}
}

func TestDownloadArchiveRejectsInvalidURL(t *testing.T) {
	_, _, err := DownloadArchive(context.Background(), "not-a-url", 0, 0)
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestDownloadArchiveRejectsEmptyURL(t *testing.T) {
	_, _, err := DownloadArchive(context.Background(), "", 0, 0)
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestDownloadArchiveRejectsEmbeddedCredentials(t *testing.T) {
	_, _, err := DownloadArchive(context.Background(), "https://user:password@example.com/backup.tar.gz", 0, 0)
	if err == nil || !strings.Contains(err.Error(), "credentials") {
		t.Fatalf("expected embedded credentials error, got: %v", err)
	}
}

func TestDownloadArchiveRejectsPrivateHostname(t *testing.T) {
	setupDownloadTest(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	_, _, err := DownloadArchive(context.Background(), "https://private.backup.test/backup.tar.gz", 0, time.Second)
	if err == nil || !strings.Contains(err.Error(), "private or local") {
		t.Fatalf("expected private-address error, got: %v", err)
	}
}

func TestDownloadArchiveRejectsUnsafeRedirect(t *testing.T) {
	url := setupDownloadTest(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "https://private.backup.test/backup.tar.gz")
		w.WriteHeader(http.StatusFound)
	})
	_, _, err := DownloadArchive(context.Background(), url, 0, time.Second)
	if err == nil || !strings.Contains(err.Error(), "unsafe redirect") {
		t.Fatalf("expected unsafe redirect error, got: %v", err)
	}
}

func TestDownloadArchiveSuccess(t *testing.T) {
	payload := []byte("fake-archive-data-1234567890")
	url := setupDownloadTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		w.Write(payload)
	})

	path, cleanup, err := DownloadArchive(context.Background(), url, 0, 10*time.Second)
	if err != nil {
		t.Fatalf("DownloadArchive() error = %v", err)
	}
	if cleanup == nil {
		t.Fatal("cleanup function is nil on success")
	}
	defer cleanup()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}
	if string(data) != string(payload) {
		t.Errorf("downloaded content = %q, want %q", data, payload)
	}

	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("cleanup did not remove the temporary file")
	}
}

func TestDownloadArchiveRejectsOversizedResponse(t *testing.T) {
	url := setupDownloadTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write(make([]byte, 1024))
	})

	_, _, err := DownloadArchive(context.Background(), url, 512, 10*time.Second)
	if err == nil {
		t.Fatal("expected error for oversized response")
	}
	if !strings.Contains(err.Error(), "exceeds maximum size") {
		t.Fatalf("expected size error, got: %v", err)
	}
}

func TestDownloadArchiveRejectsNon200(t *testing.T) {
	url := setupDownloadTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, _, err := DownloadArchive(context.Background(), url, 0, 10*time.Second)
	if err == nil {
		t.Fatal("expected error for non-200 response")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Fatalf("expected HTTP 404 error, got: %v", err)
	}
}

func TestDownloadArchiveFromStore(t *testing.T) {
	payload := []byte("fake-s3-archive")
	path, cleanup, err := DownloadArchiveFromStore(
		context.Background(),
		staticArchiveReader{data: payload},
		"krkn-operator/backups/archive.tar.gz",
		0,
		10*time.Second,
	)
	if err != nil {
		t.Fatalf("DownloadArchiveFromStore() error = %v", err)
	}
	defer cleanup()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}
	if string(data) != string(payload) {
		t.Errorf("downloaded content = %q, want %q", data, payload)
	}
}

func TestDownloadArchiveFromStoreRejectsOversizedObject(t *testing.T) {
	_, cleanup, err := DownloadArchiveFromStore(
		context.Background(),
		staticArchiveReader{data: make([]byte, 1024)},
		"krkn-operator/backups/archive.tar.gz",
		512,
		10*time.Second,
	)
	if err == nil {
		t.Fatal("expected error for oversized S3 object")
	}
	if cleanup != nil {
		t.Fatal("cleanup should be nil when the object is rejected before download")
	}
}

func TestDownloadArchiveCleansUpOnError(t *testing.T) {
	url := setupDownloadTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write(make([]byte, 2048))
	})

	_, cleanup, err := DownloadArchive(context.Background(), url, 1024, 10*time.Second)
	if err == nil {
		t.Fatal("expected error for oversized response")
	}
	if cleanup != nil {
		t.Error("cleanup should be nil on error")
	}
}
