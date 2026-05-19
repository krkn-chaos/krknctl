// Package assist provides AI-powered chaos engineering assistance using RAG (Retrieval-Augmented Generation).
// This file contains data structures for the assist service API and deployment.
package assist

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
	ID      string        `json:"id"`
	Object  string        `json:"object"`
	Created int64         `json:"created"`
	Model   string        `json:"model"`
	Choices []QueryChoice `json:"choices"`
	Usage   struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	ScenarioName *string         `json:"scenario_name,omitempty"` // krknctl scenario name if detected
	Scenarios    []ScenarioMatch `json:"scenarios,omitempty"`
}

// ScenarioMatch represents a scenario matched by the assist service.
type ScenarioMatch struct {
	Name         string  `json:"name,omitempty"`
	RunnableName string  `json:"runnable_name,omitempty"`
	Title        string  `json:"title,omitempty"`
	Summary      string  `json:"summary,omitempty"`
	RunCommand   string  `json:"run_command,omitempty"`
	Score        float64 `json:"score,omitempty"`
	Rank         int     `json:"rank,omitempty"`
	IsPrimary    bool    `json:"is_primary,omitempty"`
}
