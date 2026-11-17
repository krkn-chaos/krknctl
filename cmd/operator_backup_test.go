package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/krkn-chaos/krknctl/pkg/backup"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/rest"
)

func TestOperatorBackupCommand(t *testing.T) {
	cmd := NewOperatorBackupCommand()

	assert.Equal(t, "backup <namespace>", cmd.Use)
	assert.NotContains(t, cmd.Long, "Cloud credentials")

	cmd.SetArgs([]string{})
	assert.Error(t, cmd.Execute())
}

func TestOperatorBackupCommandSuccess(t *testing.T) {
	originalBuildConfig := buildOperatorRESTConfig
	originalDynamicClient := newOperatorDynamicClient
	t.Cleanup(func() {
		buildOperatorRESTConfig = originalBuildConfig
		newOperatorDynamicClient = originalDynamicClient
	})

	buildOperatorRESTConfig = func(_, _ string) (*rest.Config, error) {
		return &rest.Config{}, nil
	}
	newOperatorDynamicClient = func(*rest.Config) (dynamic.Interface, error) {
		return newCommandTestDynamicClient(&unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "krkn.krkn-chaos.dev/v1alpha1",
			"kind":       "KrknUser",
			"metadata": map[string]interface{}{
				"name": "test-user", "namespace": "test-ns",
			},
		}}), nil
	}

	outputDir := t.TempDir()
	cmd := NewOperatorBackupCommand()
	cmd.Flags().String("output", outputDir, "directory for the test archive")
	cmd.SetArgs([]string{"test-ns"})

	assert.NoError(t, cmd.Execute())
	archives, err := filepath.Glob(filepath.Join(outputDir, "*.tar.gz"))
	assert.NoError(t, err)
	assert.Len(t, archives, 1)
}

func TestOperatorRestoreCommand(t *testing.T) {
	cmd := NewOperatorRestoreCommand()

	assert.Equal(t, "restore <archive-path> <namespace>", cmd.Use)
	assert.Contains(t, cmd.Long, "replaces existing operator-managed resources")

	cmd.SetArgs([]string{"backup.tar.gz"})
	assert.Error(t, cmd.Execute())
}

func TestOperatorRestoreCommandSuccess(t *testing.T) {
	originalBuildConfig := buildOperatorRESTConfig
	originalDynamicClient := newOperatorDynamicClient
	t.Cleanup(func() {
		buildOperatorRESTConfig = originalBuildConfig
		newOperatorDynamicClient = originalDynamicClient
	})

	buildOperatorRESTConfig = func(_, _ string) (*rest.Config, error) {
		return &rest.Config{}, nil
	}
	dynClient := newCommandTestDynamicClient()
	newOperatorDynamicClient = func(*rest.Config) (dynamic.Interface, error) {
		return dynClient, nil
	}

	cmd := NewOperatorRestoreCommand()
	cmd.SetArgs([]string{createCommandTestArchive(t), "restore-ns"})

	assert.NoError(t, cmd.Execute())
	secret, err := dynClient.Resource(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}).Namespace("restore-ns").Get(
		cmd.Context(), "restored-secret", metav1.GetOptions{},
	)
	assert.NoError(t, err)
	assert.Equal(t, "restored-secret", secret.GetName())
}

func TestResolveOperatorKubeconfigRejectsConflictingFlags(t *testing.T) {
	_, err := resolveOperatorKubeconfig("/tmp/config", true, "test")

	assert.EqualError(t, err, "--kind and --kubeconfig cannot be used together")
}

func newCommandTestDynamicClient(objects ...runtime.Object) dynamic.Interface {
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		{Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Resource: "krknusers"}:                   "KrknUserList",
		{Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Resource: "krknusergroups"}:              "KrknUserGroupList",
		{Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Resource: "krknoperatortargets"}:         "KrknOperatorTargetList",
		{Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Resource: "krknoperatortargetproviders"}: "KrknOperatorTargetProviderList",
		{Group: "", Version: "v1", Resource: "secrets"}:                                              "SecretList",
		{Group: "", Version: "v1", Resource: "configmaps"}:                                           "ConfigMapList",
	}, objects...)
}

func createCommandTestArchive(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "restore.tar.gz")
	fileData, err := json.Marshal(map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "List",
		"items": []interface{}{map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name": "restored-secret",
				"labels": map[string]interface{}{
					"app.kubernetes.io/component": "authentication",
				},
			},
		}},
	})
	assert.NoError(t, err)

	file, err := os.Create(archivePath)
	assert.NoError(t, err)
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	root := backup.ArchiveDirectoryName + "/"
	assert.NoError(t, tarWriter.WriteHeader(&tar.Header{Name: root, Mode: 0700, Typeflag: tar.TypeDir}))
	name := root + "auth-secrets.json"
	assert.NoError(t, tarWriter.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: int64(len(fileData))}))
	_, err = io.Copy(tarWriter, bytes.NewReader(fileData))
	assert.NoError(t, err)
	assert.NoError(t, tarWriter.Close())
	assert.NoError(t, gzipWriter.Close())
	assert.NoError(t, file.Close())
	return archivePath
}
