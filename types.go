package embedrock

import "fmt"

// Embedder is the interface for generating embeddings — allows mocking Bedrock.
type Embedder interface {
	Embed(text string) ([]float64, error)
}

// MockEmbedder for testing.
type MockEmbedder struct {
	EmbedFunc func(text string) ([]float64, error)
}

func (m *MockEmbedder) Embed(text string) ([]float64, error) {
	if m.EmbedFunc != nil {
		return m.EmbedFunc(text)
	}
	return make([]float64, 1024), nil
}

// EmbedError represents an embedding failure.
type EmbedError struct {
	Message string
}

func (e *EmbedError) Error() string {
	return fmt.Sprintf("embed error: %s", e.Message)
}

// --- OpenAI-compatible request/response types ---

// EmbeddingRequest handles single string input.
type EmbeddingRequest struct {
	Input string `json:"input"`
	Model string `json:"model"`
}

// EmbeddingRequestBatch handles array input.
type EmbeddingRequestBatch struct {
	Input []string `json:"input"`
	Model string   `json:"model"`
}

// EmbeddingResponse is the OpenAI-compatible response.
type EmbeddingResponse struct {
	Object string          `json:"object"`
	Data   []EmbeddingData `json:"data"`
	Model  string          `json:"model"`
	Usage  Usage           `json:"usage"`
}

// EmbeddingData is a single embedding result.
type EmbeddingData struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}

// Usage tracks token usage.
type Usage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// HealthResponse for the health check endpoint.
type HealthResponse struct {
	Status string `json:"status"`
	Model  string `json:"model,omitempty"`
}

// ErrorResponse for error replies.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail is the error payload.
type ErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}
