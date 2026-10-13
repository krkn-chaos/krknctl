package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	"k8s.io/client-go/util/retry"
)

var allowedRestoreGVKs = map[schema.GroupVersionKind]struct{}{
	{Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Kind: "KrknUser"}:                   {},
	{Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Kind: "KrknUserGroup"}:              {},
	{Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Kind: "KrknCategory"}:               {},
	{Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Kind: "KrknOperatorTarget"}:         {},
	{Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Kind: "KrknOperatorTargetProvider"}: {},
	{Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Kind: "KrknScenarioRun"}:            {},
	{Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Kind: "KrknGraphRun"}:               {},
	{Group: "", Version: "v1", Kind: "Secret"}:                                              {},
	{Group: "", Version: "v1", Kind: "ConfigMap"}:                                           {},
}

var expectedArchiveGVKs = map[string]schema.GroupVersionKind{
	"krknuser.json":                   {Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Kind: "KrknUser"},
	"krknusergroup.json":              {Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Kind: "KrknUserGroup"},
	"krkncategory.json":               {Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Kind: "KrknCategory"},
	"krknoperatortarget.json":         {Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Kind: "KrknOperatorTarget"},
	"krknoperatortargetprovider.json": {Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Kind: "KrknOperatorTargetProvider"},
	"krknscenariorun.json":            {Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Kind: "KrknScenarioRun"},
	"krkngraphrun.json":               {Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Kind: "KrknGraphRun"},
	"auth-secrets.json":               {Group: "", Version: "v1", Kind: "Secret"},
	"user-auth-secrets.json":          {Group: "", Version: "v1", Kind: "Secret"},
	"target-secrets.json":             {Group: "", Version: "v1", Kind: "Secret"},
	"elasticsearch-secrets.json":      {Group: "", Version: "v1", Kind: "Secret"},
	"registry-secrets.json":           {Group: "", Version: "v1", Kind: "Secret"},
	"cloud-credential-secrets.json":   {Group: "", Version: "v1", Kind: "Secret"},
	"provider-configmaps.json":        {Group: "", Version: "v1", Kind: "ConfigMap"},
	"workflow-configmaps.json":        {Group: "", Version: "v1", Kind: "ConfigMap"},
	"report-configmaps.json":          {Group: "", Version: "v1", Kind: "ConfigMap"},
}

const (
	maxExtractFileSize     = 256 << 20
	maxCumulativeExtracted = 1 << 30

	workflowConfigMapsFilename = "workflow-configmaps.json"
	reportConfigMapsFilename   = "report-configmaps.json"
	workflowFilePurposeLabel   = "files.krkn.krkn-chaos.dev/file-purpose"
	workflowFilePurpose        = "workflow-template"
	workflowNameAnnotation     = "files.krkn.krkn-chaos.dev/workflow-name"
	workflowFileIDLabel        = "files.krkn.krkn-chaos.dev/file-id"
	fileReservationComponent   = "file-reservation"
	// HistoryOnlyAnnotation prevents an operator controller from treating a
	// restored terminal run as a new execution. The operator consumes this
	// marker; krknctl still restores the CR directly when the operator is down.
	HistoryOnlyAnnotation = "krkn.krkn-chaos.dev/history-only"
)

// RestoreConfig controls restoration of an operator configuration archive.
type RestoreConfig struct {
	// Namespace is the destination namespace for restored resources.
	Namespace string
	// BackupPath is the .tar.gz or .tar.gz.enc archive to restore.
	BackupPath string
	// ArchivePath is retained as a compatibility alias for BackupPath.
	// Deprecated: use BackupPath.
	ArchivePath string
	// Passphrase decrypts the archive before extraction. Required for
	// archives created with encryption (*.tar.gz.enc).
	Passphrase string
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

// LegacyProviderConfigHandler applies legacy provider settings. Standalone
// krknctl does not provide a provider migration handler because provider
// discovery and schema publication belong to the operator integration.
type LegacyProviderConfigHandler func(context.Context, []LegacyProviderConfig) error

// RestoreBackup applies an operator admin-settings archive directly through
// the Kubernetes API. The operator process itself does not need to be running
// for ordinary resources; provider settings require a migration handler.
func RestoreBackup(ctx context.Context, dynClient dynamic.Interface, config RestoreConfig) (retErr error) {
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
	defer func() {
		if cleanupErr := os.RemoveAll(extractDir); cleanupErr != nil {
			if retErr == nil {
				retErr = fmt.Errorf("restore completed but failed to clean up temporary data: %w", cleanupErr)
				return
			}
			retErr = fmt.Errorf("%w; failed to clean up temporary restore data: %v", retErr, cleanupErr)
		}
	}()

	if config.Passphrase != "" {
		decPath := filepath.Join(extractDir, "decrypted.tar.gz")
		if err := DecryptFile(archivePath, decPath, config.Passphrase); err != nil {
			return fmt.Errorf("failed to decrypt archive: %w", err)
		}
		archivePath = decPath
	}

	if err := extractTarGz(archivePath, extractDir); err != nil {
		if config.Passphrase == "" && IsEncryptedArchive(archivePath) {
			return fmt.Errorf("archive appears to be encrypted; use --passphrase to decrypt it")
		}
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
	var workflowObjects []unstructured.Unstructured
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
				continue
			}
			if entry.Name() == workflowConfigMapsFilename {
				workflowObjects = append(workflowObjects, object)
			}
		}
		jsonFiles = append(jsonFiles, filePath)
	}

	if len(validationFailures) > 0 {
		return fmt.Errorf("archive validation failed: %s", strings.Join(validationFailures, "; "))
	}
	if err := validateWorkflowRestoreConflicts(ctx, dynClient, namespace, workflowObjects); err != nil {
		return fmt.Errorf("workflow restore validation failed: %w", err)
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
			apply := applyResource
			if filepath.Base(filePath) == workflowConfigMapsFilename {
				apply = applyWorkflowResource
			}
			if err := apply(ctx, dynClient, namespace, &object); err != nil {
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
	if obj.GetKind() == "KrknScenarioRun" && !containsString([]string{"Succeeded", "PartiallyFailed", "Failed"}, phaseOf(obj)) {
		return fmt.Errorf("scenario run %q is not completed", obj.GetName())
	}
	if obj.GetKind() == "KrknGraphRun" && !containsString([]string{"Completed", "Failed", "PartiallyFailed"}, phaseOf(obj)) {
		return fmt.Errorf("graph run %q is not completed", obj.GetName())
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
			labels["app.kubernetes.io/component"] != "registry" &&
			labels["app.kubernetes.io/component"] != "cloud-credential") {
			return fmt.Errorf("secret %q is not operator-managed", obj.GetName())
		}
		if labels["app.kubernetes.io/component"] == "cloud-credential" && labels["app.kubernetes.io/name"] != "krkn-operator" {
			return fmt.Errorf("secret %q is not an operator cloud credential", obj.GetName())
		}
	}
	if obj.GetKind() == "ConfigMap" {
		labels := obj.GetLabels()
		if labels == nil || (labels["krkn.krkn-chaos.dev/provider-config"] != "true" &&
			labels["files.krkn.krkn-chaos.dev/file-purpose"] != "workflow-template") {
			return fmt.Errorf("ConfigMap %q is not an operator provider configuration or workflow", obj.GetName())
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
	if filename == reportConfigMapsFilename {
		return validateReportConfigMap(obj)
	}
	return validateRestoreObject(obj)
}

func validateReportConfigMap(obj unstructured.Unstructured) error {
	labels := obj.GetLabels()
	if labels == nil || labels["app"] != "krkn-operator" || labels["component"] != "reports" {
		return fmt.Errorf("ConfigMap %q is not an operator report", obj.GetName())
	}
	if !strings.HasPrefix(obj.GetName(), "krkn-report-") || strings.TrimSpace(labels["krkn-scenario-run"]) == "" {
		return fmt.Errorf("ConfigMap %q is not an operator report", obj.GetName())
	}
	return nil
}

func applyResource(ctx context.Context, dynClient dynamic.Interface, namespace string, obj *unstructured.Unstructured) error {
	gvk := obj.GroupVersionKind()
	gvr := schema.GroupVersionResource{Group: gvk.Group, Version: gvk.Version, Resource: resourceNameForKind(gvk.Kind)}
	resource := dynClient.Resource(gvr).Namespace(namespace)
	var preservedUserStatus map[string]interface{}
	var preservedRunStatus map[string]interface{}
	if gvk.Kind == "KrknUser" {
		if active, found, err := unstructured.NestedBool(obj.Object, "status", "active"); err == nil && found {
			preservedUserStatus = map[string]interface{}{"active": active}
		}
	}
	if gvk.Kind == "KrknScenarioRun" || gvk.Kind == "KrknGraphRun" {
		annotations := obj.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[HistoryOnlyAnnotation] = "true"
		obj.SetAnnotations(annotations)
		status, found, err := unstructured.NestedMap(obj.Object, "status")
		if err != nil {
			return fmt.Errorf("failed to read %s/%s status: %w", gvr.Resource, obj.GetName(), err)
		}
		if found {
			preservedRunStatus = status
		}
	}
	// Status is destination-observed state and must never be copied from an
	// archive, including a manually constructed archive. KrknUser.active is
	// restored separately below because it is account policy, not observed
	// runtime state.
	if preservedRunStatus == nil {
		delete(obj.Object, "status")
	}

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
	if preservedRunStatus != nil {
		if err := restoreRunStatus(ctx, resource, obj.GetName(), preservedRunStatus); err != nil {
			return fmt.Errorf("failed to restore %s/%s status: %w", gvr.Resource, obj.GetName(), err)
		}
	}

	return nil
}

// restoreRunStatus writes archived status against the latest object. The
// operator's controller may reconcile a restored run between the spec update
// and this status update, so a single UpdateStatus can otherwise fail with a
// resource-version conflict.
func restoreRunStatus(ctx context.Context, resource dynamic.ResourceInterface, name string, status map[string]interface{}) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := resource.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if err := unstructured.SetNestedMap(current.Object, status, "status"); err != nil {
			return err
		}
		_, err = resource.UpdateStatus(ctx, current, metav1.UpdateOptions{})
		return err
	})
}

// applyWorkflowResource restores a workflow ConfigMap and its deterministic
// logical-name reservation. The reservation is normally created by the
// operator API, but direct archive restores bypass that API.
func applyWorkflowResource(ctx context.Context, dynClient dynamic.Interface, namespace string, obj *unstructured.Unstructured) error {
	logicalName, fileID, err := workflowIdentity(*obj)
	if err != nil {
		return err
	}
	if err := applyResource(ctx, dynClient, namespace, obj); err != nil {
		return err
	}

	reservation := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":      workflowReservationName(logicalName),
			"namespace": namespace,
			"labels": map[string]interface{}{
				"app.kubernetes.io/name":      "krkn-operator",
				"app.kubernetes.io/component": fileReservationComponent,
			},
		},
		"data": map[string]interface{}{
			"logicalName": logicalName,
			"fileID":      fileID,
		},
	}}
	resource := dynClient.Resource(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}).Namespace(namespace)
	existing, getErr := resource.Get(ctx, reservation.GetName(), metav1.GetOptions{})
	if getErr == nil {
		reservedFileID, found, nestedErr := unstructured.NestedString(existing.Object, "data", "fileID")
		if nestedErr != nil {
			return fmt.Errorf("reservation for workflow %q has invalid fileID: %w", logicalName, nestedErr)
		}
		if !found || strings.TrimSpace(reservedFileID) != fileID {
			return fmt.Errorf("workflow name %q is reserved by another file", logicalName)
		}
		return nil
	}
	if !apierrors.IsNotFound(getErr) {
		return fmt.Errorf("failed to check reservation for workflow %q: %w", logicalName, getErr)
	}
	if _, err := resource.Create(ctx, reservation, metav1.CreateOptions{}); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("workflow name %q was reserved concurrently", logicalName)
		}
		return fmt.Errorf("failed to reserve restored workflow %q: %w", logicalName, err)
	}
	return nil
}

func validateWorkflowRestoreConflicts(ctx context.Context, dynClient dynamic.Interface, namespace string, workflows []unstructured.Unstructured) error {
	if len(workflows) == 0 {
		return nil
	}

	resource := dynClient.Resource(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}).Namespace(namespace)
	existing, err := resource.List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list destination ConfigMaps: %w", err)
	}

	seenNames := make(map[string]string, len(workflows))
	for i := range workflows {
		workflow := workflows[i]
		logicalName, fileID, err := workflowIdentity(workflow)
		if err != nil {
			return fmt.Errorf("workflow %q: %w", workflow.GetName(), err)
		}
		if previous, exists := seenNames[logicalName]; exists {
			return fmt.Errorf("archive contains duplicate workflow name %q in %q and %q", logicalName, previous, workflow.GetName())
		}
		seenNames[logicalName] = workflow.GetName()

		for j := range existing.Items {
			destination := existing.Items[j]
			if destination.GetName() == workflow.GetName() {
				continue
			}
			if destination.GetLabels()[workflowFilePurposeLabel] != workflowFilePurpose {
				continue
			}
			if strings.TrimSpace(destination.GetAnnotations()[workflowNameAnnotation]) == logicalName {
				return fmt.Errorf("workflow name %q already exists on destination as %q", logicalName, destination.GetName())
			}
		}

		reservation, getErr := resource.Get(ctx, workflowReservationName(logicalName), metav1.GetOptions{})
		if getErr != nil && !apierrors.IsNotFound(getErr) {
			return fmt.Errorf("failed to check reservation for workflow %q: %w", logicalName, getErr)
		}
		if apierrors.IsNotFound(getErr) {
			continue
		}
		reservedFileID, found, nestedErr := unstructured.NestedString(reservation.Object, "data", "fileID")
		if nestedErr != nil {
			return fmt.Errorf("reservation for workflow %q has invalid fileID: %w", logicalName, nestedErr)
		}
		if !found || strings.TrimSpace(reservedFileID) == "" {
			return fmt.Errorf("reservation for workflow %q has no fileID", logicalName)
		}
		if reservedFileID != fileID {
			return fmt.Errorf("workflow name %q is reserved by another file", logicalName)
		}
	}
	return nil
}

func workflowIdentity(obj unstructured.Unstructured) (string, string, error) {
	if obj.GetLabels()[workflowFilePurposeLabel] != workflowFilePurpose {
		return "", "", fmt.Errorf("resource is not a workflow template")
	}
	logicalName := strings.TrimSpace(obj.GetAnnotations()[workflowNameAnnotation])
	if logicalName == "" {
		return "", "", fmt.Errorf("workflow is missing %s annotation", workflowNameAnnotation)
	}
	fileID := strings.TrimSpace(obj.GetLabels()[workflowFileIDLabel])
	if fileID == "" {
		return "", "", fmt.Errorf("workflow %q is missing %s label", logicalName, workflowFileIDLabel)
	}
	return logicalName, fileID, nil
}

func workflowReservationName(logicalName string) string {
	digest := sha256.Sum256([]byte(logicalName))
	return "file-reservation-" + hex.EncodeToString(digest[:16])
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
	case "KrknCategory":
		return "krkncategories"
	case "KrknOperatorTarget":
		return "krknoperatortargets"
	case "KrknOperatorTargetProvider":
		return "krknoperatortargetproviders"
	case "KrknScenarioRun":
		return "krknscenarioruns"
	case "KrknGraphRun":
		return "krkngraphruns"
	case "Secret":
		return "secrets"
	case "ConfigMap":
		return "configmaps"
	default:
		return strings.ToLower(kind) + "s"
	}
}

func phaseOf(obj unstructured.Unstructured) string {
	phase, _, _ := unstructured.NestedString(obj.Object, "status", "phase")
	return phase
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
