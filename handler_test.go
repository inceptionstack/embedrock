package embedrock

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- Health ---

func TestHealthEndpoint(t *testing.T) {
	handler := NewHandler(&MockEmbedder{})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp HealthResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got '%s'", resp.Status)
	}
	if resp.Model != defaultModel {
		t.Errorf("expected default model, got '%s'", resp.Model)
	}
}

func TestHealthWithCustomModel(t *testing.T) {
	handler := NewHandlerWithModel(&MockEmbedder{}, "cohere.embed-v4:0")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	var resp HealthResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Model != "cohere.embed-v4:0" {
		t.Errorf("expected 'cohere.embed-v4:0', got '%s'", resp.Model)
	}
}

// --- Single embedding ---

func TestSingleEmbedding(t *testing.T) {
	mock := &MockEmbedder{
		EmbedFunc: func(ctx context.Context, text string) ([]float64, error) {
			return make([]float64, 1024), nil
		},
	}
	handler := NewHandler(mock)

	body, _ := json.Marshal(EmbeddingRequest{Input: "test", Model: "amazon.titan-embed-text-v2:0"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp EmbeddingResponse
	json.NewDecoder(w.Body).Decode(&resp)
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

// --- Batch ---

func TestBatchEmbeddings(t *testing.T) {
	callCount := 0
	mock := &MockEmbedder{
		EmbedFunc: func(ctx context.Context, text string) ([]float64, error) {
			callCount++
			return make([]float64, 1024), nil
		},
	}
	handler := NewHandler(mock)

	body, _ := json.Marshal(EmbeddingRequestBatch{
		Input: []string{"first", "second", "third"},
		Model: "amazon.titan-embed-text-v2:0",
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp EmbeddingResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Data) != 3 {
		t.Fatalf("expected 3 embeddings, got %d", len(resp.Data))
	}
	if callCount != 3 {
		t.Errorf("expected 3 embed calls, got %d", callCount)
	}
	for i, d := range resp.Data {
		if d.Index != i {
			t.Errorf("expected index %d, got %d", i, d.Index)
		}
	}
}

// --- Cohere v4 ---

func TestCohereV4SingleEmbedding(t *testing.T) {
	mock := &MockEmbedder{
		EmbedFunc: func(ctx context.Context, text string) ([]float64, error) {
			return make([]float64, 1536), nil
		},
	}
	handler := NewHandlerWithModel(mock, "cohere.embed-v4:0")

	body, _ := json.Marshal(EmbeddingRequest{Input: "test cohere v4", Model: "cohere.embed-v4:0"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp EmbeddingResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Model != "cohere.embed-v4:0" {
		t.Errorf("expected model 'cohere.embed-v4:0', got '%s'", resp.Model)
	}
	if len(resp.Data) != 1 || len(resp.Data[0].Embedding) != 1536 {
		t.Errorf("expected 1 embedding with 1536 dims")
	}
}

func TestCohereV4BatchEmbeddings(t *testing.T) {
	callCount := 0
	mock := &MockEmbedder{
		EmbedFunc: func(ctx context.Context, text string) ([]float64, error) {
			callCount++
			return make([]float64, 1536), nil
		},
	}
	handler := NewHandlerWithModel(mock, "cohere.embed-v4:0")

	body, _ := json.Marshal(EmbeddingRequestBatch{Input: []string{"a", "b"}, Model: "cohere.embed-v4:0"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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
		t.Errorf("expected 2 calls, got %d", callCount)
	}
}

// --- Default model fallback ---

func TestDefaultModelFallback(t *testing.T) {
	mock := &MockEmbedder{
		EmbedFunc: func(ctx context.Context, text string) ([]float64, error) {
			return make([]float64, 1536), nil
		},
	}
	handler := NewHandlerWithModel(mock, "cohere.embed-v4:0")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader([]byte(`{"input":"test"}`)))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(w, req)

	var resp EmbeddingResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Model != "cohere.embed-v4:0" {
		t.Errorf("expected default model 'cohere.embed-v4:0', got '%s'", resp.Model)
	}
}

func TestModelMismatchReturns400(t *testing.T) {
	mock := &MockEmbedder{
		EmbedFunc: func(ctx context.Context, text string) ([]float64, error) {
			return make([]float64, 256), nil
		},
	}
	// Handler configured with titan, but request sends cohere model
	handler := NewHandler(mock)

	body, _ := json.Marshal(EmbeddingRequest{Input: "test", Model: "cohere.embed-english-v3"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for model mismatch, got %d", w.Code)
	}
	var errResp ErrorResponse
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp.Error.Message == "" {
		t.Error("expected error message about model mismatch")
	}
}

func TestModelMatchProceeds(t *testing.T) {
	mock := &MockEmbedder{
		EmbedFunc: func(ctx context.Context, text string) ([]float64, error) {
			return make([]float64, 256), nil
		},
	}
	handler := NewHandler(mock)

	// Send the matching model — should succeed
	body, _ := json.Marshal(EmbeddingRequest{Input: "test", Model: "amazon.titan-embed-text-v2:0"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for matching model, got %d", w.Code)
	}
	var resp EmbeddingResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Model != "amazon.titan-embed-text-v2:0" {
		t.Errorf("expected model 'amazon.titan-embed-text-v2:0', got '%s'", resp.Model)
	}
}

func TestNoModelProceeds(t *testing.T) {
	mock := &MockEmbedder{
		EmbedFunc: func(ctx context.Context, text string) ([]float64, error) {
			return make([]float64, 256), nil
		},
	}
	handler := NewHandler(mock)

	// No model in request — should succeed with default
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader([]byte(`{"input":"test"}`)))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for no model, got %d", w.Code)
	}
	var resp EmbeddingResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Model != "amazon.titan-embed-text-v2:0" {
		t.Errorf("expected default model, got '%s'", resp.Model)
	}
}

// --- Error handling ---

func TestInvalidMethod(t *testing.T) {
	handler := NewHandler(&MockEmbedder{})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/v1/embeddings", nil))

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestInvalidPath(t *testing.T) {
	handler := NewHandler(&MockEmbedder{})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/completions", nil))

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestEmptyBody(t *testing.T) {
	handler := NewHandler(&MockEmbedder{})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader([]byte{}))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestEmbedderError(t *testing.T) {
	mock := &MockEmbedder{
		EmbedFunc: func(ctx context.Context, text string) ([]float64, error) {
			return nil, &EmbedError{Message: "model not available"}
		},
	}
	handler := NewHandler(mock)

	body, _ := json.Marshal(EmbeddingRequest{Input: "test", Model: "amazon.titan-embed-text-v2:0"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// --- CORS ---

func TestCORSHeaders(t *testing.T) {
	handler := NewHandler(&MockEmbedder{})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodOptions, "/v1/embeddings", nil))

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for OPTIONS, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("missing CORS header")
	}
}

// --- Context propagation ---

func TestCancelledContextReturnsError(t *testing.T) {
	mock := &MockEmbedder{
		EmbedFunc: func(ctx context.Context, text string) ([]float64, error) {
			// Respect the context — if cancelled, return error
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			return make([]float64, 256), nil
		},
	}
	handler := NewHandler(mock)

	body, _ := json.Marshal(EmbeddingRequest{Input: "test", Model: "amazon.titan-embed-text-v2:0"})
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Cancel the context before the request is handled
	ctx, cancel := context.WithCancel(req.Context())
	cancel() // cancel immediately
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for cancelled context, got %d", w.Code)
	}
}

// --- Error sanitization ---

func TestSensitiveErrorsNotLeaked(t *testing.T) {
	sensitiveMsg := "AccessDeniedException: User arn:aws:iam::123456789:role/Foo is not authorized"
	mock := &MockEmbedder{
		EmbedFunc: func(ctx context.Context, text string) ([]float64, error) {
			return nil, fmt.Errorf("%s", sensitiveMsg)
		},
	}
	handler := NewHandler(mock)

	body, _ := json.Marshal(EmbeddingRequest{Input: "test", Model: "amazon.titan-embed-text-v2:0"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}

	respBody := w.Body.String()
	if strings.Contains(respBody, "arn:aws:iam") {
		t.Errorf("response body leaked sensitive error: %s", respBody)
	}
	if strings.Contains(respBody, "AccessDeniedException") {
		t.Errorf("response body leaked AWS error type: %s", respBody)
	}

	var errResp ErrorResponse
	json.NewDecoder(strings.NewReader(respBody)).Decode(&errResp)
	if errResp.Error.Message != "embedding failed" {
		t.Errorf("expected generic 'embedding failed', got '%s'", errResp.Error.Message)
	}
}
