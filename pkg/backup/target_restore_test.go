package backup

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestRefreshRestoredTargetReadinessMarksReachableTargetReady(t *testing.T) {
	setLivenessCheck(t, nil)

	client := newTargetReadinessClient(t, "target-secret", targetSecret(t, "target-secret", "encoded-kubeconfig"))
	var warnings []string
	err := RefreshRestoredTargetReadiness(context.Background(), client, "destination-ns", func(message string) {
		warnings = append(warnings, message)
	})

	assert.NoError(t, err)
	assert.Empty(t, warnings)
	assertTargetReadiness(t, client, true)
}

func TestRefreshRestoredTargetReadinessMarksUnreachableTargetNotReady(t *testing.T) {
	setLivenessCheck(t, errors.New("unreachable"))

	client := newTargetReadinessClient(t, "target-secret", targetSecret(t, "target-secret", "encoded-kubeconfig"))
	var warnings []string
	err := RefreshRestoredTargetReadiness(context.Background(), client, "destination-ns", func(message string) {
		warnings = append(warnings, message)
	})

	assert.NoError(t, err)
	assert.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "not ready")
	assertTargetReadiness(t, client, false)
}

func TestRefreshRestoredTargetReadinessMarksMissingAndInvalidSecretsNotReady(t *testing.T) {
	setLivenessCheck(t, nil)

	tests := []struct {
		name   string
		secret *unstructured.Unstructured
	}{
		{name: "missing secret"},
		{
			name: "invalid secret",
			secret: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Secret",
				"metadata":   map[string]interface{}{"name": "target-secret", "namespace": "destination-ns"},
				"data":       map[string]interface{}{"kubeconfig": "not-base64"},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTargetReadinessClient(t, "target-secret", tt.secret)
			err := RefreshRestoredTargetReadiness(context.Background(), client, "destination-ns", nil)
			assert.NoError(t, err)
			assertTargetReadiness(t, client, false)
		})
	}
}

func TestRefreshRestoredTargetReadinessRequiresClient(t *testing.T) {
	err := RefreshRestoredTargetReadiness(context.Background(), nil, "destination-ns", nil)
	assert.EqualError(t, err, "dynamic Kubernetes client is not configured")
}

func TestRefreshRestoredTargetReadinessPropagatesListError(t *testing.T) {
	setLivenessCheck(t, nil)
	client := newTargetReadinessClient(t, "target-secret", targetSecret(t, "target-secret", "encoded-kubeconfig"))
	client.PrependReactor("list", "krknoperatortargets", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("list failed")
	})

	err := RefreshRestoredTargetReadiness(context.Background(), client, "destination-ns", nil)
	assert.ErrorContains(t, err, "list failed")
}

func TestRefreshRestoredTargetReadinessPropagatesStatusUpdateError(t *testing.T) {
	setLivenessCheck(t, nil)
	client := newTargetReadinessClient(t, "target-secret", targetSecret(t, "target-secret", "encoded-kubeconfig"))
	client.PrependReactor("update", "krknoperatortargets", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("status update failed")
	})

	err := RefreshRestoredTargetReadiness(context.Background(), client, "destination-ns", nil)
	assert.ErrorContains(t, err, "status update failed")
}

func setLivenessCheck(t *testing.T, checkErr error) {
	t.Helper()
	original := checkClusterLivenessFunc
	checkClusterLivenessFunc = func(context.Context, string, time.Duration) error {
		return checkErr
	}
	t.Cleanup(func() {
		checkClusterLivenessFunc = original
	})
}

func targetSecret(t *testing.T, name, kubeconfig string) *unstructured.Unstructured {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"kubeconfig": kubeconfig})
	if err != nil {
		t.Fatal(err)
	}
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata":   map[string]interface{}{"name": name, "namespace": "destination-ns"},
		"data":       map[string]interface{}{"kubeconfig": base64.StdEncoding.EncodeToString(payload)},
	}}
}

func newTargetReadinessClient(t *testing.T, secretName string, secret *unstructured.Unstructured) *dynamicfake.FakeDynamicClient {
	t.Helper()
	objects := []runtime.Object{
		&unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "krkn.krkn-chaos.dev/v1alpha1",
			"kind":       "KrknOperatorTarget",
			"metadata":   map[string]interface{}{"name": "target-1", "namespace": "destination-ns"},
			"spec":       map[string]interface{}{"clusterName": "destination", "secretUUID": secretName},
		}},
	}
	if secret != nil {
		objects = append(objects, secret)
	}
	return newFakeDynamicClient(objects...)
}

func assertTargetReadiness(t *testing.T, client *dynamicfake.FakeDynamicClient, wantReady bool) {
	t.Helper()
	target, err := client.Resource(krknOperatorTargetResource).Namespace("destination-ns").Get(context.Background(), "target-1", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	ready, found, err := unstructured.NestedBool(target.Object, "status", "ready")
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	assert.Equal(t, wantReady, ready)

	lastUpdated, found, err := unstructured.NestedString(target.Object, "status", "lastUpdated")
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	assert.NotEmpty(t, lastUpdated)
}
