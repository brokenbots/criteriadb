package test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/brokenbots/criteriadb/pkg/memory"
	pb "github.com/brokenbots/criteriadb/pkg/pb/criteriadb/v1"
)

func TestLocalHTTPEmbeddingEndpoint(t *testing.T) {
	ctx := context.Background()

	// 1. Spin up mock local HTTP embedding server (OpenAI / Ollama compatible)
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		// Return 4-dimensional mock vector embedding [0.2, 0.8, 0.4, 0.9]
		resp := map[string]any{
			"data": []map[string]any{
				{
					"embedding": []float32{0.2, 0.8, 0.4, 0.9},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	// 2. Initialize Memory Engine configured with mock local HTTP embedding endpoint
	engine, err := memory.NewMemoryEngine(memory.Config{
		EmbeddingEndpoint: mockServer.URL,
		EmbeddingModel:    "test-embedding-model",
	})
	if err != nil {
		t.Fatalf("Failed to initialize engine: %v", err)
	}
	defer engine.Close()

	// 3. Test auto-embedding text on Remember
	node := &pb.MemoryNode{
		Id:      "embedded-node-1",
		Label:   "Neural Embedding Test",
		Summary: "Automated vector embedding generation via local HTTP endpoint",
	}

	id, err := engine.Remember(ctx, node, nil)
	if err != nil || id != "embedded-node-1" {
		t.Fatalf("Failed to remember node: %v", err)
	}

	// Verify that packed_embedding was auto-populated by local HTTP endpoint
	if len(node.GetPackedEmbedding()) == 0 {
		t.Fatalf("Expected packed_embedding to be auto-populated from local HTTP endpoint, got 0 bytes")
	}

	unpackedVec := memory.UnpackFloats(node.GetPackedEmbedding())
	if len(unpackedVec) != 4 {
		t.Fatalf("Expected 4-dim vector, got %d dimensions", len(unpackedVec))
	}
	if unpackedVec[0] != 0.2 || unpackedVec[1] != 0.8 {
		t.Errorf("Unexpected vector values: %v", unpackedVec)
	}

	// 4. Test Recall with query text auto-embedded via local HTTP endpoint
	results, err := engine.Recall(ctx, &pb.QueryRequest{
		QueryText: "Neural Embedding",
	})
	if err != nil || len(results) == 0 {
		t.Fatalf("Failed to recall auto-embedded memory node: %v", err)
	}

	if results[0].Node.GetId() != "embedded-node-1" {
		t.Errorf("Expected top result embedded-node-1, got %s", results[0].Node.GetId())
	}
}
