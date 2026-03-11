package embedrock

import (
	"context"
	"encoding/json"

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

// Embed generates an embedding vector for the given text.
func (b *BedrockEmbedder) Embed(text string) ([]float64, error) {
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

func strPtr(s string) *string { return &s }
