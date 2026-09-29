// Package backup creates and restores portable krkn-operator configuration
// archives. Kubernetes client construction remains outside this package so
// callers can provide their own client and authentication setup.
package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// ArchiveDirectoryName is the root directory inside every backup archive.
// Keep this contract stable so krknctl and krkn-operator archives remain
// interchangeable.
const ArchiveDirectoryName = "krkn-backup"

const backupDirName = ArchiveDirectoryName

// BackupConfig controls creation of an operator configuration archive.
type BackupConfig struct {
	// Namespace is the Kubernetes namespace containing operator resources.
	Namespace string
	// OutputDir is where the archive is written. Empty means the current directory.
	OutputDir string
	// BackupName is an optional archive filename, with or without .tar.gz.
	BackupName string
}

type ResourceSpec struct {
	GVR           schema.GroupVersionResource
	Filename      string
	LabelSelector string
	ExcludeNames  map[string]struct{}
}

var resourceSpecs = []ResourceSpec{
	{GVR: schema.GroupVersionResource{Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Resource: "krknusers"}, Filename: "krknuser.json"},
	{GVR: schema.GroupVersionResource{Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Resource: "krknusergroups"}, Filename: "krknusergroup.json"},
	{GVR: schema.GroupVersionResource{Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Resource: "krknoperatortargets"}, Filename: "krknoperatortarget.json"},
	// Provider registrations identify source-cluster providers. They remain in
	// the archive for inventory/compatibility, but restore intentionally leaves
	// them to the destination operator/provider discovery path.
	{GVR: schema.GroupVersionResource{Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Resource: "krknoperatortargetproviders"}, Filename: "krknoperatortargetprovider.json"},
	{GVR: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}, Filename: "auth-secrets.json", LabelSelector: "app.kubernetes.io/component=authentication", ExcludeNames: map[string]struct{}{"krkn-operator-jwt": {}}},
	{GVR: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}, Filename: "user-auth-secrets.json", LabelSelector: "app.kubernetes.io/component=user-auth"},
	{GVR: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}, Filename: "target-secrets.json", LabelSelector: "krkn-target-uuid"},
	{GVR: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}, Filename: "elasticsearch-secrets.json", LabelSelector: "app.kubernetes.io/component=elasticsearch-config"},
	{GVR: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}, Filename: "registry-secrets.json", LabelSelector: "app.kubernetes.io/component=registry"},
	{GVR: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}, Filename: "provider-configmaps.json", LabelSelector: "krkn.krkn-chaos.dev/provider-config=true"},
}

// AllResourceSpecs returns a copy of the operator-compatible inventory.
func AllResourceSpecs() []ResourceSpec {
	result := make([]ResourceSpec, len(resourceSpecs))
	for i, spec := range resourceSpecs {
		result[i] = spec
		if spec.ExcludeNames != nil {
			result[i].ExcludeNames = make(map[string]struct{}, len(spec.ExcludeNames))
			for name := range spec.ExcludeNames {
				result[i].ExcludeNames[name] = struct{}{}
			}
		}
	}
	return result
}

// CreateBackup creates an admin-settings archive from Kubernetes.
func CreateBackup(ctx context.Context, dynClient dynamic.Interface, config BackupConfig) (string, error) {
	namespace := config.Namespace
	if strings.TrimSpace(namespace) == "" {
		return "", fmt.Errorf("namespace must not be empty")
	}
	outputDir := config.OutputDir
	if outputDir == "" {
		outputDir = "."
	}
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	tempParent, err := os.MkdirTemp("", "krkn-backup-tmp-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tempParent)

	backupDir := filepath.Join(tempParent, backupDirName)
	if err := os.Mkdir(backupDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %w", err)
	}

	backupCount := 0
	var failures []string
	for _, spec := range resourceSpecs {
		list, err := listResources(ctx, dynClient, namespace, spec)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", spec.Filename, err))
			continue
		}
		if len(list.Items) == 0 {
			continue
		}

		data, err := buildListJSON(spec.GVR, list)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", spec.Filename, err))
			continue
		}
		if err := os.WriteFile(filepath.Join(backupDir, spec.Filename), data, 0600); err != nil {
			failures = append(failures, fmt.Sprintf("%s: failed to write: %v", spec.Filename, err))
			continue
		}
		backupCount++
	}

	if backupCount == 0 {
		if len(failures) > 0 {
			return "", fmt.Errorf("no resources were backed up: %s", strings.Join(failures, "; "))
		}
		return "", fmt.Errorf("no resources were backed up")
	}
	if len(failures) > 0 {
		return "", fmt.Errorf("backup incomplete (%d resource type(s) failed): %s", len(failures), strings.Join(failures, "; "))
	}

	archiveName := config.BackupName
	if archiveName == "" {
		archiveName = fmt.Sprintf("krkn-backup-%s", time.Now().Format("20060102-150405.000000000"))
	}
	if err := validateBackupName(archiveName); err != nil {
		return "", err
	}
	archivePath := filepath.Join(outputDir, archiveName)
	if !strings.HasSuffix(archiveName, ".tar.gz") {
		archivePath += ".tar.gz"
	}
	tmpFile, err := os.CreateTemp(outputDir, ".krkn-backup-*.tmp")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary archive: %w", err)
	}
	tmpArchive := tmpFile.Name()
	if err := createTarGzFile(tmpFile, filepath.Dir(tempParent), filepath.Base(tempParent)); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpArchive)
		return "", fmt.Errorf("failed to create archive: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpArchive)
		return "", fmt.Errorf("failed to close archive: %w", err)
	}
	// Linking creates the final name without replacing an existing archive.
	// The temporary file and final path are in the same directory, so this is
	// atomic and cannot follow a pre-existing symlink at the final path.
	if err := os.Link(tmpArchive, archivePath); err != nil {
		_ = os.Remove(tmpArchive)
		return "", fmt.Errorf("failed to finalize archive %q: %w", archivePath, err)
	}
	if err := os.Remove(tmpArchive); err != nil {
		return "", fmt.Errorf("failed to remove temporary archive: %w", err)
	}
	return archivePath, nil
}

func validateBackupName(name string) error {
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name || strings.ContainsAny(name, `/\\`) {
		return fmt.Errorf("invalid backup name: must be a single file name")
	}
	return nil
}

// Backup is retained for compatibility with the original krknctl API.
// Deprecated: use CreateBackup with BackupConfig.
func Backup(ctx context.Context, dynClient dynamic.Interface, _ interface{}, namespace, outputDir string) (string, error) {
	return CreateBackup(ctx, dynClient, BackupConfig{Namespace: namespace, OutputDir: outputDir})
}

func listResources(ctx context.Context, dynClient dynamic.Interface, namespace string, spec ResourceSpec) (*unstructured.UnstructuredList, error) {
	list, err := dynClient.Resource(spec.GVR).Namespace(namespace).List(ctx, metav1.ListOptions{LabelSelector: spec.LabelSelector})
	if err != nil {
		return nil, fmt.Errorf("failed to list %s: %w", spec.GVR.Resource, err)
	}

	filtered := &unstructured.UnstructuredList{}
	for _, item := range list.Items {
		if _, excluded := spec.ExcludeNames[item.GetName()]; excluded {
			continue
		}
		filtered.Items = append(filtered.Items, *item.DeepCopy())
	}
	return filtered, nil
}

func buildListJSON(gvr schema.GroupVersionResource, list *unstructured.UnstructuredList) ([]byte, error) {
	items := make([]interface{}, 0, len(list.Items))
	for i := range list.Items {
		items = append(items, stripMetadata(list.Items[i].DeepCopy().Object))
	}
	apiVersion := gvr.Version
	if gvr.Group != "" {
		apiVersion = gvr.Group + "/" + gvr.Version
	}
	return json.MarshalIndent(map[string]interface{}{
		"apiVersion": apiVersion,
		"kind":       "List",
		"items":      items,
	}, "", "  ")
}

func stripMetadata(obj map[string]interface{}) map[string]interface{} {
	// Status is observed state, not portable configuration. It must be
	// recalculated by the destination operator after restoration. A user's
	// active flag is the exception: it is persisted account policy, and the
	// authentication middleware uses it to decide whether login is allowed.
	var userActive bool
	var hasUserActive bool
	if obj["kind"] == "KrknUser" {
		if active, found, err := unstructured.NestedBool(obj, "status", "active"); err == nil && found {
			userActive = active
			hasUserActive = true
		}
	}
	delete(obj, "status")
	if hasUserActive {
		obj["status"] = map[string]interface{}{"active": userActive}
	}
	if metadata, ok := obj["metadata"].(map[string]interface{}); ok {
		for _, field := range []string{"resourceVersion", "uid", "creationTimestamp", "generation", "managedFields"} {
			delete(metadata, field)
		}
		if annotations, ok := metadata["annotations"].(map[string]interface{}); ok {
			delete(annotations, "kubectl.kubernetes.io/last-applied-configuration")
			if len(annotations) == 0 {
				delete(metadata, "annotations")
			}
		}
	}
	return obj
}

// StripMetadata is retained for callers and tests that used the original
// krknctl backup helper.
func StripMetadata(obj map[string]interface{}) {
	stripMetadata(obj)
}

func createTarGzFile(file *os.File, sourceParent, rootName string) error {
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	sourceRoot, err := os.OpenRoot(sourceParent)
	if err != nil {
		return err
	}
	defer sourceRoot.Close()

	archiveRoot, err := sourceRoot.OpenRoot(rootName)
	if err != nil {
		return err
	}
	defer archiveRoot.Close()

	err = fs.WalkDir(archiveRoot.FS(), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink %q is not allowed in backup source", path)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel := filepath.Join(rootName, filepath.FromSlash(path))
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported file type %s in backup source", path)
		}
		input, err := archiveRoot.Open(filepath.FromSlash(path))
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(tarWriter, input)
		closeErr := input.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	if closeErr := tarWriter.Close(); err == nil {
		err = closeErr
	}
	if closeErr := gzipWriter.Close(); err == nil {
		err = closeErr
	}
	return err
}
