package embedrock

// MockEmbedder is a test double for the Embedder interface.
type MockEmbedder struct {
	EmbedFunc func(text string) ([]float64, error)
}

func (m *MockEmbedder) Embed(text string) ([]float64, error) {
	if m.EmbedFunc != nil {
		return m.EmbedFunc(text)
	}
	return make([]float64, 1024), nil
}
