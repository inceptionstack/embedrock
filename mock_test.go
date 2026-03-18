package embedrock

import "context"

// MockEmbedder is a test double for the Embedder interface.
type MockEmbedder struct {
	EmbedFunc func(ctx context.Context, text string) ([]float64, error)
}

func (m *MockEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
	if m.EmbedFunc != nil {
		return m.EmbedFunc(ctx, text)
	}
	return make([]float64, 1024), nil
}
