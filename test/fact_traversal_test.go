package test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/brokenbots/criteriadb/pkg/memory"
	pb "github.com/brokenbots/criteriadb/pkg/pb/criteriadb/v1"
)

func TestFactFilter_MultiHopAndRelationTypes(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "fact_traversal_test.db")

	engine, err := memory.NewMemoryEngine(memory.Config{
		StoragePath: dbPath,
	})
	if err != nil {
		t.Fatalf("Failed to init memory engine: %v", err)
	}
	defer engine.Close()

	// Setup graph:
	// [Node A: "Frontend UI Component"]
	//       │
	//       ▼ (DEPENDS_ON)
	// [Node B: "GraphQL Gateway"]
	//       │
	//       ▼ (CALLS)
	// [Node C: "Payment Billing Microservice"]
	nodeA := &pb.MemoryNode{
		Id:      "node-a",
		Label:   "Frontend UI Component",
		Summary: "React user checkout screen",
		Type:    "component",
	}
	nodeB := &pb.MemoryNode{
		Id:      "node-b",
		Label:   "GraphQL Gateway",
		Summary: "API router and aggregator",
		Type:    "service",
	}
	nodeC := &pb.MemoryNode{
		Id:      "node-c",
		Label:   "Payment Billing Microservice",
		Summary: "Stripe and credit card processing engine",
		Type:    "backend",
	}

	_, _ = engine.Remember(ctx, nodeA, nil)
	_, _ = engine.Remember(ctx, nodeB, nil)
	_, _ = engine.Remember(ctx, nodeC, nil)

	// Add edges
	edgeAB := &pb.MemoryEdge{
		Id:       "edge-ab",
		SourceId: "node-a",
		TargetId: "node-b",
		Relation: "DEPENDS_ON",
	}
	edgeBC := &pb.MemoryEdge{
		Id:       "edge-bc",
		SourceId: "node-b",
		TargetId: "node-c",
		Relation: "CALLS",
	}
	_ = engine.RememberEdge(ctx, edgeAB)
	_ = engine.RememberEdge(ctx, edgeBC)

	// Test 1: Direct 1-Hop query from Node A targeting "GraphQL Gateway"
	t.Run("Direct 1-Hop Traversal", func(t *testing.T) {
		res, err := engine.Recall(ctx, &pb.QueryRequest{
			QueryText: "Frontend",
			FactFilter: &pb.FactQuery{
				TargetLabel:   "GraphQL",
				MaxHops:       1,
				RelationTypes: []string{"DEPENDS_ON"},
			},
		})
		if err != nil || len(res) == 0 {
			t.Fatalf("Expected Node A to match 1-hop target, got %d results (err: %v)", len(res), err)
		}
		if res[0].Node.GetId() != "node-a" {
			t.Errorf("Expected node-a, got %s", res[0].Node.GetId())
		}
	})

	// Test 2: 2-Hop Transitive query from Node A targeting "Billing Microservice"
	t.Run("Transitive 2-Hop Traversal Success", func(t *testing.T) {
		res, err := engine.Recall(ctx, &pb.QueryRequest{
			QueryText: "Frontend",
			FactFilter: &pb.FactQuery{
				TargetLabel:   "Payment Billing",
				MaxHops:       2,
				RelationTypes: []string{"DEPENDS_ON", "CALLS"},
			},
		})
		if err != nil || len(res) == 0 {
			t.Fatalf("Expected Node A to match 2-hop target, got %d results (err: %v)", len(res), err)
		}
		if res[0].Node.GetId() != "node-a" {
			t.Errorf("Expected node-a, got %s", res[0].Node.GetId())
		}
	})

	// Test 3: Insufficient max_hops (max_hops=1 cannot reach Node C from Node A)
	t.Run("Transitive 2-Hop Blocked by MaxHops=1", func(t *testing.T) {
		res, err := engine.Recall(ctx, &pb.QueryRequest{
			QueryText: "Frontend",
			FactFilter: &pb.FactQuery{
				TargetLabel:   "Payment Billing",
				MaxHops:       1,
				RelationTypes: []string{"DEPENDS_ON", "CALLS"},
			},
		})
		if err != nil {
			t.Fatalf("Recall error: %v", err)
		}
		if len(res) > 0 {
			t.Errorf("Expected 0 results when max_hops is insufficient, got %d", len(res))
		}
	})

	// Test 4: Relation type mismatch (CALLS is omitted from allowed relations)
	t.Run("Relation Type Filtering Mismatch", func(t *testing.T) {
		res, err := engine.Recall(ctx, &pb.QueryRequest{
			QueryText: "Frontend",
			FactFilter: &pb.FactQuery{
				TargetLabel:   "Payment Billing",
				MaxHops:       2,
				RelationTypes: []string{"DEPENDS_ON"}, // Does NOT allow CALLS
			},
		})
		if err != nil {
			t.Fatalf("Recall error: %v", err)
		}
		if len(res) > 0 {
			t.Errorf("Expected 0 results due to unallowed relation type, got %d", len(res))
		}
	})

	// Test 5: Relation type only filter (find nodes that have CALLS edges)
	t.Run("Relation Type Only Filter", func(t *testing.T) {
		res, err := engine.Recall(ctx, &pb.QueryRequest{
			FactFilter: &pb.FactQuery{
				RelationTypes: []string{"CALLS"},
				MaxHops:       1,
			},
			Limit: 10,
		})
		if err != nil || len(res) == 0 {
			t.Fatalf("Expected matches for CALLS relation, got %d", len(res))
		}
		// Nodes B and C are connected via CALLS
		foundB := false
		for _, r := range res {
			if r.Node.GetId() == "node-b" || r.Node.GetId() == "node-c" {
				foundB = true
			}
		}
		if !foundB {
			t.Errorf("Expected node-b or node-c to match relation CALLS")
		}
	})
}
