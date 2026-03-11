package embedrock

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// --- Request/Response types (RED: these don't exist yet) ---

func TestHealthEndpoint(t *testing.T) {
	handler := NewHandler(&MockEmbedder{})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got '%s'", resp.Status)
	}
}

func TestSingleEmbedding(t *testing.T) {
	mock := &MockEmbedder{
		EmbedFunc: func(text string) ([]float64, error) {
			return make([]float64, 1024), nil
		},
	}
	handler := NewHandler(mock)

	body := EmbeddingRequest{
		Input: "test embedding",
		Model: "amazon.titan-embed-text-v2:0",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp EmbeddingResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Object != "list" {
		t.Errorf("expected object 'list', got '%s'", resp.Object)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 embedding, got %d", len(resp.Data))
	}
	if len(resp.Data[0].Embedding) != 1024 {
		t.Errorf("expected 1024 dims, got %d", len(resp.Data[0].Embedding))
	}
	if resp.Data[0].Index != 0 {
		t.Errorf("expected index 0, got %d", resp.Data[0].Index)
	}
}

func TestBatchEmbeddings(t *testing.T) {
	callCount := 0
	mock := &MockEmbedder{
		EmbedFunc: func(text string) ([]float64, error) {
			callCount++
			return make([]float64, 1024), nil
		},
	}
	handler := NewHandler(mock)

	body := EmbeddingRequestBatch{
		Input: []string{"first", "second", "third"},
		Model: "amazon.titan-embed-text-v2:0",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp EmbeddingResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Data) != 3 {
		t.Fatalf("expected 3 embeddings, got %d", len(resp.Data))
	}
	if callCount != 3 {
		t.Errorf("expected 3 embed calls, got %d", callCount)
	}
	// Verify indices are correct
	for i, d := range resp.Data {
		if d.Index != i {
			t.Errorf("expected index %d, got %d", i, d.Index)
		}
	}
}

func TestInvalidMethod(t *testing.T) {
	handler := NewHandler(&MockEmbedder{})
	req := httptest.NewRequest(http.MethodPut, "/v1/embeddings", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestInvalidPath(t *testing.T) {
	handler := NewHandler(&MockEmbedder{})
	req := httptest.NewRequest(http.MethodPost, "/v1/completions", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestEmptyBody(t *testing.T) {
	handler := NewHandler(&MockEmbedder{})
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader([]byte{}))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestEmbedderError(t *testing.T) {
	mock := &MockEmbedder{
		EmbedFunc: func(text string) ([]float64, error) {
			return nil, &EmbedError{Message: "model not available"}
		},
	}
	handler := NewHandler(mock)

	body := EmbeddingRequest{Input: "test", Model: "bad-model"}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestModelPassthrough(t *testing.T) {
	mock := &MockEmbedder{
		EmbedFunc: func(text string) ([]float64, error) {
			return make([]float64, 256), nil
		},
	}
	handler := NewHandler(mock)

	body := EmbeddingRequest{Input: "test", Model: "cohere.embed-english-v3"}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	var resp EmbeddingResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Model != "cohere.embed-english-v3" {
		t.Errorf("expected model 'cohere.embed-english-v3', got '%s'", resp.Model)
	}
}

func TestCohereV4ModelInHealth(t *testing.T) {
	mock := &MockEmbedder{}
	handler := NewHandlerWithModel(mock, "cohere.embed-v4:0")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	var resp HealthResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Model != "cohere.embed-v4:0" {
		t.Errorf("expected model 'cohere.embed-v4:0', got '%s'", resp.Model)
	}
}

func TestCohereV4Embedding(t *testing.T) {
	mock := &MockEmbedder{
		EmbedFunc: func(text string) ([]float64, error) {
			// Cohere v4 returns 1536 dims
			return make([]float64, 1536), nil
		},
	}
	handler := NewHandlerWithModel(mock, "cohere.embed-v4:0")

	body := EmbeddingRequest{
		Input: "test cohere v4 embedding",
		Model: "cohere.embed-v4:0",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp EmbeddingResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Model != "cohere.embed-v4:0" {
		t.Errorf("expected model 'cohere.embed-v4:0', got '%s'", resp.Model)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 embedding, got %d", len(resp.Data))
	}
	if len(resp.Data[0].Embedding) != 1536 {
		t.Errorf("expected 1536 dims, got %d", len(resp.Data[0].Embedding))
	}
}

func TestCohereV4BatchEmbeddings(t *testing.T) {
	callCount := 0
	mock := &MockEmbedder{
		EmbedFunc: func(text string) ([]float64, error) {
			callCount++
			return make([]float64, 1536), nil
		},
	}
	handler := NewHandlerWithModel(mock, "cohere.embed-v4:0")

	body := EmbeddingRequestBatch{
		Input: []string{"first", "second"},
		Model: "cohere.embed-v4:0",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp EmbeddingResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 embeddings, got %d", len(resp.Data))
	}
	if callCount != 2 {
		t.Errorf("expected 2 embed calls, got %d", callCount)
	}
	for i, d := range resp.Data {
		if d.Index != i {
			t.Errorf("expected index %d, got %d", i, d.Index)
		}
		if len(d.Embedding) != 1536 {
			t.Errorf("data[%d]: expected 1536 dims, got %d", i, len(d.Embedding))
		}
	}
}

func TestDefaultModelFallback(t *testing.T) {
	mock := &MockEmbedder{
		EmbedFunc: func(text string) ([]float64, error) {
			return make([]float64, 1536), nil
		},
	}
	handler := NewHandlerWithModel(mock, "cohere.embed-v4:0")

	// Request without model field — should use default
	body := `{"input": "test"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	var resp EmbeddingResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Model != "cohere.embed-v4:0" {
		t.Errorf("expected default model 'cohere.embed-v4:0', got '%s'", resp.Model)
	}
}

func TestCORSHeaders(t *testing.T) {
	handler := NewHandler(&MockEmbedder{})
	req := httptest.NewRequest(http.MethodOptions, "/v1/embeddings", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for OPTIONS, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("missing CORS header")
	}
}
