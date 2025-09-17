package lightspeed

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/briandowns/spinner"
	"github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	orchestratormodels "github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/models"
)

// MockScenarioOrchestrator implements the ScenarioOrchestrator interface for testing
type MockScenarioOrchestrator struct {
	containerID      string
	shouldFailRun    bool
	shouldFailKill   bool
	containers       []orchestratormodels.Container
	shouldFailList   bool
}

func (m *MockScenarioOrchestrator) Run(imageURI string, containerName string, env map[string]string, waitForCompletion bool, volumes map[string]string, devices *map[string]string, commChan *chan *string, ctx context.Context, registrySettings *models.RegistryV2, portMappings *map[string]string) (*string, error) {
	if m.shouldFailRun {
		return nil, fmt.Errorf("mock run failure")
	}

	// Simulate pulling progress
	if commChan != nil {
		go func() {
			defer close(*commChan)
			messages := []string{"Pulling image...", "Downloaded layer 1/3", "Downloaded layer 2/3", "Downloaded layer 3/3", "Pull complete"}
			for _, msg := range messages {
				select {
				case *commChan <- &msg:
					time.Sleep(10 * time.Millisecond) // Simulate some delay
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	return &m.containerID, nil
}

func (m *MockScenarioOrchestrator) Kill(containerID *string, ctx context.Context) error {
	if m.shouldFailKill {
		return fmt.Errorf("mock kill failure")
	}
	return nil
}

func (m *MockScenarioOrchestrator) ListRunningContainers(ctx context.Context) (*map[int64]orchestratormodels.Container, error) {
	if m.shouldFailList {
		return nil, fmt.Errorf("mock list failure")
	}
	containerMap := make(map[int64]orchestratormodels.Container)
	for i, container := range m.containers {
		containerMap[int64(i)] = container
	}
	return &containerMap, nil
}

// Implement other required methods (empty implementations for test)
func (m *MockScenarioOrchestrator) PrintContainerRuntime() {}
func (m *MockScenarioOrchestrator) GetContainerRuntime() orchestratormodels.ContainerRuntime { return orchestratormodels.Podman }
func (m *MockScenarioOrchestrator) GetContainerRuntimeSocket(*int) (*string, error) { socket := "unix://test.sock"; return &socket, nil }
func (m *MockScenarioOrchestrator) Connect(string) (context.Context, error) { return context.Background(), nil }
func (m *MockScenarioOrchestrator) RunAttached(string, string, map[string]string, bool, map[string]string, *map[string]string, io.Writer, io.Writer, *chan *string, context.Context, *models.RegistryV2) (*string, error) { return nil, nil }
func (m *MockScenarioOrchestrator) RunGraph(orchestratormodels.ScenarioSet, orchestratormodels.ResolvedGraph, map[string]string, map[string]string, bool, chan *orchestratormodels.GraphCommChannel, *models.RegistryV2, *int) {}
func (m *MockScenarioOrchestrator) CleanContainers(context.Context) (*int, error) { return nil, nil }
func (m *MockScenarioOrchestrator) AttachWait(*string, io.Writer, io.Writer, context.Context) (*bool, error) { return nil, nil }
func (m *MockScenarioOrchestrator) Attach(*string, chan os.Signal, io.Writer, io.Writer, context.Context) (bool, error) { return false, nil }
func (m *MockScenarioOrchestrator) ListRunningScenarios(context.Context) (*[]orchestratormodels.ScenarioContainer, error) { return nil, nil }
func (m *MockScenarioOrchestrator) InspectScenario(orchestratormodels.Container, context.Context) (*orchestratormodels.ScenarioContainer, error) { return nil, nil }
func (m *MockScenarioOrchestrator) GetConfig() config.Config { return config.Config{} }
func (m *MockScenarioOrchestrator) ResolveContainerName(string, context.Context) (*string, error) { return nil, nil }

// MockGPUDetector implements the GPUDetector interface for testing
type MockGPUDetector struct {
	gpuType     GPUAcceleration
	imageURI    string
	deviceMounts map[string]string
}

func (m *MockGPUDetector) DetectGPUAcceleration(ctx context.Context, noGPU bool) GPUAcceleration {
	return m.gpuType
}

func (m *MockGPUDetector) GetLightspeedImageURI(gpuType GPUAcceleration) (string, error) {
	return m.imageURI, nil
}

func (m *MockGPUDetector) GetDeviceMounts(gpuType GPUAcceleration) map[string]string {
	return m.deviceMounts
}

func (m *MockGPUDetector) GetGPUDescription(gpuType GPUAcceleration) string {
	switch gpuType {
	case GPUAccelerationAppleSilicon:
		return "Apple Silicon GPU (Metal)"
	case GPUAccelerationNVIDIA:
		return "NVIDIA GPU (CUDA)"
	case GPUAccelerationGeneric:
		return "CPU-only (no GPU acceleration)"
	default:
		return "Unknown GPU type"
	}
}

func (m *MockGPUDetector) HandleContainerError(err error, gpuType GPUAcceleration) error {
	return err
}

func (m *MockGPUDetector) AutoSelectLightspeedConfig(ctx context.Context, noGPU bool) (string, GPUAcceleration, map[string]string, error) {
	return m.imageURI, m.gpuType, m.deviceMounts, nil
}

func createTestConfig() config.Config {
	return config.Config{
		RAGContainerPrefix:            "test-lightspeed",
		RAGServicePort:               "8080",
		RAGHost:                      "127.0.0.1",
		RAGHealthEndpoint:            "/health",
		RAGQueryEndpoint:             "/v1/chat/completions",
		RAGHealthMaxRetries:          3,
		RAGHealthRetryIntervalSeconds: 1,
	}
}

func TestDeployLightspeedModelWithGPUType_Success(t *testing.T) {
	// Setup
	ctx := context.Background()
	config := createTestConfig()

	mockOrchestrator := &MockScenarioOrchestrator{
		containerID: "test-container-123",
	}

	mockDetector := &MockGPUDetector{
		gpuType:      GPUAccelerationAppleSilicon,
		imageURI:     "test-registry/lightspeed:latest",
		deviceMounts: map[string]string{"/dev/dri": "/dev/dri"},
	}

	mockSpinner := spinner.New(spinner.CharSets[35], 100*time.Millisecond)

	// Execute
	result, err := DeployLightspeedModelWithGPUType(ctx, GPUAccelerationAppleSilicon, mockOrchestrator, config, nil, mockDetector, mockSpinner)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	if result.ContainerID != "test-container-123" {
		t.Errorf("Expected ContainerID 'test-container-123', got '%s'", result.ContainerID)
	}

	if result.HostPort != "8080" {
		t.Errorf("Expected HostPort '8080', got '%s'", result.HostPort)
	}
}

func TestDeployLightspeedModelWithGPUType_RunFailure(t *testing.T) {
	// Setup
	ctx := context.Background()
	config := createTestConfig()

	mockOrchestrator := &MockScenarioOrchestrator{
		shouldFailRun: true,
	}

	mockDetector := &MockGPUDetector{
		gpuType:  GPUAccelerationNVIDIA,
		imageURI: "test-registry/lightspeed-nvidia:latest",
	}

	// Execute
	result, err := DeployLightspeedModelWithGPUType(ctx, GPUAccelerationNVIDIA, mockOrchestrator, config, nil, mockDetector, nil)

	// Assert
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if result != nil {
		t.Fatal("Expected nil result on error")
	}

	if err.Error() != "failed to run RAG container: mock run failure" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestDeployLightspeedModelWithGPUType_WithSpinner(t *testing.T) {
	// Setup
	ctx := context.Background()
	config := createTestConfig()

	mockOrchestrator := &MockScenarioOrchestrator{
		containerID: "test-container-456",
	}

	mockDetector := &MockGPUDetector{
		gpuType:  GPUAccelerationGeneric,
		imageURI: "test-registry/lightspeed-generic:latest",
	}

	mockSpinner := spinner.New(spinner.CharSets[35], 100*time.Millisecond)

	// Execute
	result, err := DeployLightspeedModelWithGPUType(ctx, GPUAccelerationGeneric, mockOrchestrator, config, nil, mockDetector, mockSpinner)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.ContainerID != "test-container-456" {
		t.Errorf("Expected ContainerID 'test-container-456', got '%s'", result.ContainerID)
	}
}

func TestPerformLightspeedHealthCheck_Success(t *testing.T) {
	// Setup mock HTTP server that returns healthy status
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			response := `{
				"status": "healthy",
				"service": "lightspeed-rag",
				"model": "llama-3.2-1b",
				"documents_indexed": 150
			}`
			w.Write([]byte(response))
		}
	}))
	defer server.Close()

	// Extract port from server URL
	config := createTestConfig()
	config.RAGHost = "127.0.0.1"
	// Parse port from server.URL (format: http://127.0.0.1:PORT)
	serverURL := server.URL
	port := serverURL[len("http://127.0.0.1:"):]

	ctx := context.Background()
	containerID := "test-container"

	mockOrchestrator := &MockScenarioOrchestrator{
		containers: []orchestratormodels.Container{
			{ID: containerID, Name: "test-container"},
		},
	}

	// Execute
	healthy, err := PerformLightspeedHealthCheck(containerID, port, mockOrchestrator, ctx, config)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !healthy {
		t.Error("Expected healthy=true, got false")
	}
}

func TestPerformLightspeedHealthCheck_Unhealthy(t *testing.T) {
	// Setup mock HTTP server that returns unhealthy status
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			response := `{
				"status": "unhealthy",
				"service": "lightspeed-rag",
				"model": "llama-3.2-1b",
				"documents_indexed": 0
			}`
			w.Write([]byte(response))
		}
	}))
	defer server.Close()

	config := createTestConfig()
	config.RAGHost = "127.0.0.1"
	config.RAGHealthMaxRetries = 2
	serverURL := server.URL
	port := serverURL[len("http://127.0.0.1:"):]

	ctx := context.Background()
	containerID := "test-container"

	mockOrchestrator := &MockScenarioOrchestrator{
		containers: []orchestratormodels.Container{
			{ID: containerID, Name: "test-container"},
		},
	}

	// Execute
	healthy, err := PerformLightspeedHealthCheck(containerID, port, mockOrchestrator, ctx, config)

	// Assert
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if healthy {
		t.Error("Expected healthy=false, got true")
	}
}

func TestPerformLightspeedHealthCheck_ContainerNotRunning(t *testing.T) {
	config := createTestConfig()
	ctx := context.Background()
	containerID := "missing-container"
	hostPort := "8080"

	mockOrchestrator := &MockScenarioOrchestrator{
		containers: []orchestratormodels.Container{
			{ID: "other-container", Name: "other"},
		},
	}

	// Execute
	healthy, err := PerformLightspeedHealthCheck(containerID, hostPort, mockOrchestrator, ctx, config)

	// Assert
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if healthy {
		t.Error("Expected healthy=false, got true")
	}

	expectedError := "container missing-container is no longer running"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestQueryLightspeedService_Success(t *testing.T) {
	// Setup mock HTTP server that returns a valid response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			response := `{
				"id": "chatcmpl-123",
				"object": "chat.completion",
				"created": 1677652288,
				"model": "llama",
				"choices": [
					{
						"index": 0,
						"message": {
							"role": "assistant",
							"content": "To run a pod deletion scenario, use: krknctl run pod-scenarios --pod-name test-pod"
						},
						"finish_reason": "stop"
					}
				],
				"usage": {
					"prompt_tokens": 15,
					"completion_tokens": 20,
					"total_tokens": 35
				}
			}`
			w.Write([]byte(response))
		}
	}))
	defer server.Close()

	config := createTestConfig()
	config.RAGHost = "127.0.0.1"
	serverURL := server.URL
	port := serverURL[len("http://127.0.0.1:"):]

	// Execute
	response, err := queryLightspeedService(port, "How do I run a pod deletion scenario?", config)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	if len(response.Choices) != 1 {
		t.Fatalf("Expected 1 choice, got %d", len(response.Choices))
	}

	expectedContent := "To run a pod deletion scenario, use: krknctl run pod-scenarios --pod-name test-pod"
	if response.Choices[0].Message.Content != expectedContent {
		t.Errorf("Expected content '%s', got '%s'", expectedContent, response.Choices[0].Message.Content)
	}
}

func TestQueryLightspeedService_ServerError(t *testing.T) {
	// Setup mock HTTP server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	config := createTestConfig()
	config.RAGHost = "127.0.0.1"
	serverURL := server.URL
	port := serverURL[len("http://127.0.0.1:"):]

	// Execute
	response, err := queryLightspeedService(port, "test query", config)

	// Assert
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if response != nil {
		t.Fatal("Expected nil response on error")
	}

	expectedError := "service returned error 500: Internal Server Error"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestRAGDeploymentResult_Fields(t *testing.T) {
	// Test that RAGDeploymentResult has the correct exported fields
	result := &RAGDeploymentResult{
		ContainerID: "test-container-789",
		HostPort:    "9090",
	}

	if result.ContainerID != "test-container-789" {
		t.Errorf("Expected ContainerID 'test-container-789', got '%s'", result.ContainerID)
	}

	if result.HostPort != "9090" {
		t.Errorf("Expected HostPort '9090', got '%s'", result.HostPort)
	}
}