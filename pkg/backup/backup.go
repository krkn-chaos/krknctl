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
	// Passphrase, when non-empty, encrypts the archive with AES-256-GCM.
	// The resulting file has the additional .enc suffix.
	Passphrase string
}

type ResourceSpec struct {
	GVR           schema.GroupVersionResource
	Filename      string
	LabelSelector string
	ExcludeNames  map[string]struct{}
	// StatusPhases limits this resource to objects whose status.phase matches
	// one of these values. It is used for immutable run history so active runs
	// are not captured.
	StatusPhases []string
	// PreserveStatus keeps the status section in the archive. Run history is
	// historical data, so its phase, timing, and scores must be retained.
	PreserveStatus bool
}

var resourceSpecs = []ResourceSpec{
	{GVR: schema.GroupVersionResource{Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Resource: "krknusers"}, Filename: "krknuser.json"},
	{GVR: schema.GroupVersionResource{Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Resource: "krknusergroups"}, Filename: "krknusergroup.json"},
	{GVR: schema.GroupVersionResource{Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Resource: "krkncategories"}, Filename: "krkncategory.json"},
	{GVR: schema.GroupVersionResource{Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Resource: "krknoperatortargets"}, Filename: "krknoperatortarget.json"},
	{GVR: schema.GroupVersionResource{Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Resource: "krknscenarioruns"}, Filename: "krknscenariorun.json", StatusPhases: []string{"Succeeded", "PartiallyFailed", "Failed"}, PreserveStatus: true},
	{GVR: schema.GroupVersionResource{Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Resource: "krkngraphruns"}, Filename: "krkngraphrun.json", StatusPhases: []string{"Completed", "Failed", "PartiallyFailed"}, PreserveStatus: true},
	// Provider registrations identify source-cluster providers. They remain in
	// the archive for inventory/compatibility, but restore intentionally leaves
	// them to the destination operator/provider discovery path.
	{GVR: schema.GroupVersionResource{Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Resource: "krknoperatortargetproviders"}, Filename: "krknoperatortargetprovider.json"},
	{GVR: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}, Filename: "auth-secrets.json", LabelSelector: "app.kubernetes.io/component=authentication", ExcludeNames: map[string]struct{}{"krkn-operator-jwt": {}}},
	{GVR: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}, Filename: "user-auth-secrets.json", LabelSelector: "app.kubernetes.io/component=user-auth"},
	{GVR: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}, Filename: "target-secrets.json", LabelSelector: "krkn-target-uuid"},
	{GVR: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}, Filename: "elasticsearch-secrets.json", LabelSelector: "app.kubernetes.io/component=elasticsearch-config"},
	{GVR: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}, Filename: "registry-secrets.json", LabelSelector: "app.kubernetes.io/component=registry"},
	{GVR: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}, Filename: "cloud-credential-secrets.json", LabelSelector: "app.kubernetes.io/name=krkn-operator,app.kubernetes.io/component=cloud-credential"},
	{GVR: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}, Filename: "provider-configmaps.json", LabelSelector: "krkn.krkn-chaos.dev/provider-config=true"},
	{GVR: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}, Filename: "workflow-configmaps.json", LabelSelector: "files.krkn.krkn-chaos.dev/file-purpose=workflow-template"},
	{GVR: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}, Filename: "report-configmaps.json", LabelSelector: "app=krkn-operator,component=reports"},
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
		result[i].StatusPhases = append([]string(nil), spec.StatusPhases...)
	}
	return result
}

// CreateBackup creates an admin-settings archive from Kubernetes.
func CreateBackup(ctx context.Context, dynClient dynamic.Interface, config BackupConfig) (resultPath string, retErr error) {
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
	defer func() {
		if cleanupErr := os.RemoveAll(tempParent); cleanupErr != nil {
			if retErr == nil {
				resultPath = ""
				retErr = fmt.Errorf("failed to clean up temporary backup data: %w", cleanupErr)
				return
			}
			retErr = fmt.Errorf("%w; failed to clean up temporary backup data: %v", retErr, cleanupErr)
		}
	}()

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

		data, err := buildListJSONForSpec(spec, list)
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
	if err := os.Remove(tmpArchive); err != nil && !os.IsNotExist(err) {
		if cleanupErr := os.Remove(archivePath); cleanupErr != nil && !os.IsNotExist(cleanupErr) {
			return "", fmt.Errorf("failed to remove temporary archive: %w; failed to remove finalized archive: %v", err, cleanupErr)
		}
		return "", fmt.Errorf("failed to remove temporary archive: %w", err)
	}

	if config.Passphrase != "" {
		encPath := archivePath + encryptedSuffix
		if err := EncryptFile(archivePath, encPath, config.Passphrase); err != nil {
			// Do not return or retain a usable plaintext backup when encryption
			// fails. The caller may otherwise print the returned path and expose
			// an unencrypted archive containing credentials.
			if removeErr := os.Remove(archivePath); removeErr != nil && !os.IsNotExist(removeErr) {
				return "", fmt.Errorf("encryption failed: %w; failed to remove unencrypted archive: %v", err, removeErr)
			}
			return "", fmt.Errorf("encryption failed: %w", err)
		}
		if err := os.Remove(archivePath); err != nil {
			return "", fmt.Errorf("encrypted backup created at %q, but failed to remove plaintext archive %q: %w", encPath, archivePath, err)
		}
		return encPath, nil
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
		if len(spec.StatusPhases) > 0 {
			phase, found, err := unstructured.NestedString(item.Object, "status", "phase")
			if err != nil || !found || !containsString(spec.StatusPhases, phase) {
				continue
			}
		}
		filtered.Items = append(filtered.Items, *item.DeepCopy())
	}
	return filtered, nil
}

func buildListJSON(gvr schema.GroupVersionResource, list *unstructured.UnstructuredList) ([]byte, error) {
	return buildListJSONWithStatus(gvr, list, false)
}

func buildListJSONForSpec(spec ResourceSpec, list *unstructured.UnstructuredList) ([]byte, error) {
	return buildListJSONWithStatus(spec.GVR, list, spec.PreserveStatus)
}

func buildListJSONWithStatus(gvr schema.GroupVersionResource, list *unstructured.UnstructuredList, preserveStatus bool) ([]byte, error) {
	items := make([]interface{}, 0, len(list.Items))
	for i := range list.Items {
		object := list.Items[i].DeepCopy().Object
		stripMetadataForArchive(object, preserveStatus)
		items = append(items, object)
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
	stripMetadataForArchive(obj, false)
	return obj
}

func stripMetadataForArchive(obj map[string]interface{}, preserveStatus bool) map[string]interface{} {
	// Status is observed state, not portable configuration. It must be
	// recalculated by the destination operator after restoration. A user's
	// active flag is the exception: it is persisted account policy, and the
	// authentication middleware uses it to decide whether login is allowed.
	if !preserveStatus {
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
	}
	if metadata, ok := obj["metadata"].(map[string]interface{}); ok {
		for _, field := range []string{"resourceVersion", "uid", "creationTimestamp", "generation", "managedFields", "ownerReferences"} {
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

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
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
