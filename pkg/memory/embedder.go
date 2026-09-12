package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Embedder fetches vector embeddings from a local HTTP endpoint (e.g. Ollama or local OpenAI endpoint).
type Embedder struct {
	EndpointURL string
	ModelName   string
	Client      *http.Client
}

type openAIEmbeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type openAIEmbeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

func NewEmbedder(endpointURL, modelName string) *Embedder {
	if modelName == "" {
		modelName = "nomic-embed-text"
	}
	return &Embedder{
		EndpointURL: endpointURL,
		ModelName:   modelName,
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GenerateEmbedding calls the local HTTP endpoint to embed text. Returns nil if failed or endpoint is empty.
func (e *Embedder) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	if e == nil || e.EndpointURL == "" || text == "" {
		return nil, nil
	}

	reqPayload := openAIEmbeddingRequest{
		Model: e.ModelName,
		Input: text,
	}

	bodyBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embedding payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.EndpointURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call embedding endpoint %s: %w", e.EndpointURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding endpoint returned status: %s", resp.Status)
	}

	var res openAIEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(res.Data) == 0 || len(res.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}

	return res.Data[0].Embedding, nil
}
