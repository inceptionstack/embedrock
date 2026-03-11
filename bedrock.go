package embedrock

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

// BedrockEmbedder implements Embedder using Amazon Bedrock.
type BedrockEmbedder struct {
	client  *bedrockruntime.Client
	modelID string
}

// titanRequest is the Titan Embed input format.
type titanRequest struct {
	InputText string `json:"inputText"`
}

// titanResponse is the Titan Embed output format.
type titanResponse struct {
	Embedding []float64 `json:"embedding"`
}

// cohereRequest is the Cohere Embed input format.
type cohereRequest struct {
	Texts     []string `json:"texts"`
	InputType string   `json:"input_type"`
}

// cohereResponse is the Cohere Embed v3 output format (flat array).
type cohereResponse struct {
	Embeddings [][]float64 `json:"embeddings"`
}

// cohereV4Response is the Cohere Embed v4 output format (dict of typed arrays).
type cohereV4Response struct {
	Embeddings cohereV4Embeddings `json:"embeddings"`
}

type cohereV4Embeddings struct {
	Float [][]float64 `json:"float"`
}

// NewBedrockEmbedder creates an embedder backed by Bedrock.
func NewBedrockEmbedder(region, modelID string) (*BedrockEmbedder, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, err
	}

	client := bedrockruntime.NewFromConfig(cfg)
	return &BedrockEmbedder{
		client:  client,
		modelID: modelID,
	}, nil
}

// isCohere returns true if the model ID is a Cohere model.
func isCohere(modelID string) bool {
	return strings.HasPrefix(modelID, "cohere.")
}

// isCohereV4 returns true if the model ID is Cohere Embed v4.
func isCohereV4(modelID string) bool {
	return strings.HasPrefix(modelID, "cohere.embed-v4")
}

// Embed generates an embedding vector for the given text.
func (b *BedrockEmbedder) Embed(text string) ([]float64, error) {
	if isCohere(b.modelID) {
		return b.embedCohere(text)
	}
	return b.embedTitan(text)
}

func (b *BedrockEmbedder) embedTitan(text string) ([]float64, error) {
	input, err := json.Marshal(titanRequest{InputText: text})
	if err != nil {
		return nil, err
	}

	resp, err := b.client.InvokeModel(context.Background(), &bedrockruntime.InvokeModelInput{
		ModelId:     &b.modelID,
		ContentType: strPtr("application/json"),
		Accept:      strPtr("application/json"),
		Body:        input,
	})
	if err != nil {
		return nil, err
	}

	var result titanResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, err
	}

	return result.Embedding, nil
}

func (b *BedrockEmbedder) embedCohere(text string) ([]float64, error) {
	input, err := json.Marshal(cohereRequest{
		Texts:     []string{text},
		InputType: "search_query",
	})
	if err != nil {
		return nil, err
	}

	resp, err := b.client.InvokeModel(context.Background(), &bedrockruntime.InvokeModelInput{
		ModelId:     &b.modelID,
		ContentType: strPtr("application/json"),
		Accept:      strPtr("application/json"),
		Body:        input,
	})
	if err != nil {
		return nil, err
	}

	// Cohere v4 returns {"embeddings": {"float": [[...]]}}
	// Cohere v3 returns {"embeddings": [[...]]}
	if isCohereV4(b.modelID) {
		var result cohereV4Response
		if err := json.Unmarshal(resp.Body, &result); err != nil {
			return nil, fmt.Errorf("failed to parse Cohere v4 response: %w", err)
		}
		if len(result.Embeddings.Float) == 0 {
			return nil, &EmbedError{Message: "Cohere v4 returned no embeddings"}
		}
		return result.Embeddings.Float[0], nil
	}

	var result cohereResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse Cohere response: %w", err)
	}
	if len(result.Embeddings) == 0 {
		return nil, &EmbedError{Message: "Cohere returned no embeddings"}
	}
	return result.Embeddings[0], nil
}

func strPtr(s string) *string { return &s }
