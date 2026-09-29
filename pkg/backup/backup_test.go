package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func newFakeDynamicClient(objects ...runtime.Object) *dynamicfake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Resource: "krknusers"}:                   "KrknUserList",
		{Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Resource: "krknusergroups"}:              "KrknUserGroupList",
		{Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Resource: "krknoperatortargets"}:         "KrknOperatorTargetList",
		{Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Resource: "krknoperatortargetproviders"}: "KrknOperatorTargetProviderList",
		{Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Resource: "krknfiletypes"}:               "KrknFileTypeList",
		{Group: "", Version: "v1", Resource: "secrets"}:                                              "SecretList",
		{Group: "", Version: "v1", Resource: "configmaps"}:                                           "ConfigMapList",
	}
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, objects...)
}

func createTarGz(archivePath, sourceParent, rootName string) (retErr error) {
	archiveRoot, err := os.OpenRoot(filepath.Dir(archivePath))
	if err != nil {
		return err
	}
	defer archiveRoot.Close()

	file, err := archiveRoot.OpenFile(filepath.Base(archivePath), os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); retErr == nil && closeErr != nil {
			retErr = closeErr
		}
	}()
	return createTarGzFile(file, sourceParent, rootName)
}

func makeUnstructured(apiVersion, kind, name, namespace string, extra map[string]interface{}) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"name":              name,
				"namespace":         namespace,
				"resourceVersion":   "12345",
				"uid":               "abc-123",
				"creationTimestamp": "2026-01-01T00:00:00Z",
				"generation":        int64(1),
				"managedFields":     []interface{}{},
				"annotations": map[string]interface{}{
					"kubectl.kubernetes.io/last-applied-configuration": "{}",
					"keep-this": "yes",
				},
			},
		},
	}
	for k, v := range extra {
		obj.Object[k] = v
	}
	return obj
}

func TestStripMetadata(t *testing.T) {
	obj := makeUnstructured("v1", "Secret", "test", "ns", nil)
	StripMetadata(obj.Object)

	meta := obj.Object["metadata"].(map[string]interface{})
	assert.Nil(t, meta["resourceVersion"])
	assert.Nil(t, meta["uid"])
	assert.Nil(t, meta["creationTimestamp"])
	assert.Nil(t, meta["generation"])
	assert.Nil(t, meta["managedFields"])
	assert.Equal(t, "ns", meta["namespace"])

	annotations := meta["annotations"].(map[string]interface{})
	assert.Nil(t, annotations["kubectl.kubernetes.io/last-applied-configuration"])
	assert.Equal(t, "yes", annotations["keep-this"])
}

func TestStripMetadata_EmptyAnnotations(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name": "test",
				"annotations": map[string]interface{}{
					"kubectl.kubernetes.io/last-applied-configuration": "{}",
				},
			},
		},
	}
	StripMetadata(obj.Object)
	meta := obj.Object["metadata"].(map[string]interface{})
	assert.Nil(t, meta["annotations"])
}

func TestStripMetadataPreservesKrknUserActiveStatus(t *testing.T) {
	obj := makeUnstructured("krkn.krkn-chaos.dev/v1alpha1", "KrknUser", "user", "ns", map[string]interface{}{
		"status": map[string]interface{}{
			"active":  true,
			"created": "2026-01-01T00:00:00Z",
		},
	})
	StripMetadata(obj.Object)

	status, found, err := unstructured.NestedMap(obj.Object, "status")
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, true, status["active"])
	assert.NotContains(t, status, "created")
}

func TestBuildListJSON(t *testing.T) {
	list := &unstructured.UnstructuredList{
		Items: []unstructured.Unstructured{
			*makeUnstructured("v1", "Secret", "s1", "ns", map[string]interface{}{
				"status": map[string]interface{}{"ready": true},
			}),
		},
	}

	data, err := buildListJSON(schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, list)
	assert.Nil(t, err)

	var result map[string]interface{}
	assert.Nil(t, json.Unmarshal(data, &result))
	assert.Equal(t, "v1", result["apiVersion"])
	assert.Equal(t, "List", result["kind"])

	items := result["items"].([]interface{})
	assert.Equal(t, 1, len(items))

	item := items[0].(map[string]interface{})
	meta := item["metadata"].(map[string]interface{})
	assert.Nil(t, meta["resourceVersion"])
	assert.Nil(t, meta["uid"])
	assert.Nil(t, item["status"])
}

func TestCreateBackup_RejectsEmptyNamespace(t *testing.T) {
	_, err := CreateBackup(context.Background(), newFakeDynamicClient(), BackupConfig{})
	assert.EqualError(t, err, "namespace must not be empty")
}

func TestBackup_CreatesArchive(t *testing.T) {
	user := makeUnstructured("krkn.krkn-chaos.dev/v1alpha1", "KrknUser", "testuser", "test-ns", nil)
	dynClient := newFakeDynamicClient(user)

	outputDir := t.TempDir()
	archivePath, err := CreateBackup(context.Background(), dynClient, BackupConfig{
		Namespace: "test-ns",
		OutputDir: outputDir,
	})
	assert.Nil(t, err)
	assert.NotEmpty(t, archivePath)

	_, statErr := os.Stat(archivePath)
	assert.Nil(t, statErr)

	files := listTarGzFiles(t, archivePath)
	assert.True(t, containsArchivePath(files, "krknuser.json"))
	assert.False(t, containsArchivePath(files, "krkn-operator-jwt"))
}

func TestBackupCompatibilityWrapper(t *testing.T) {
	user := makeUnstructured("krkn.krkn-chaos.dev/v1alpha1", "KrknUser", "testuser", "test-ns", nil)
	archivePath, err := Backup(context.Background(), newFakeDynamicClient(user), nil, "test-ns", t.TempDir())
	assert.NoError(t, err)
	assert.FileExists(t, archivePath)
}

func TestBackupCompatibilityWrapperPropagatesValidationErrors(t *testing.T) {
	_, err := Backup(context.Background(), newFakeDynamicClient(), nil, "", t.TempDir())
	assert.EqualError(t, err, "namespace must not be empty")
}

func TestBackup_SkipsEmptyResources(t *testing.T) {
	dynClient := newFakeDynamicClient()

	outputDir := t.TempDir()
	_, err := CreateBackup(context.Background(), dynClient, BackupConfig{
		Namespace: "empty-ns",
		OutputDir: outputDir,
	})
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "no resources were backed up")
}

func TestBackup_DoesNotOverwriteExistingArchive(t *testing.T) {
	user := makeUnstructured("krkn.krkn-chaos.dev/v1alpha1", "KrknUser", "testuser", "test-ns", nil)
	dynClient := newFakeDynamicClient(user)
	outputDir := t.TempDir()
	backupName := "existing.tar.gz"
	existingPath := filepath.Join(outputDir, backupName)
	assert.NoError(t, os.WriteFile(existingPath, []byte("original"), 0600))

	_, err := CreateBackup(context.Background(), dynClient, BackupConfig{
		Namespace:  "test-ns",
		OutputDir:  outputDir,
		BackupName: backupName,
	})
	assert.Error(t, err)
	data, readErr := os.ReadFile(existingPath)
	assert.NoError(t, readErr)
	assert.Equal(t, []byte("original"), data)
}

func TestRestore_AppliesResources(t *testing.T) {
	archivePath := createTestArchive(t, map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "List",
		"items": []interface{}{
			map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Secret",
				"metadata": map[string]interface{}{
					"name":   "test-secret",
					"labels": map[string]interface{}{"app.kubernetes.io/component": "authentication"},
				},
				"data": map[string]interface{}{"key": "dmFsdWU="},
			},
		},
	}, "auth-secrets.json")

	dynClient := newFakeDynamicClient()
	err := Restore(context.Background(), dynClient, archivePath, "restore-ns")
	assert.Nil(t, err)

	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}
	secret, err := dynClient.Resource(gvr).Namespace("restore-ns").Get(context.Background(), "test-secret", metav1.GetOptions{})
	assert.Nil(t, err)
	assert.Equal(t, "test-secret", secret.GetName())
}

func TestRestore_ReplacesExistingResource(t *testing.T) {
	archivePath := createTestArchive(t, map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "List",
		"items": []interface{}{map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":   "same-name",
				"labels": map[string]interface{}{"app.kubernetes.io/component": "authentication"},
			},
			"data": map[string]interface{}{"new": "bmV3"},
		}},
	}, "auth-secrets.json")

	existing := makeUnstructured("v1", "Secret", "same-name", "restore-ns", map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "same-name",
			"namespace": "restore-ns",
		},
		"data": map[string]interface{}{"old": "b2xk"},
	})
	dynClient := newFakeDynamicClient(existing)
	assert.NoError(t, Restore(context.Background(), dynClient, archivePath, "restore-ns"))

	secret, err := dynClient.Resource(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}).Namespace("restore-ns").Get(context.Background(), "same-name", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"new": "bmV3"}, secret.Object["data"])
}

func TestRestore_PreservesKrknUserActiveStatus(t *testing.T) {
	archivePath := createTestArchive(t, map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "List",
		"items": []interface{}{
			map[string]interface{}{
				"apiVersion": "krkn.krkn-chaos.dev/v1alpha1",
				"kind":       "KrknUser",
				"metadata":   map[string]interface{}{"name": "user1"},
				"spec":       map[string]interface{}{"username": "admin"},
				"status":     map[string]interface{}{"active": true},
			},
		},
	}, "krknuser.json")

	existing := makeUnstructured("krkn.krkn-chaos.dev/v1alpha1", "KrknUser", "user1", "restore-ns", nil)
	dynClient := newFakeDynamicClient(existing)

	err := Restore(context.Background(), dynClient, archivePath, "restore-ns")
	assert.NoError(t, err)

	resource, err := dynClient.Resource(schema.GroupVersionResource{
		Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Resource: "krknusers",
	}).Namespace("restore-ns").Get(context.Background(), "user1", metav1.GetOptions{})
	assert.NoError(t, err)
	status, found, err := unstructured.NestedMap(resource.Object, "status")
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, true, status["active"])
}

func TestRestore_LegacyProviderConfigUsesMigrationHandler(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, backupDirName)
	assert.NoError(t, os.Mkdir(backupDir, 0700))

	secret := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "List",
		"items": []interface{}{map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":   "restore-secret",
				"labels": map[string]interface{}{"app.kubernetes.io/component": "authentication"},
			},
		}},
	}
	provider := map[string]interface{}{
		"apiVersion": "krkn.krkn-chaos.dev/v1alpha1",
		"kind":       "List",
		"items": []interface{}{map[string]interface{}{
			"apiVersion": "krkn.krkn-chaos.dev/v1alpha1",
			"kind":       "KrknOperatorTargetProvider",
			"metadata":   map[string]interface{}{"name": "krkn-operator-acm"},
			"spec":       map[string]interface{}{"operator-name": "krkn-operator-acm", "active": true},
		}},
	}
	providerConfig := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "List",
		"items": []interface{}{map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name": "krkn-operator-acm-config",
				"labels": map[string]interface{}{
					"krkn.krkn-chaos.dev/provider-config": "true",
				},
			},
			"data": map[string]interface{}{
				"ACM_SECRET_LOCAL_CLUSTER": "application-manager",
			},
		}},
	}
	for filename, content := range map[string]interface{}{
		"auth-secrets.json":               secret,
		"krknoperatortargetprovider.json": provider,
		"provider-configmaps.json":        providerConfig,
	} {
		data, err := json.Marshal(content)
		assert.NoError(t, err)
		assert.NoError(t, os.WriteFile(filepath.Join(backupDir, filename), data, 0600))
	}
	archivePath := filepath.Join(tmpDir, "legacy-provider.tar.gz")
	assert.NoError(t, createTarGz(archivePath, tmpDir, backupDirName))

	var migrated []LegacyProviderConfig
	dynClient := newFakeDynamicClient()
	err := RestoreBackup(context.Background(), dynClient, RestoreConfig{
		Namespace:  "restore-ns",
		BackupPath: archivePath,
		LegacyProviderConfigHandler: func(_ context.Context, configs []LegacyProviderConfig) error {
			migrated = configs
			return nil
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, []LegacyProviderConfig{{
		Provider: "krkn-operator-acm",
		Values:   map[string]string{"ACM_SECRET_LOCAL_CLUSTER": "application-manager"},
	}}, migrated)

	_, err = dynClient.Resource(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}).Namespace("restore-ns").Get(context.Background(), "krkn-operator-acm-config", metav1.GetOptions{})
	assert.Error(t, err, "legacy ConfigMap must not be copied directly")
	_, err = dynClient.Resource(schema.GroupVersionResource{Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Resource: "krknoperatortargetproviders"}).Namespace("restore-ns").Get(context.Background(), "krkn-operator-acm", metav1.GetOptions{})
	assert.Error(t, err, "provider registration must not be copied directly")
	_, err = dynClient.Resource(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}).Namespace("restore-ns").Get(context.Background(), "restore-secret", metav1.GetOptions{})
	assert.NoError(t, err, "ordinary resources must still restore")
}

func TestRestore_ProviderOnlyArchiveUsesMigrationHandler(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, backupDirName)
	assert.NoError(t, os.Mkdir(backupDir, 0700))

	providerConfig := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "List",
		"items": []interface{}{map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name": "krkn-operator-acm-config",
				"labels": map[string]interface{}{
					"krkn.krkn-chaos.dev/provider-config": "true",
				},
			},
			"data": map[string]interface{}{
				"ACM_SECRET_LOCAL_CLUSTER": "application-manager",
			},
		}},
	}
	data, err := json.Marshal(providerConfig)
	assert.NoError(t, err)
	assert.NoError(t, os.WriteFile(filepath.Join(backupDir, "provider-configmaps.json"), data, 0600))

	archivePath := filepath.Join(tmpDir, "provider-only.tar.gz")
	assert.NoError(t, createTarGz(archivePath, tmpDir, backupDirName))

	var migrated []LegacyProviderConfig
	err = RestoreBackup(context.Background(), newFakeDynamicClient(), RestoreConfig{
		Namespace:  "restore-ns",
		BackupPath: archivePath,
		LegacyProviderConfigHandler: func(_ context.Context, configs []LegacyProviderConfig) error {
			migrated = configs
			return nil
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, []LegacyProviderConfig{{
		Provider: "krkn-operator-acm",
		Values:   map[string]string{"ACM_SECRET_LOCAL_CLUSTER": "application-manager"},
	}}, migrated)
}

func TestRestore_WarnsWhenProviderSettingsHaveNoMigrationHandler(t *testing.T) {
	archivePath := createTestArchive(t, map[string]interface{}{
		"apiVersion": "v1", "kind": "List", "items": []interface{}{
			map[string]interface{}{
				"apiVersion": "v1", "kind": "ConfigMap",
				"metadata": map[string]interface{}{
					"name":   "krkn-operator-config",
					"labels": map[string]interface{}{"krkn.krkn-chaos.dev/provider-config": "true"},
				},
				"data": map[string]interface{}{"KEY": "value"},
			},
		},
	}, "provider-configmaps.json")

	var warning string
	err := RestoreBackup(context.Background(), newFakeDynamicClient(), RestoreConfig{
		Namespace: "restore-ns", BackupPath: archivePath,
		WarningHandler: func(message string) {
			warning = message
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, "provider settings were not restored: no provider migration handler was provided", warning)
}

func TestRestore_ValidatesEntireArchiveBeforeApplying(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, backupDirName)
	assert.NoError(t, os.Mkdir(backupDir, 0700))

	valid := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "List",
		"items": []interface{}{map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":   "valid-secret",
				"labels": map[string]interface{}{"app.kubernetes.io/component": "authentication"},
			},
		}},
	}
	invalid := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "List",
		"items": []interface{}{map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata":   map[string]interface{}{"name": "not-allowed"},
		}},
	}
	for filename, data := range map[string]interface{}{
		"01-valid.json":   valid,
		"02-invalid.json": invalid,
	} {
		encoded, err := json.Marshal(data)
		assert.NoError(t, err)
		assert.NoError(t, os.WriteFile(filepath.Join(backupDir, filename), encoded, 0600))
	}
	archivePath := filepath.Join(tmpDir, "test-backup.tar.gz")
	assert.NoError(t, createTarGz(archivePath, tmpDir, backupDirName))

	dynClient := newFakeDynamicClient()
	err := Restore(context.Background(), dynClient, archivePath, "restore-ns")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "02-invalid.json")
	_, getErr := dynClient.Resource(schema.GroupVersionResource{
		Group: "", Version: "v1", Resource: "secrets",
	}).Namespace("restore-ns").Get(context.Background(), "valid-secret", metav1.GetOptions{})
	assert.Error(t, getErr)
}

func TestRestore_CleansUpTempDir(t *testing.T) {
	archivePath := createTestArchive(t, map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "List",
		"items": []interface{}{
			map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Secret",
				"metadata":   map[string]interface{}{"name": "s1"},
			},
		},
	}, "auth-secrets.json")

	dynClient := newFakeDynamicClient()
	_ = Restore(context.Background(), dynClient, archivePath, "ns")

	matches, _ := filepath.Glob(filepath.Join(os.TempDir(), "krkn-restore-*"))
	for _, m := range matches {
		t.Errorf("temp directory not cleaned up: %s", m)
	}
}

func TestExtractTarGz_PathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "evil.tar.gz")

	f, err := os.Create(archivePath)
	assert.Nil(t, err)
	gzw := gzip.NewWriter(f)
	tw := tar.NewWriter(gzw)

	hdr := &tar.Header{
		Name: "../../../etc/passwd",
		Mode: 0600,
		Size: 4,
	}
	assert.Nil(t, tw.WriteHeader(hdr))
	_, err = tw.Write([]byte("evil"))
	assert.Nil(t, err)
	tw.Close()
	gzw.Close()
	f.Close()

	destDir := t.TempDir()
	err = extractTarGz(archivePath, destDir)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "path traversal detected")
}

func TestGvrFromObject(t *testing.T) {
	tests := []struct {
		name       string
		apiVersion string
		kind       string
		wantGroup  string
		wantVer    string
		wantRes    string
		wantErr    bool
	}{
		{
			name:       "core secret",
			apiVersion: "v1",
			kind:       "Secret",
			wantGroup:  "",
			wantVer:    "v1",
			wantRes:    "secrets",
		},
		{
			name:       "custom resource",
			apiVersion: "krkn.krkn-chaos.dev/v1alpha1",
			kind:       "KrknUser",
			wantGroup:  "krkn.krkn-chaos.dev",
			wantVer:    "v1alpha1",
			wantRes:    "krknusers",
		},
		{
			name:    "missing apiVersion",
			kind:    "Secret",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := unstructured.Unstructured{Object: map[string]interface{}{}}
			if tt.apiVersion != "" {
				obj.Object["apiVersion"] = tt.apiVersion
			}
			if tt.kind != "" {
				obj.Object["kind"] = tt.kind
			}

			gvr, err := gvrFromObject(obj)
			if tt.wantErr {
				assert.NotNil(t, err)
				return
			}
			assert.Nil(t, err)
			assert.Equal(t, tt.wantGroup, gvr.Group)
			assert.Equal(t, tt.wantVer, gvr.Version)
			assert.Equal(t, tt.wantRes, gvr.Resource)
		})
	}
}

// helpers

func gvrFromObject(obj unstructured.Unstructured) (schema.GroupVersionResource, error) {
	apiVersion := obj.GetAPIVersion()
	if apiVersion == "" || obj.GetKind() == "" {
		return schema.GroupVersionResource{}, fmt.Errorf("missing apiVersion or kind")
	}
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("invalid apiVersion %q: %w", apiVersion, err)
	}
	return schema.GroupVersionResource{Group: gv.Group, Version: gv.Version, Resource: resourceNameForKind(obj.GetKind())}, nil
}

func createTestArchive(t *testing.T, listData map[string]interface{}, filename string) string {
	t.Helper()
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, backupDirName)
	assert.Nil(t, os.Mkdir(backupDir, 0700))

	data, err := json.Marshal(listData)
	assert.Nil(t, err)
	assert.Nil(t, os.WriteFile(filepath.Join(backupDir, filename), data, 0600))

	archivePath := filepath.Join(tmpDir, "test-backup.tar.gz")
	assert.Nil(t, createTarGz(archivePath, tmpDir, backupDirName))
	return archivePath
}

func listTarGzFiles(t *testing.T, archivePath string) []string {
	t.Helper()
	f, err := os.Open(archivePath)
	assert.Nil(t, err)
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	assert.Nil(t, err)
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	var files []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		assert.Nil(t, err)
		if !hdr.FileInfo().IsDir() {
			files = append(files, hdr.Name)
		}
	}
	return files
}

func containsArchivePath(files []string, name string) bool {
	for _, file := range files {
		if strings.HasSuffix(file, "/"+name) || filepath.Base(file) == name {
			return true
		}
	}
	return false
}
