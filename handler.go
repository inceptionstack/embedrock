package embedrock

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

const defaultModel = "amazon.titan-embed-text-v2:0"

// Handler serves the OpenAI-compatible embedding API.
type Handler struct {
	embedder     Embedder
	defaultModel string
}

// NewHandler creates a handler with the default model name.
func NewHandler(embedder Embedder) *Handler {
	return &Handler{embedder: embedder, defaultModel: defaultModel}
}

// NewHandlerWithModel creates a handler with a specific default model name.
// The model name appears in health checks and responses when the client omits it.
func NewHandlerWithModel(embedder Embedder, model string) *Handler {
	return &Handler{embedder: embedder, defaultModel: model}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	path := strings.TrimSuffix(r.URL.Path, "/")

	switch {
	case r.Method == http.MethodGet && (path == "" || path == "/"):
		h.handleHealth(w)
	case path == "/v1/embeddings" && r.Method == http.MethodPost:
		h.handleEmbeddings(w, r)
	case path == "/v1/embeddings":
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "invalid_request")
	default:
		h.writeError(w, http.StatusNotFound, "not found", "invalid_request")
	}
}

func (h *Handler) handleHealth(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(HealthResponse{Status: "ok", Model: h.defaultModel})
}

func (h *Handler) handleEmbeddings(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		h.writeError(w, http.StatusBadRequest, "empty or invalid request body", "invalid_request")
		return
	}

	inputs, model, err := parseInput(body)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error(), "invalid_request")
		return
	}
	if model == "" {
		model = h.defaultModel
	} else if model != h.defaultModel {
		h.writeError(w, http.StatusBadRequest,
			fmt.Sprintf("model '%s' is not available; this server is configured with '%s'", model, h.defaultModel),
			"invalid_request")
		return
	}

	data := make([]EmbeddingData, 0, len(inputs))
	promptTokens := 0
	for i, text := range inputs {
		embedding, err := h.embedder.Embed(r.Context(), text)
		if err != nil {
			log.Printf("embedding error: %v", err)
			h.writeError(w, http.StatusInternalServerError, "embedding failed", "server_error")
			return
		}
		data = append(data, EmbeddingData{
			Object:    "embedding",
			Index:     i,
			Embedding: embedding,
		})
		promptTokens += len(text)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(EmbeddingResponse{
		Object: "list",
		Data:   data,
		Model:  model,
		Usage:  Usage{PromptTokens: promptTokens, TotalTokens: promptTokens},
	})
}

// parseInput extracts text inputs and model from an OpenAI-format request body.
func parseInput(body []byte) ([]string, string, error) {
	// Try batch (array input)
	var batch EmbeddingRequestBatch
	if err := json.Unmarshal(body, &batch); err == nil && len(batch.Input) > 0 {
		return batch.Input, batch.Model, nil
	}

	// Try single string input
	var single EmbeddingRequest
	if err := json.Unmarshal(body, &single); err == nil && single.Input != "" {
		return []string{single.Input}, single.Model, nil
	}

	// Fallback: raw JSON with generic input field
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
		Error: ErrorDetail{Message: message, Type: errType},
	})
}
