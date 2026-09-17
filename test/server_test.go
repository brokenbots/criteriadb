package test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/brokenbots/criteriadb/pkg/memory"
	pb "github.com/brokenbots/criteriadb/pkg/pb/criteriadb/v1"
	"github.com/brokenbots/criteriadb/pkg/server"
)

func TestVisualizerServer(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "viz_test.db")

	engine, err := memory.NewMemoryEngine(memory.Config{
		StoragePath: dbPath,
	})
	if err != nil {
		t.Fatalf("Failed to init memory engine: %v", err)
	}
	defer engine.Close()

	// Populate engine with 2 nodes and 1 edge
	node1 := &pb.MemoryNode{
		Id:      "viz-node-1",
		Label:   "Service Alpha",
		Summary: "Core Auth API",
		Type:    "service",
		DigitalLocation: &pb.DigitalLocation{
			Project: "cloud-core",
		},
		AgentScope: &pb.AgentScope{
			CreatorAdapterId: "agent-1",
			Visibility:       pb.VisibilityScope_VISIBILITY_GLOBAL,
		},
	}
	node2 := &pb.MemoryNode{
		Id:      "viz-node-2",
		Label:   "Service Beta",
		Summary: "Billing API",
		Type:    "service",
		DigitalLocation: &pb.DigitalLocation{
			Project: "cloud-core",
		},
		AgentScope: &pb.AgentScope{
			CreatorAdapterId: "agent-2",
			Visibility:       pb.VisibilityScope_VISIBILITY_GLOBAL,
		},
	}
	_, _ = engine.Remember(ctx, node1, nil)
	_, _ = engine.Remember(ctx, node2, nil)

	edge := &pb.MemoryEdge{
		Id:       "edge-1-2",
		SourceId: "viz-node-1",
		TargetId: "viz-node-2",
		Relation: "CALLS",
		Weight:   0.9,
	}
	_ = engine.RememberEdge(ctx, edge)

	// Start web visualizer on random high port
	port := 18980
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	go func() {
		_ = server.StartWebServer(addr, engine)
	}()

	// Wait briefly for server to bind
	time.Sleep(150 * time.Millisecond)

	baseURL := fmt.Sprintf("http://%s", addr)

	// Test 1: Fetch HTML dashboard
	t.Run("HTML Dashboard", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/")
		if err != nil {
			t.Fatalf("Failed to GET dashboard: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read body: %v", err)
		}

		if len(body) == 0 {
			t.Errorf("Expected non-empty HTML body")
		}
	})

	// Test 2: Fetch /api/graph JSON
	t.Run("API Graph Endpoint & Deduplication", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/graph")
		if err != nil {
			t.Fatalf("Failed to GET /api/graph: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		var data server.GraphData
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			t.Fatalf("Failed to decode graph JSON: %v", err)
		}

		if len(data.Nodes) != 2 {
			t.Errorf("Expected 2 nodes in graph, got %d", len(data.Nodes))
		}

		// Ensure link is present and NOT duplicated
		if len(data.Links) != 1 {
			t.Errorf("Expected exactly 1 deduplicated link in graph, got %d", len(data.Links))
		}

		if len(data.Links) > 0 {
			link := data.Links[0]
			if link.Source != "viz-node-1" || link.Target != "viz-node-2" || link.Relation != "CALLS" {
				t.Errorf("Unexpected link attributes: %+v", link)
			}
		}
	})
}
