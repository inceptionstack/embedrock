package embedrock

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// Handler is the HTTP handler for the embedding proxy.
type Handler struct {
	embedder     Embedder
	defaultModel string
}

// NewHandler creates a new embedding proxy handler.
func NewHandler(embedder Embedder) *Handler {
	return &Handler{
		embedder:     embedder,
		defaultModel: "amazon.titan-embed-text-v2:0",
	}
}

// NewHandlerWithModel creates a handler with a specific default model.
func NewHandlerWithModel(embedder Embedder, model string) *Handler {
	return &Handler{
		embedder:     embedder,
		defaultModel: model,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	path := strings.TrimSuffix(r.URL.Path, "/")

	// Health check: GET /
	if r.Method == http.MethodGet && (path == "" || path == "/") {
		h.handleHealth(w)
		return
	}

	// Embeddings: POST /v1/embeddings
	if path == "/v1/embeddings" {
		if r.Method != http.MethodPost {
			h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "invalid_request")
			return
		}
		h.handleEmbeddings(w, r)
		return
	}

	h.writeError(w, http.StatusNotFound, "not found", "invalid_request")
}

func (h *Handler) handleHealth(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(HealthResponse{
		Status: "ok",
		Model:  h.defaultModel,
	})
}

func (h *Handler) handleEmbeddings(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		h.writeError(w, http.StatusBadRequest, "empty or invalid request body", "invalid_request")
		return
	}

	// Try to parse as batch (array input) or single (string input)
	inputs, model, err := h.parseInput(body)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error(), "invalid_request")
		return
	}

	if model == "" {
		model = h.defaultModel
	}

	// Generate embeddings
	data := make([]EmbeddingData, 0, len(inputs))
	for i, text := range inputs {
		embedding, err := h.embedder.Embed(text)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, err.Error(), "server_error")
			return
		}
		data = append(data, EmbeddingData{
			Object:    "embedding",
			Index:     i,
			Embedding: embedding,
		})
	}

	// Calculate rough token count
	promptTokens := 0
	for _, text := range inputs {
		promptTokens += len(text)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(EmbeddingResponse{
		Object: "list",
		Data:   data,
		Model:  model,
		Usage: Usage{
			PromptTokens: promptTokens,
			TotalTokens:  promptTokens,
		},
	})
}

// parseInput handles both single string and array input formats.
func (h *Handler) parseInput(body []byte) ([]string, string, error) {
	// Try batch first (array input)
	var batch EmbeddingRequestBatch
	if err := json.Unmarshal(body, &batch); err == nil && len(batch.Input) > 0 {
		return batch.Input, batch.Model, nil
	}

	// Try single string input
	var single EmbeddingRequest
	if err := json.Unmarshal(body, &single); err == nil && single.Input != "" {
		return []string{single.Input}, single.Model, nil
	}

	// Try raw JSON with input as generic
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, "", err
	}

	model, _ := raw["model"].(string)

	switch v := raw["input"].(type) {
	case string:
		if v == "" {
			return nil, "", &EmbedError{Message: "input is empty"}
		}
		return []string{v}, model, nil
	case []interface{}:
		inputs := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, "", &EmbedError{Message: "input array must contain strings"}
			}
			inputs = append(inputs, s)
		}
		if len(inputs) == 0 {
			return nil, "", &EmbedError{Message: "input array is empty"}
		}
		return inputs, model, nil
	default:
		return nil, "", &EmbedError{Message: "input must be a string or array of strings"}
	}
}

func (h *Handler) writeError(w http.ResponseWriter, status int, message, errType string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error: ErrorDetail{
			Message: message,
			Type:    errType,
		},
	})
}
