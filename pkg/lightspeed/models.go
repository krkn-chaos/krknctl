package lightspeed

import (
	"context"
	"github.com/krkn-chaos/krknctl/pkg/config"
)

// GPUAcceleration represents the type of GPU acceleration available
type GPUAcceleration string

const (
	GPUAccelerationAppleSilicon GPUAcceleration = "apple-silicon"
	GPUAccelerationNVIDIA       GPUAcceleration = "nvidia"
	GPUAccelerationGeneric      GPUAcceleration = "generic"
)

// GPUDetector interface for platform-based GPU detection
type GPUDetector interface {
	DetectGPUAcceleration(ctx context.Context, noGPU bool) GPUAcceleration
	GetLightspeedImageURI(gpuType GPUAcceleration) (string, error)
	GetDeviceMounts(gpuType GPUAcceleration) map[string]string
	GetGPUDescription(gpuType GPUAcceleration) string
	HandleContainerError(err error, gpuType GPUAcceleration) error
	AutoSelectLightspeedConfig(ctx context.Context, noGPU bool) (string, GPUAcceleration, map[string]string, error)
}

// PlatformGPUDetector provides platform-based GPU detection
type PlatformGPUDetector struct {
	config config.Config
}

// RAGDeploymentResult holds information about the deployed RAG model
type RAGDeploymentResult struct {
	ContainerID string
	HostPort    string
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status           string `json:"status"`
	Service          string `json:"service"`
	Model            string `json:"model"`
	DocumentsIndexed int    `json:"documents_indexed"`
}

// ChatMessage represents a message in OpenAI format
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// QueryRequest represents an OpenAI-compatible chat completion request
type QueryRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

// QueryChoice represents a choice in OpenAI response format
type QueryChoice struct {
	Index   int `json:"index"`
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	FinishReason string `json:"finish_reason"`
}

// QueryResponse represents an OpenAI-compatible response
type QueryResponse struct {
	ID           string        `json:"id"`
	Object       string        `json:"object"`
	Created      int64         `json:"created"`
	Model        string        `json:"model"`
	Choices      []QueryChoice `json:"choices"`
	Usage        struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	ScenarioName *string       `json:"scenario_name,omitempty"` // krknctl scenario name if detected
}
