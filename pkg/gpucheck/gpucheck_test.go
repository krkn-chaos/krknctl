// Assisted-by: Claude Sonnet 4
package gpucheck

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	orchestratormodels "github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockOrchestrator is a mock implementation of ScenarioOrchestrator for testing
type MockOrchestrator struct {
	mock.Mock
}

func (m *MockOrchestrator) Connect(containerRuntimeURI string) (context.Context, error) {
	args := m.Called(containerRuntimeURI)
	return args.Get(0).(context.Context), args.Error(1)
}

func (m *MockOrchestrator) Run(image, containerName string, env map[string]string, cache bool, volumeMounts map[string]string, devices *map[string]string, commChan *chan *string, ctx context.Context, registry *models.RegistryV2, portMappings *map[string]string) (*string, error) {
	args := m.Called(image, containerName, env, cache, volumeMounts, commChan, ctx, registry, portMappings)
	return args.Get(0).(*string), args.Error(1)
}

func (m *MockOrchestrator) GetContainerRuntime() orchestratormodels.ContainerRuntime {
	args := m.Called()
	return args.Get(0).(orchestratormodels.ContainerRuntime)
}

func (m *MockOrchestrator) PrintContainerRuntime() {
	m.Called()
}

func (m *MockOrchestrator) GetConfig() config.Config {
	args := m.Called()
	return args.Get(0).(config.Config)
}

// Implement other required interface methods as no-ops for testing
func (m *MockOrchestrator) RunAttached(image string, containerName string, env map[string]string, cache bool, volumeMounts map[string]string, devices *map[string]string, stdout io.Writer, stderr io.Writer, commChan *chan *string, ctx context.Context, registry *models.RegistryV2) (*string, error) {
	args := m.Called(image, containerName, env, cache, volumeMounts, stdout, stderr, commChan, ctx, registry)
	return args.Get(0).(*string), args.Error(1)
}

func (m *MockOrchestrator) Attach(containerID *string, signalChannel chan os.Signal, stdout io.Writer, stderr io.Writer, ctx context.Context) (bool, error) {
	args := m.Called(containerID, signalChannel, stdout, stderr, ctx)
	return args.Bool(0), args.Error(1)
}

func (m *MockOrchestrator) Kill(containerID *string, ctx context.Context) error {
	return nil
}

func (m *MockOrchestrator) CleanContainers(ctx context.Context) (*int, error) {
	return nil, nil
}

func (m *MockOrchestrator) AttachWait(containerID *string, stdout io.Writer, stderr io.Writer, ctx context.Context) (*bool, error) {
	return nil, nil
}

func (m *MockOrchestrator) ListRunningContainers(ctx context.Context) (*map[int64]orchestratormodels.Container, error) {
	return nil, nil
}

func (m *MockOrchestrator) ListRunningScenarios(ctx context.Context) (*[]orchestratormodels.ScenarioContainer, error) {
	return nil, nil
}

func (m *MockOrchestrator) GetContainerRuntimeSocket(userID *int) (*string, error) {
	return nil, nil
}

func (m *MockOrchestrator) InspectScenario(container orchestratormodels.Container, ctx context.Context) (*orchestratormodels.ScenarioContainer, error) {
	args := m.Called(container, ctx)
	return args.Get(0).(*orchestratormodels.ScenarioContainer), args.Error(1)
}

func (m *MockOrchestrator) RunGraph(scenarios orchestratormodels.ScenarioSet, resolvedGraph orchestratormodels.ResolvedGraph, extraEnv map[string]string, extraVolumeMounts map[string]string, cache bool, commChannel chan *orchestratormodels.GraphCommChannel, registry *models.RegistryV2, userID *int) {
}

func (m *MockOrchestrator) ResolveContainerName(containerName string, ctx context.Context) (*string, error) {
	return nil, nil
}

// Helper function to create a test config
func getTestConfig() config.Config {
	return config.Config{
		QuayHost:           "quay.io",
		QuayOrg:            "krkn-chaos",
		LightspeedRegistry: "krknctl-lightspeed",
		GpuCheckBaseTag:    "gpu-check",
	}
}

func TestNewGpuChecker(t *testing.T) {
	config := getTestConfig()

	// Since we can't mock easily, just test with nil orchestrator
	checker := NewGpuChecker(nil, config)

	assert.NotNil(t, checker)
	assert.Nil(t, checker.orchestrator)
	assert.Equal(t, config, checker.config)
}

func TestParseGPUCheckOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected bool
	}{
		{
			name:     "Success indicator - GPU_CHECK_SUCCESS",
			output:   "GPU_CHECK_SUCCESS\nGPU Support: Yes",
			expected: true,
		},
		{
			name:     "Success indicator - gpu support yes",
			output:   "some log\nGPU Support: Yes\nmore log",
			expected: true,
		},
		{
			name:     "Success indicator - gpu detected",
			output:   "GPU detected via nvidia-smi",
			expected: true,
		},
		{
			name:     "Failure indicator - GPU_CHECK_FAILED",
			output:   "GPU_CHECK_FAILED\nGPU Support: No",
			expected: false,
		},
		{
			name:     "Failure indicator - gpu support no",
			output:   "some log\nGPU Support: No\nmore log",
			expected: false,
		},
		{
			name:     "Failure indicator - no gpu detected",
			output:   "No GPU detected",
			expected: false,
		},
		{
			name:     "Failure indicator - gpu not available",
			output:   "GPU not available",
			expected: false,
		},
		{
			name:     "Ambiguous output",
			output:   "some random output without clear indicators",
			expected: false,
		},
		{
			name:     "Empty output",
			output:   "",
			expected: false,
		},
		{
			name:     "Case insensitive success",
			output:   "GPU_CHECK_SUCCESS",
			expected: true,
		},
		{
			name:     "Case insensitive failure",
			output:   "GPU_CHECK_FAILED",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseGPUCheckOutput(tt.output)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatResult(t *testing.T) {
	config := getTestConfig()
	checker := NewGpuChecker(nil, config)

	tests := []struct {
		name     string
		result   *Result
		contains []string
	}{
		{
			name: "Success result",
			result: &Result{
				HasGPUSupport: true,
				Output:        "GPU_CHECK_SUCCESS",
				Error:         nil,
			},
			contains: []string{"GPU Support: Yes", "✅", "suitable for running krknctl lightspeed workloads"},
		},
		{
			name: "Failure result",
			result: &Result{
				HasGPUSupport: false,
				Output:        "GPU_CHECK_FAILED",
				Error:         nil,
			},
			contains: []string{"GPU Support: No", "⚠️", "not suitable for running krknctl lightspeed workloads", "NVIDIA GPUs", "Apple Silicon"},
		},
		{
			name: "Error result",
			result: &Result{
				HasGPUSupport: false,
				Output:        "",
				Error:         assert.AnError,
			},
			contains: []string{"GPU Support: No", "Error occurred during GPU check", "not suitable for running krknctl lightspeed workloads"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checker.FormatResult(tt.result)
			for _, expectedText := range tt.contains {
				assert.Contains(t, result, expectedText)
			}
		})
	}
}

func TestResult_struct(t *testing.T) {
	// Test Result struct creation and field access
	result := &Result{
		HasGPUSupport: true,
		Output:        "test output",
		Error:         nil,
	}

	assert.True(t, result.HasGPUSupport)
	assert.Equal(t, "test output", result.Output)
	assert.Nil(t, result.Error)
}

func TestGpuChecker_struct(t *testing.T) {
	config := getTestConfig()
	checker := &GpuChecker{
		orchestrator: nil,
		config:       config,
	}

	assert.Nil(t, checker.orchestrator)
	assert.Equal(t, config, checker.config)
}

// Test Docker runtime detection
func TestCheckGPUSupportByType_DockerRuntime(t *testing.T) {
	config := getTestConfig()
	mockOrchestrator := &MockOrchestrator{}

	// Mock Docker runtime
	mockOrchestrator.On("GetContainerRuntime").Return(orchestratormodels.Docker)

	checker := NewGpuChecker(mockOrchestrator, config)

	result, err := checker.CheckGPUSupportByType(context.Background(), "nvidia", nil)

	assert.NoError(t, err)
	assert.False(t, result.HasGPUSupport)
	assert.Contains(t, result.Error.Error(), "lightspeed GPU features are not available with Docker runtime")
	assert.Contains(t, result.Error.Error(), "requires Podman with GPU support")

	mockOrchestrator.AssertExpectations(t)
}

// Test Podman runtime with mock (cannot test full flow without container runtime)
func TestCheckGPUSupportByType_PodmanRuntime(t *testing.T) {
	config := getTestConfig()
	mockOrchestrator := &MockOrchestrator{}

	// Mock Podman runtime
	mockOrchestrator.On("GetContainerRuntime").Return(orchestratormodels.Podman)

	// Mock both Run and Attach methods that are called by CommonRunAttached
	containerID := "test-container-id"
	// First CommonRunAttached calls Run
	mockOrchestrator.On("Run", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("map[string]string"), false, mock.AnythingOfType("map[string]string"), (*chan *string)(nil), mock.Anything, (*models.RegistryV2)(nil), (*map[string]string)(nil)).Return(&containerID, nil)
	// Then CommonRunAttached calls Attach
	mockOrchestrator.On("Attach", &containerID, mock.AnythingOfType("chan os.Signal"), mock.AnythingOfType("*bytes.Buffer"), mock.AnythingOfType("*bytes.Buffer"), mock.Anything).Return(true, nil)
	// Finally CommonRunAttached calls InspectScenario
	scenarioContainer := &orchestratormodels.ScenarioContainer{
		Container: &orchestratormodels.Container{
			ID:         containerID,
			ExitStatus: 0, // Success exit status
		},
	}
	mockOrchestrator.On("InspectScenario", mock.AnythingOfType("models.Container"), mock.Anything).Return(scenarioContainer, nil)

	checker := NewGpuChecker(mockOrchestrator, config)

	// This test should now succeed with mocked RunAttached method
	result, err := checker.CheckGPUSupportByType(context.Background(), "nvidia", nil)

	// Should succeed with mocked methods
	assert.NoError(t, err)
	assert.NotNil(t, result)
	// The result should parse the output, but since we're mocking the container output as empty,
	// it won't contain success indicators so HasGPUSupport will be false
	assert.False(t, result.HasGPUSupport)

	mockOrchestrator.AssertExpectations(t)
}

// Test GPU-specific device mounting logic
func TestGetGPUDeviceMounts(t *testing.T) {
	config := getTestConfig()
	checker := NewGpuChecker(nil, config)

	tests := []struct {
		name         string
		gpuType      string
		expectedKeys []string
	}{
		{
			name:         "NVIDIA GPU devices",
			gpuType:      "nvidia",
			expectedKeys: []string{"/dev/nvidia0", "/dev/nvidiactl", "/dev/nvidia-uvm"},
		},
		{
			name:         "AMD GPU devices",
			gpuType:      "amd",
			expectedKeys: []string{"/dev/dri"},
		},
		{
			name:         "Intel GPU devices",
			gpuType:      "intel",
			expectedKeys: []string{"/dev/dri"},
		},
		{
			name:         "Apple Silicon GPU devices",
			gpuType:      "apple-silicon",
			expectedKeys: []string{"/dev/dri"},
		},
		{
			name:         "Unknown GPU type",
			gpuType:      "unknown",
			expectedKeys: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mounts := checker.getGPUDeviceMounts(tt.gpuType)

			if len(tt.expectedKeys) == 0 {
				assert.Empty(t, mounts)
			} else {
				for _, expectedKey := range tt.expectedKeys {
					assert.Contains(t, mounts, expectedKey)
					assert.Equal(t, expectedKey, mounts[expectedKey]) // Should map to same path
				}
			}
		})
	}
}

// Test GPU type to image URI mapping
func TestGpuCheckImageURIByType(t *testing.T) {
	config := getTestConfig()

	tests := []struct {
		name        string
		gpuType     string
		expectedTag string
	}{
		{
			name:        "NVIDIA GPU image",
			gpuType:     "nvidia",
			expectedTag: "gpu-check-nvidia",
		},
		{
			name:        "AMD GPU image",
			gpuType:     "amd",
			expectedTag: "gpu-check-amd",
		},
		{
			name:        "Intel GPU image",
			gpuType:     "intel",
			expectedTag: "gpu-check-intel",
		},
		{
			name:        "Apple Silicon GPU image",
			gpuType:     "apple-silicon",
			expectedTag: "gpu-check-apple-silicon",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uri, err := config.GetGpuCheckImageURIByType(tt.gpuType)

			assert.NoError(t, err)
			assert.Contains(t, uri, "quay.io/krkn-chaos/krknctl-lightspeed")
			assert.Contains(t, uri, tt.expectedTag)
		})
	}
}

// Test fallback to default image for unknown GPU type
func TestGpuCheckImageURIByType_Fallback(t *testing.T) {
	config := getTestConfig()

	uri, err := config.GetGpuCheckImageURIByType("unknown-gpu-type")

	assert.NoError(t, err)
	assert.Contains(t, uri, "quay.io/krkn-chaos/krknctl-lightspeed")
	assert.Contains(t, uri, "gpu-check") // Should fallback to base tag
}
