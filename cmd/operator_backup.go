package cmd

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	commonutils "github.com/krkn-chaos/krknctl/pkg/utils"

	"github.com/krkn-chaos/krknctl/pkg/backup"
	"github.com/krkn-chaos/krknctl/pkg/backupstore"
	"github.com/spf13/cobra"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var buildOperatorRESTConfig = clientcmd.BuildConfigFromFlags
var newOperatorDynamicClient = func(config *rest.Config) (dynamic.Interface, error) {
	return dynamic.NewForConfig(config)
}

func NewOperatorBackupCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "backup <namespace>",
		Short: "back up krkn-operator configuration to a .tar.gz archive",
		Long: `Exports krkn-operator admin settings—users, groups, targets, providers,
target authentication, and provider configuration—from a Kubernetes namespace
into a timestamped .tar.gz archive.

Cluster-specific metadata is stripped so the archive can be restored to a
different namespace or cluster. Cloud credentials, saved workflow templates,
completed scenario/graph run history, and generated reports are included;
active runs are excluded. Protect the archive as sensitive data. Other runtime state and the
JWT signing secret are intentionally excluded.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			namespace := args[0]
			outputDir, _ := cmd.Flags().GetString("output")
			kubeconfig, _ := cmd.Flags().GetString("kubeconfig")

			useKind, _ := cmd.Flags().GetBool("kind")
			clusterName, _ := cmd.Flags().GetString("cluster-name")
			kubeconfigPath, err := resolveOperatorKubeconfig(kubeconfig, useKind, clusterName)
			if err != nil {
				return err
			}

			restConfig, err := buildOperatorRESTConfig("", kubeconfigPath)
			if err != nil {
				return fmt.Errorf("failed to build REST config: %w", err)
			}

			dynClient, err := newOperatorDynamicClient(restConfig)
			if err != nil {
				return fmt.Errorf("failed to create dynamic client: %w", err)
			}

			passphrase, _ := cmd.Flags().GetString("passphrase")
			if passphrase == "" {
				passphrase = os.Getenv("KRKNCTL_BACKUP_PASSPHRASE")
			}

			store, err := resolveBackupStore(cmd.Context(), cmd)
			if err != nil {
				return err
			}

			fmt.Printf("Backing up krkn-operator resources from namespace: %s\n", namespace)
			result, err := backup.CreateBackupAndStore(cmd.Context(), dynClient, backup.BackupConfig{
				Namespace:  namespace,
				OutputDir:  outputDir,
				Passphrase: passphrase,
			}, store)
			if result.ArchivePath != "" {
				fmt.Printf("Backup saved to: %s\n", result.ArchivePath)
			}
			if result.Location != nil {
				fmt.Printf("Uploaded to S3: %s\n", result.Location.Key)
				fmt.Printf("Download URL: %s\n", result.Location.DownloadURL)
			}
			if err != nil {
				return fmt.Errorf("backup completed with errors: %w", err)
			}
			return nil
		},
	}
}

func NewOperatorRestoreCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "restore <archive-path-or-url> <namespace>",
		Short: "restore krkn-operator configuration from a local file, HTTPS URL, or S3 URI",
		Long: `Extracts an operator admin-settings archive and applies its resources to
the target namespace. The source can be a local file path, an HTTPS URL, or an
S3 URI such as s3://bucket/path/backup.tar.gz.enc. S3 restores use the AWS
SDK default credential chain and the KRKN_BACKUP_S3_* connection settings.

The archive is validated against the operator's supported resource allowlist;
JWT signing secrets and unrelated Secrets/ConfigMaps are rejected.

Legacy provider settings in older archives are not applied by standalone
krknctl and are reported as a warning. The operator console supplies the
provider migration handler when provider settings need to be restored.

WARNING: restoring replaces existing operator-managed resources with the
contents of the archive. Verify the archive and destination namespace first.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			source := args[0]
			namespace := args[1]
			kubeconfig, _ := cmd.Flags().GetString("kubeconfig")
			useKind, _ := cmd.Flags().GetBool("kind")
			clusterName, _ := cmd.Flags().GetString("cluster-name")

			kubeconfigPath, err := resolveOperatorKubeconfig(kubeconfig, useKind, clusterName)
			if err != nil {
				return err
			}

			restConfig, err := buildOperatorRESTConfig("", kubeconfigPath)
			if err != nil {
				return fmt.Errorf("failed to build REST config: %w", err)
			}

			dynClient, err := newOperatorDynamicClient(restConfig)
			if err != nil {
				return fmt.Errorf("failed to create dynamic client: %w", err)
			}

			passphrase, _ := cmd.Flags().GetString("passphrase")
			if passphrase == "" {
				passphrase = os.Getenv("KRKNCTL_BACKUP_PASSPHRASE")
			}

			archivePath := source
			if strings.HasPrefix(source, "https://") {
				fmt.Printf("Downloading backup from %s\n", redactURL(source))
				downloaded, cleanup, err := backup.DownloadArchive(cmd.Context(), source, 0, 0)
				if err != nil {
					return fmt.Errorf("failed to download backup: %w", err)
				}
				defer cleanup()
				archivePath = downloaded
			} else if strings.HasPrefix(source, "s3://") {
				reference, err := backupstore.ParseS3URI(source)
				if err != nil {
					return err
				}
				store, err := backupstore.NewS3StoreForBucketFromEnv(cmd.Context(), reference.Bucket)
				if err != nil {
					return fmt.Errorf("failed to configure S3 restore: %w", err)
				}
				downloaded, cleanup, err := backup.DownloadArchiveFromStore(cmd.Context(), store, reference.Key, 0, 0)
				if err != nil {
					return fmt.Errorf("failed to download backup from S3: %w", err)
				}
				defer cleanup()
				archivePath = downloaded
			}

			fmt.Printf("Restoring resources from %s to namespace: %s\n", redactURL(source), namespace)
			if err := backup.RestoreBackup(cmd.Context(), dynClient, backup.RestoreConfig{
				BackupPath:             archivePath,
				Namespace:              namespace,
				Passphrase:             passphrase,
				RefreshTargetReadiness: true,
				WarningHandler: func(message string) {
					fmt.Printf("Warning: %s\n", message)
				},
			}); err != nil {
				return err
			}

			fmt.Println("\nRestore complete. Verify with:")
			fmt.Printf("  kubectl get krknusers -n %s\n", namespace)
			fmt.Printf("  kubectl get krknusergroups -n %s\n", namespace)
			fmt.Printf("  kubectl get krknoperatortargets -n %s\n", namespace)
			fmt.Printf("  kubectl get krknoperatortargetproviders -n %s\n", namespace)
			fmt.Printf("  kubectl get krknscenarioruns -n %s\n", namespace)
			fmt.Printf("  kubectl get krkngraphruns -n %s\n", namespace)
			fmt.Printf("  kubectl get secrets -n %s -l 'app.kubernetes.io/component=authentication'\n", namespace)
			fmt.Printf("  kubectl get secrets -n %s -l 'app.kubernetes.io/component=user-auth'\n", namespace)
			fmt.Printf("  kubectl get secrets -n %s -l 'krkn-target-uuid'\n", namespace)
			fmt.Printf("  kubectl get secrets -n %s -l 'app.kubernetes.io/component=elasticsearch-config'\n", namespace)
			fmt.Printf("  kubectl get secrets -n %s -l 'app.kubernetes.io/component=registry'\n", namespace)
			fmt.Printf("  kubectl get configmaps -n %s -l 'krkn.krkn-chaos.dev/provider-config=true'\n", namespace)
			return nil
		},
	}
}

// resolveBackupStore builds an S3 store by merging CLI flags over environment
// defaults. Flags take precedence; unset flags fall through to the env var.
// Returns nil when no bucket is configured by either source.
func resolveBackupStore(ctx context.Context, cmd *cobra.Command) (backupstore.Store, error) {
	bucket, _ := cmd.Flags().GetString("s3-bucket")
	if bucket == "" {
		bucket = os.Getenv(backupstore.BucketEnv)
	}
	if strings.TrimSpace(bucket) == "" {
		return nil, nil
	}

	prefix, _ := cmd.Flags().GetString("s3-prefix")
	if prefix == "" {
		prefix = os.Getenv(backupstore.PrefixEnv)
	}
	region, _ := cmd.Flags().GetString("s3-region")
	if region == "" {
		region = os.Getenv(backupstore.RegionEnv)
	}
	endpoint, _ := cmd.Flags().GetString("s3-endpoint")
	if endpoint == "" {
		endpoint = os.Getenv(backupstore.EndpointEnv)
	}
	kmsKeyID, _ := cmd.Flags().GetString("s3-kms-key-id")
	if kmsKeyID == "" {
		kmsKeyID = os.Getenv(backupstore.KMSKeyIDEnv)
	}

	return backupstore.NewS3Store(ctx, backupstore.S3Config{
		Bucket:   bucket,
		Prefix:   prefix,
		Region:   region,
		Endpoint: endpoint,
		KMSKeyID: kmsKeyID,
	})
}

func resolveKubeconfig(kubeconfig string) (string, error) {
	if kubeconfig == "" {
		kubeconfig = "~/.kube/config"
	}
	expanded, err := commonutils.ExpandFolder(kubeconfig, nil)
	if err != nil {
		return "", fmt.Errorf("invalid kubeconfig path: %w", err)
	}
	return *expanded, nil
}

func resolveOperatorKubeconfig(kubeconfig string, useKind bool, clusterName string) (string, error) {
	if !useKind {
		return resolveKubeconfig(kubeconfig)
	}
	if kubeconfig != "" {
		return "", fmt.Errorf("--kind and --kubeconfig cannot be used together")
	}
	if clusterName == "" {
		return "", fmt.Errorf("cluster name must not be empty")
	}
	if strings.ContainsAny(clusterName, `/\.`) || clusterName == ".." {
		return "", fmt.Errorf("cluster name %q contains invalid characters", clusterName)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to determine home directory: %w", err)
	}
	path := filepath.Join(home, ".krknctl", clusterName+".kubeconfig")
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("KinD kubeconfig not found at %q; install the cluster first or provide --kubeconfig: %w", path, err)
	}
	return path, nil
}

// redactURL strips query parameters from HTTPS URLs so presigned S3
// credentials are not leaked to logs. Non-URL strings pass through unchanged.
func redactURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" {
		return raw
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}
