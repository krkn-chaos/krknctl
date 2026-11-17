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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var allowedRestoreGVKs = map[schema.GroupVersionKind]struct{}{
	{Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Kind: "KrknUser"}:                   {},
	{Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Kind: "KrknUserGroup"}:              {},
	{Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Kind: "KrknOperatorTarget"}:         {},
	{Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Kind: "KrknOperatorTargetProvider"}: {},
	{Group: "", Version: "v1", Kind: "Secret"}:                                              {},
	{Group: "", Version: "v1", Kind: "ConfigMap"}:                                           {},
}

var expectedArchiveGVKs = map[string]schema.GroupVersionKind{
	"krknuser.json":                   {Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Kind: "KrknUser"},
	"krknusergroup.json":              {Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Kind: "KrknUserGroup"},
	"krknoperatortarget.json":         {Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Kind: "KrknOperatorTarget"},
	"krknoperatortargetprovider.json": {Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Kind: "KrknOperatorTargetProvider"},
	"auth-secrets.json":               {Group: "", Version: "v1", Kind: "Secret"},
	"user-auth-secrets.json":          {Group: "", Version: "v1", Kind: "Secret"},
	"target-secrets.json":             {Group: "", Version: "v1", Kind: "Secret"},
	"elasticsearch-secrets.json":      {Group: "", Version: "v1", Kind: "Secret"},
	"registry-secrets.json":           {Group: "", Version: "v1", Kind: "Secret"},
	"provider-configmaps.json":        {Group: "", Version: "v1", Kind: "ConfigMap"},
}

const (
	maxExtractFileSize     = 256 << 20
	maxCumulativeExtracted = 1 << 30
)

// RestoreConfig controls restoration of an operator configuration archive.
type RestoreConfig struct {
	// Namespace is the destination namespace for restored resources.
	Namespace string
	// BackupPath is the .tar.gz archive to restore.
	BackupPath string
	// ArchivePath is retained as a compatibility alias for BackupPath.
	// Deprecated: use BackupPath.
	ArchivePath string
	// LegacyProviderConfigHandler migrates provider settings from the legacy
	// provider-configmaps.json archive section. The handler receives only a
	// stable provider identity and logical key/value data; source ConfigMap
	// names and namespaces are intentionally not passed through.
	LegacyProviderConfigHandler LegacyProviderConfigHandler
	// WarningHandler receives non-fatal restore warnings, such as provider
	// settings that cannot be restored without an operator migration handler or
	// target credentials that could not be checked.
	WarningHandler RestoreWarningHandler
	// RefreshTargetReadiness checks restored target kubeconfigs against their
	// destination cluster API servers and writes the resulting readiness to
	// status. It is disabled by default; the CLI and operator restore handlers
	// enable it explicitly.
	RefreshTargetReadiness bool
}

// RestoreWarningHandler reports a non-fatal restore warning to the caller.
type RestoreWarningHandler func(string)

func warnRestore(warn RestoreWarningHandler, message string) {
	if warn != nil {
		warn(message)
	}
}

// LegacyProviderConfig is the normalized form of one legacy provider
// ConfigMap. It exists only for restore-time migration of old archives.
type LegacyProviderConfig struct {
	Provider string
	Values   map[string]string
}

// LegacyProviderConfigHandler overrides the package's default destination-
// aware migration of legacy provider settings. It is retained so an operator
// integration can supply its own provider discovery implementation.
type LegacyProviderConfigHandler func(context.Context, []LegacyProviderConfig) error

// RestoreBackup applies an operator admin-settings archive directly through
// the Kubernetes API. The operator process itself does not need to be running.
func RestoreBackup(ctx context.Context, dynClient dynamic.Interface, config RestoreConfig) error {
	if strings.TrimSpace(config.Namespace) == "" {
		return fmt.Errorf("namespace must not be empty")
	}
	archivePath := config.BackupPath
	if archivePath == "" {
		archivePath = config.ArchivePath
	}
	if strings.TrimSpace(archivePath) == "" {
		return fmt.Errorf("backup path must not be empty")
	}
	namespace := config.Namespace
	extractDir, err := os.MkdirTemp("", "krkn-restore-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(extractDir)

	if err := extractTarGz(archivePath, extractDir); err != nil {
		return fmt.Errorf("failed to extract archive: %w", err)
	}
	backupDir, err := findBackupDir(extractDir)
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return fmt.Errorf("failed to read backup directory: %w", err)
	}

	var jsonFiles []string
	var validationFailures []string
	var legacyProviderConfigs []LegacyProviderConfig
	seenLegacyProviders := make(map[string]struct{})
	targetArchivePresent := false
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		filePath := filepath.Join(backupDir, entry.Name())
		if entry.Name() == "krknoperatortargetprovider.json" || entry.Name() == "krknoperatortargetproviders.json" {
			// Provider registration is source-cluster state. It must not be
			// copied to the destination during legacy migration.
			continue
		}
		if entry.Name() == "provider-configmaps.json" {
			objects, err := readListFile(filePath)
			if err != nil {
				return fmt.Errorf("%s: %w", entry.Name(), err)
			}
			for _, object := range objects {
				if err := validateRestoreObjectForFile(entry.Name(), object); err != nil {
					validationFailures = append(validationFailures, formatRestoreFailure(entry.Name(), object, err))
					continue
				}
				providerName, ok := legacyProviderNameForConfigMap(object.GetName())
				if !ok {
					continue
				}
				if _, exists := seenLegacyProviders[providerName]; exists {
					validationFailures = append(validationFailures, fmt.Sprintf("%s provider %q appears more than once", entry.Name(), providerName))
					continue
				}
				values, found, err := unstructured.NestedStringMap(object.Object, "data")
				if err != nil {
					validationFailures = append(validationFailures, formatRestoreFailure(entry.Name(), object, fmt.Errorf("invalid provider settings data: %w", err)))
					continue
				}
				if !found {
					values = map[string]string{}
				}
				seenLegacyProviders[providerName] = struct{}{}
				legacyProviderConfigs = append(legacyProviderConfigs, LegacyProviderConfig{
					Provider: providerName,
					Values:   values,
				})
			}
			continue
		}
		objects, err := readListFile(filePath)
		if err != nil {
			return fmt.Errorf("%s: %w", entry.Name(), err)
		}
		if entry.Name() == "krknoperatortarget.json" {
			targetArchivePresent = len(objects) > 0
		}
		for _, object := range objects {
			if err := validateRestoreObjectForFile(entry.Name(), object); err != nil {
				validationFailures = append(validationFailures, formatRestoreFailure(entry.Name(), object, err))
			}
		}
		jsonFiles = append(jsonFiles, filePath)
	}

	if len(validationFailures) > 0 {
		return fmt.Errorf("archive validation failed: %s", strings.Join(validationFailures, "; "))
	}
	var restoreFailures []string
	applied := 0
	for _, filePath := range jsonFiles {
		objects, err := readListFile(filePath)
		if err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(filePath), err)
		}
		for _, object := range objects {
			object.SetNamespace(namespace)
			if err := applyResource(ctx, dynClient, namespace, &object); err != nil {
				restoreFailures = append(restoreFailures, formatRestoreFailure(filepath.Base(filePath), object, err))
				continue
			}
			applied++
		}
	}
	if len(restoreFailures) > 0 {
		return fmt.Errorf("restore completed with %d resource failures out of %d total: %s", len(restoreFailures), applied+len(restoreFailures), strings.Join(restoreFailures, "; "))
	}
	if len(legacyProviderConfigs) > 0 && config.LegacyProviderConfigHandler != nil {
		if err := config.LegacyProviderConfigHandler(ctx, legacyProviderConfigs); err != nil {
			return fmt.Errorf("legacy provider settings restore failed: %w", err)
		}
	} else if len(legacyProviderConfigs) > 0 {
		warnRestore(config.WarningHandler, "provider settings were not restored: no provider migration handler was provided")
	}
	if config.RefreshTargetReadiness && targetArchivePresent {
		if err := RefreshRestoredTargetReadiness(ctx, dynClient, namespace, config.WarningHandler); err != nil {
			return fmt.Errorf("restored target readiness refresh failed: %w", err)
		}
	}
	if applied == 0 && len(legacyProviderConfigs) == 0 {
		return fmt.Errorf("no resources were restored")
	}
	return nil
}

// Restore is retained for compatibility with the original krknctl API.
// Deprecated: use RestoreBackup with RestoreConfig.
func Restore(ctx context.Context, dynClient dynamic.Interface, archivePath, namespace string) error {
	return RestoreBackup(ctx, dynClient, RestoreConfig{BackupPath: archivePath, Namespace: namespace})
}

func validateRestoreObject(obj unstructured.Unstructured) error {
	gvk := obj.GroupVersionKind()
	if _, ok := allowedRestoreGVKs[gvk]; !ok {
		return fmt.Errorf("resource type %s is not allowed", gvk.String())
	}
	if obj.GetName() == "" {
		return fmt.Errorf("resource is missing metadata.name")
	}
	if obj.IsList() {
		return fmt.Errorf("nested list resource is not allowed")
	}
	if obj.GetKind() == "Secret" {
		if obj.GetName() == "krkn-operator-jwt" {
			return fmt.Errorf("JWT signing Secret cannot be restored")
		}
		labels := obj.GetLabels()
		if labels == nil || (labels["krkn-target-uuid"] == "" &&
			labels["app.kubernetes.io/component"] != "authentication" &&
			labels["app.kubernetes.io/component"] != "user-auth" &&
			labels["app.kubernetes.io/component"] != "elasticsearch-config" &&
			labels["app.kubernetes.io/component"] != "registry") {
			return fmt.Errorf("secret %q is not operator-managed", obj.GetName())
		}
	}
	if obj.GetKind() == "ConfigMap" {
		labels := obj.GetLabels()
		if labels == nil || labels["krkn.krkn-chaos.dev/provider-config"] != "true" {
			return fmt.Errorf("ConfigMap %q is not an operator provider configuration", obj.GetName())
		}
	}
	return nil
}

func validateRestoreObjectForFile(filename string, obj unstructured.Unstructured) error {
	expected, ok := expectedArchiveGVKs[filename]
	if !ok {
		return fmt.Errorf("archive file %q is not supported", filename)
	}
	if obj.GroupVersionKind() != expected {
		return fmt.Errorf("resource type %s does not match archive file %q", obj.GroupVersionKind().String(), filename)
	}
	return validateRestoreObject(obj)
}

func applyResource(ctx context.Context, dynClient dynamic.Interface, namespace string, obj *unstructured.Unstructured) error {
	gvk := obj.GroupVersionKind()
	gvr := schema.GroupVersionResource{Group: gvk.Group, Version: gvk.Version, Resource: resourceNameForKind(gvk.Kind)}
	resource := dynClient.Resource(gvr).Namespace(namespace)
	var preservedUserStatus map[string]interface{}
	if gvk.Kind == "KrknUser" {
		if active, found, err := unstructured.NestedBool(obj.Object, "status", "active"); err == nil && found {
			preservedUserStatus = map[string]interface{}{"active": active}
		}
	}
	// Status is destination-observed state and must never be copied from an
	// archive, including a manually constructed archive. KrknUser.active is
	// restored separately below because it is account policy, not observed
	// runtime state.
	delete(obj.Object, "status")

	existing, getErr := resource.Get(ctx, obj.GetName(), metav1.GetOptions{})
	if getErr != nil && !apierrors.IsNotFound(getErr) {
		return fmt.Errorf("failed to check existing resource %s/%s: %w", gvr.Resource, obj.GetName(), getErr)
	}
	if apierrors.IsNotFound(getErr) {
		created, err := resource.Create(ctx, obj, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create %s/%s: %w", gvr.Resource, obj.GetName(), err)
		}
		obj = created
	} else {
		obj.SetResourceVersion(existing.GetResourceVersion())
		obj.SetUID(existing.GetUID())
		updated, err := resource.Update(ctx, obj, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update %s/%s: %w", gvr.Resource, obj.GetName(), err)
		}
		obj = updated
	}

	if preservedUserStatus != nil {
		if err := unstructured.SetNestedMap(obj.Object, preservedUserStatus, "status"); err != nil {
			return fmt.Errorf("failed to prepare %s/%s status: %w", gvr.Resource, obj.GetName(), err)
		}
		if _, err := resource.UpdateStatus(ctx, obj, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("failed to restore %s/%s status: %w", gvr.Resource, obj.GetName(), err)
		}
	}

	return nil
}

func formatRestoreFailure(filename string, obj unstructured.Unstructured, err error) string {
	kind := obj.GetKind()
	if kind == "" {
		kind = "resource"
	}
	name := obj.GetName()
	if name == "" {
		name = "<unnamed>"
	}
	return fmt.Sprintf("%s %s/%s: %v", filename, kind, name, err)
}

func resourceNameForKind(kind string) string {
	switch kind {
	case "KrknUser":
		return "krknusers"
	case "KrknUserGroup":
		return "krknusergroups"
	case "KrknOperatorTarget":
		return "krknoperatortargets"
	case "KrknOperatorTargetProvider":
		return "krknoperatortargetproviders"
	case "Secret":
		return "secrets"
	case "ConfigMap":
		return "configmaps"
	default:
		return strings.ToLower(kind) + "s"
	}
}

func readListFile(filePath string) ([]unstructured.Unstructured, error) {
	backupRoot, err := os.OpenRoot(filepath.Dir(filePath))
	if err != nil {
		return nil, fmt.Errorf("failed to open backup directory for %s: %w", filepath.Base(filePath), err)
	}
	defer backupRoot.Close()

	data, err := backupRoot.ReadFile(filepath.Base(filePath))
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", filepath.Base(filePath), err)
	}
	var wrapper struct {
		APIVersion string                   `json:"apiVersion"`
		Kind       string                   `json:"kind"`
		Items      []map[string]interface{} `json:"items"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", filepath.Base(filePath), err)
	}
	if wrapper.Kind != "List" || wrapper.APIVersion == "" {
		return nil, fmt.Errorf("%s must be a Kubernetes List with apiVersion", filepath.Base(filePath))
	}
	items := make([]unstructured.Unstructured, 0, len(wrapper.Items))
	for _, item := range wrapper.Items {
		items = append(items, unstructured.Unstructured{Object: item})
	}
	return items, nil
}

// legacyProviderNameForConfigMap maps only the provider ConfigMap names that
// were part of the old, known archive contract. This is a migration hint, not
// a destination location: the operator still discovers the destination
// provider and its ConfigMap independently.
func legacyProviderNameForConfigMap(name string) (string, bool) {
	switch name {
	case "krkn-operator-acm-config":
		return "krkn-operator-acm", true
	case "krkn-operator-config":
		return "krkn-operator", true
	default:
		return "", false
	}
}

func extractTarGz(archivePath, destDir string) error {
	archiveRoot, err := os.OpenRoot(filepath.Dir(archivePath))
	if err != nil {
		return err
	}
	defer archiveRoot.Close()

	file, err := archiveRoot.Open(filepath.Base(archivePath))
	if err != nil {
		return err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzipReader.Close()
	destinationRoot, err := os.OpenRoot(destDir)
	if err != nil {
		return err
	}
	defer destinationRoot.Close()

	tarReader := tar.NewReader(gzipReader)
	cumulative, files := int64(0), 0
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("error reading tar: %w", err)
		}
		relPath := filepath.Clean(filepath.FromSlash(header.Name))
		if header.Name == "" || filepath.IsAbs(relPath) || relPath == "." || relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
			return fmt.Errorf("invalid path in archive (path traversal detected): %s", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := destinationRoot.MkdirAll(relPath, 0700); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := destinationRoot.MkdirAll(filepath.Dir(relPath), 0700); err != nil {
				return err
			}
			output, err := destinationRoot.OpenFile(relPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
			if err != nil {
				return err
			}
			written, copyErr := io.Copy(output, io.LimitReader(tarReader, maxExtractFileSize+1))
			closeErr := output.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
			if written > maxExtractFileSize {
				return fmt.Errorf("file %s exceeds maximum allowed size", filepath.Base(relPath))
			}
			cumulative += written
			files++
			if cumulative > maxCumulativeExtracted {
				return fmt.Errorf("archive exceeds maximum extracted size (1 GB), files=%d", files)
			}
		}
	}
}

func findBackupDir(tempDir string) (string, error) {
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return "", fmt.Errorf("failed to read extracted archive: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if entry.Name() == backupDirName {
			return filepath.Join(tempDir, entry.Name()), nil
		}
		if strings.HasPrefix(entry.Name(), "krkn-backup-tmp-") {
			candidate := filepath.Join(tempDir, entry.Name(), backupDirName)
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				return candidate, nil
			}
		}
	}
	return "", fmt.Errorf("backup directory not found in archive")
}
