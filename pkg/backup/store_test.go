package backup

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/krkn-chaos/krknctl/pkg/backupstore"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type fakeStore struct {
	location backupstore.Location
	uploaded []byte
}

func (s *fakeStore) Put(_ context.Context, _ string, source io.Reader, _ int64) (backupstore.Location, error) {
	data, err := io.ReadAll(source)
	if err != nil {
		return backupstore.Location{}, err
	}
	s.uploaded = data
	return s.location, nil
}

func TestCreateBackupAndStoreNilStore(t *testing.T) {
	dynClient := newFakeDynamicClient()
	_, err := dynClient.Resource(schema.GroupVersionResource{
		Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Resource: "krknusers",
	}).Namespace("default").Create(context.Background(), &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "krkn.krkn-chaos.dev/v1alpha1",
		"kind":       "KrknUser",
		"metadata":   map[string]interface{}{"name": "test-user", "namespace": "default"},
	}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Create fixture: %v", err)
	}

	result, err := CreateBackupAndStore(context.Background(), dynClient, BackupConfig{
		Namespace: "default",
		OutputDir: t.TempDir(),
	}, nil)
	if err != nil {
		t.Fatalf("CreateBackupAndStore() error = %v", err)
	}
	if result.ArchivePath == "" {
		t.Error("expected non-empty archive path")
	}
	if result.Location != nil {
		t.Error("expected nil location when store is nil")
	}
	defer os.Remove(result.ArchivePath)
}

func TestCreateBackupAndStoreWithStore(t *testing.T) {
	dynClient := newFakeDynamicClient()
	_, err := dynClient.Resource(schema.GroupVersionResource{
		Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Resource: "krknusers",
	}).Namespace("default").Create(context.Background(), &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "krkn.krkn-chaos.dev/v1alpha1",
		"kind":       "KrknUser",
		"metadata":   map[string]interface{}{"name": "test-user", "namespace": "default"},
	}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Create fixture: %v", err)
	}

	store := &fakeStore{location: backupstore.Location{
		Key:         "backups/test.tar.gz",
		DownloadURL: "https://s3.example.test/test.tar.gz?sig=abc",
	}}

	result, err := CreateBackupAndStore(context.Background(), dynClient, BackupConfig{
		Namespace: "default",
		OutputDir: t.TempDir(),
	}, store)
	if err != nil {
		t.Fatalf("CreateBackupAndStore() error = %v", err)
	}
	if result.Location == nil {
		t.Fatal("expected non-nil location")
	}
	if result.Location.DownloadURL != store.location.DownloadURL {
		t.Errorf("download URL = %q, want %q", result.Location.DownloadURL, store.location.DownloadURL)
	}
	if len(store.uploaded) == 0 {
		t.Error("store received empty upload")
	}
	defer os.Remove(result.ArchivePath)
}
