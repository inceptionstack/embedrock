package embedrock

import "fmt"

// --- Core interface ---

// Embedder generates embedding vectors from text.
// Implementations handle model-specific request/response formats.
type Embedder interface {
	Embed(text string) ([]float64, error)
}

// --- OpenAI-compatible request types ---

// EmbeddingRequest is a single-string input request.
type EmbeddingRequest struct {
	Input string `json:"input"`
	Model string `json:"model"`
}

// EmbeddingRequestBatch is an array input request.
type EmbeddingRequestBatch struct {
	Input []string `json:"input"`
	Model string   `json:"model"`
}

// --- OpenAI-compatible response types ---

// EmbeddingResponse is the top-level response envelope.
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

// Usage reports token consumption.
type Usage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// --- API response types ---

// HealthResponse is returned by GET /.
type HealthResponse struct {
	Status string `json:"status"`
	Model  string `json:"model,omitempty"`
}

// ErrorResponse wraps errors in OpenAI-compatible format.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail is the error payload.
type ErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// --- Errors ---

// EmbedError represents an embedding failure.
type EmbedError struct {
	Message string
}

func (e *EmbedError) Error() string {
	return fmt.Sprintf("embed error: %s", e.Message)
}
