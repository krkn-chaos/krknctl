package backup

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
)

var (
	krknOperatorTargetResource = schema.GroupVersionResource{Group: "krkn.krkn-chaos.dev", Version: "v1alpha1", Resource: "krknoperatortargets"}
	secretResource             = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}
)

const targetLivenessTimeout = 5 * time.Second

var checkClusterLivenessFunc = checkClusterLiveness

// RefreshRestoredTargetReadiness recalculates target status after target
// Secrets are restored. Status is deliberately not copied from the archive;
// it is destination-cluster state and must be derived from the restored
// kubeconfig.
func RefreshRestoredTargetReadiness(ctx context.Context, dynClient dynamic.Interface, namespace string, warn RestoreWarningHandler) error {
	if dynClient == nil {
		return fmt.Errorf("dynamic Kubernetes client is not configured")
	}
	targets := dynClient.Resource(krknOperatorTargetResource).Namespace(namespace)
	secrets := dynClient.Resource(secretResource).Namespace(namespace)
	targetList, err := targets.List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list restored targets: %w", err)
	}

	for i := range targetList.Items {
		target := &targetList.Items[i]
		clusterName, _, err := unstructured.NestedString(target.Object, "spec", "clusterName")
		if err != nil {
			return fmt.Errorf("failed to read cluster name for restored target %q: %w", target.GetName(), err)
		}
		secretName, _, err := unstructured.NestedString(target.Object, "spec", "secretUUID")
		if err != nil {
			return fmt.Errorf("failed to read Secret name for restored target %q: %w", target.GetName(), err)
		}

		ready, checkErr := restoredTargetIsReady(ctx, secrets, secretName)
		if checkErr != nil {
			return fmt.Errorf("failed to check restored target %q: %w", clusterName, checkErr)
		}
		if !ready {
			warnRestore(warn, fmt.Sprintf("restored target %q is not ready: its destination cluster API could not be reached", clusterName))
		}
		if err := updateRestoredTargetStatus(ctx, targets, target.GetName(), ready); err != nil {
			return fmt.Errorf("failed to update restored target %q status: %w", clusterName, err)
		}
	}
	return nil
}

func restoredTargetIsReady(ctx context.Context, secrets dynamic.ResourceInterface, secretName string) (bool, error) {
	if secretName == "" {
		return false, nil
	}
	secret, err := secrets.Get(ctx, secretName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	encodedSecretData, found, err := unstructured.NestedString(secret.Object, "data", "kubeconfig")
	if err != nil || !found {
		return false, err
	}
	secretData, err := base64.StdEncoding.DecodeString(encodedSecretData)
	if err != nil {
		return false, nil
	}
	var data struct {
		Kubeconfig string `json:"kubeconfig"`
	}
	if err := json.Unmarshal(secretData, &data); err != nil || data.Kubeconfig == "" {
		return false, nil
	}
	if err := checkClusterLivenessFunc(ctx, data.Kubeconfig, targetLivenessTimeout); err != nil {
		return false, nil
	}
	return true, nil
}

func checkClusterLiveness(ctx context.Context, kubeconfigBase64 string, timeout time.Duration) error {
	kubeconfigBytes, err := base64.StdEncoding.DecodeString(kubeconfigBase64)
	if err != nil {
		return err
	}
	config, err := clientcmd.Load(kubeconfigBytes)
	if err != nil {
		return err
	}
	restConfig, err := clientcmd.NewDefaultClientConfig(*config, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return err
	}
	transport, err := rest.TransportFor(restConfig)
	if err != nil {
		return err
	}
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, strings.TrimRight(restConfig.Host, "/")+"/version", nil)
	if err != nil {
		return err
	}
	response, err := (&http.Client{Transport: transport}).Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("api server returned HTTP status %s", response.Status)
	}
	return nil
}

func updateRestoredTargetStatus(ctx context.Context, targets dynamic.ResourceInterface, name string, ready bool) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		target, err := targets.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		status, found, err := unstructured.NestedMap(target.Object, "status")
		if err != nil {
			return err
		}
		if !found {
			status = make(map[string]interface{})
		}
		status["ready"] = ready
		status["lastUpdated"] = time.Now().UTC().Format(time.RFC3339Nano)
		if err := unstructured.SetNestedMap(target.Object, status, "status"); err != nil {
			return err
		}
		_, err = targets.UpdateStatus(ctx, target, metav1.UpdateOptions{})
		return err
	})
}
