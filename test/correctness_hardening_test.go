package test

import (
	"context"
	"math"
	"path/filepath"
	"testing"

	"github.com/brokenbots/criteriadb/pkg/memory"
	pb "github.com/brokenbots/criteriadb/pkg/pb/criteriadb/v1"
)

func TestStorageHardening_ValidationAndEdgeDeduplication(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "hardening_storage.db")

	engine, err := memory.NewMemoryEngine(memory.Config{StoragePath: dbPath})
	if err != nil {
		t.Fatalf("Failed to init engine: %v", err)
	}
	defer engine.Close()

	// 1. Validation: Node with empty ID must be rejected
	_, err = engine.Remember(ctx, &pb.MemoryNode{Id: "   "}, nil)
	if err == nil {
		t.Errorf("Expected error for empty node ID, got nil")
	}

	// 2. Validation: Edge with empty source_id or target_id must be rejected
	err = engine.RememberEdge(ctx, &pb.MemoryEdge{Id: "e1", SourceId: "", TargetId: "n2"})
	if err == nil {
		t.Errorf("Expected error for empty source_id, got nil")
	}
	err = engine.RememberEdge(ctx, &pb.MemoryEdge{Id: "e1", SourceId: "n1", TargetId: ""})
	if err == nil {
		t.Errorf("Expected error for empty target_id, got nil")
	}

	// 3. Node insertion and edge insertion
	node1 := &pb.MemoryNode{Id: "n1", Label: "Node 1", Summary: "Initial summary"}
	node2 := &pb.MemoryNode{Id: "n2", Label: "Node 2", Summary: "Target node"}
	_, _ = engine.Remember(ctx, node1, nil)
	_, _ = engine.Remember(ctx, node2, nil)

	edge := &pb.MemoryEdge{
		Id:       "e1",
		SourceId: "n1",
		TargetId: "n2",
		Relation: "REL_V1",
		Weight:   0.5,
	}
	if err := engine.RememberEdge(ctx, edge); err != nil {
		t.Fatalf("RememberEdge failed: %v", err)
	}

	if stats := engine.GetStats(); stats.TotalEdges != 1 {
		t.Fatalf("Expected 1 edge, got %d", stats.TotalEdges)
	}

	// 4. Edge deduplication / update in place: Re-insert edge with same ID and new relation/weight
	edgeUpdated := &pb.MemoryEdge{
		Id:       "e1",
		SourceId: "n1",
		TargetId: "n2",
		Relation: "REL_V2",
		Weight:   0.9,
	}
	if err := engine.RememberEdge(ctx, edgeUpdated); err != nil {
		t.Fatalf("RememberEdge update failed: %v", err)
	}

	// Verify edges count remains 1 and is NOT duplicated
	if stats := engine.GetStats(); stats.TotalEdges != 1 {
		t.Fatalf("Expected TotalEdges to remain 1 after edge update, got %d", stats.TotalEdges)
	}

	// Verify updated edge content on Recall
	recalled, err := engine.Recall(ctx, &pb.QueryRequest{
		QueryText: "Initial",
	})
	if err != nil || len(recalled) == 0 {
		t.Fatalf("Recall failed: %v", err)
	}
	if len(recalled[0].ConnectedEdges) != 1 {
		t.Fatalf("Expected 1 connected edge on recalled node, got %d", len(recalled[0].ConnectedEdges))
	}
	if recalled[0].ConnectedEdges[0].GetRelation() != "REL_V2" {
		t.Errorf("Expected updated relation REL_V2, got %s", recalled[0].ConnectedEdges[0].GetRelation())
	}
	if recalled[0].ConnectedEdges[0].GetWeight() != 0.9 {
		t.Errorf("Expected updated weight 0.9, got %f", recalled[0].ConnectedEdges[0].GetWeight())
	}
}

func TestQueryHardening_ProtobufMemoryIsolation(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "isolation.db")

	engine, err := memory.NewMemoryEngine(memory.Config{StoragePath: dbPath})
	if err != nil {
		t.Fatalf("Failed to init engine: %v", err)
	}
	defer engine.Close()

	originalNode := &pb.MemoryNode{
		Id:      "iso-1",
		Label:   "Immutable Label",
		Summary: "Initial Uncorrupted Summary",
	}
	_, _ = engine.Remember(ctx, originalNode, nil)

	// Mutate original pointer after Remember
	originalNode.Summary = "MUTATED_AFTER_REMEMBER"

	// Verify that Recall still yields initial summary
	res1, err := engine.Recall(ctx, &pb.QueryRequest{QueryText: "Immutable"})
	if err != nil || len(res1) == 0 {
		t.Fatalf("Recall failed: %v", err)
	}
	if res1[0].Node.GetSummary() != "Initial Uncorrupted Summary" {
		t.Errorf("Memory isolation failed after Remember mutation! Got: %s", res1[0].Node.GetSummary())
	}

	// Mutate returned result node
	res1[0].Node.Summary = "MUTATED_AFTER_RECALL"

	// Verify second Recall still yields uncorrupted summary
	res2, err := engine.Recall(ctx, &pb.QueryRequest{QueryText: "Immutable"})
	if err != nil || len(res2) == 0 {
		t.Fatalf("Second Recall failed: %v", err)
	}
	if res2[0].Node.GetSummary() != "Initial Uncorrupted Summary" {
		t.Errorf("Memory isolation failed after Recall mutation! Got: %s", res2[0].Node.GetSummary())
	}
}

func TestQueryHardening_ConjunctiveLocationFiltering(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "location_conjunctive.db")

	engine, err := memory.NewMemoryEngine(memory.Config{StoragePath: dbPath})
	if err != nil {
		t.Fatalf("Failed to init engine: %v", err)
	}
	defer engine.Close()

	node := &pb.MemoryNode{
		Id:    "loc-node",
		Label: "Backend Server",
		DigitalLocation: &pb.DigitalLocation{
			Project:    "project-alpha",
			FolderPath: "/var/log/app",
			Machine:    "srv-01",
		},
	}
	_, _ = engine.Remember(ctx, node, nil)

	// Case 1: Both project and folder match -> Match
	res, err := engine.Recall(ctx, &pb.QueryRequest{
		LocationFilter: &pb.LocationQuery{
			MatchProject:      "project-alpha",
			MatchFolderPrefix: "/var/log",
		},
	})
	if err != nil || len(res) == 0 {
		t.Fatalf("Expected match when all location filters satisfied, got %d", len(res))
	}

	// Case 2: Project matches but Machine mismatches -> Must NOT match
	resMismatch, err := engine.Recall(ctx, &pb.QueryRequest{
		LocationFilter: &pb.LocationQuery{
			MatchProject: "project-alpha",
			MatchMachine: "srv-02", // Different machine!
		},
	})
	if err != nil {
		t.Fatalf("Recall error: %v", err)
	}
	if len(resMismatch) > 0 {
		t.Fatalf("Expected 0 results for partial location mismatch under conjunctive filtering, got %d", len(resMismatch))
	}
}

func TestQueryHardening_CosineSimilarityNaNInf(t *testing.T) {
	// Cosine similarity with zero norms
	simZero := memory.CosineSimilarity([]float32{0, 0, 0}, []float32{1, 2, 3})
	if simZero != 0.0 {
		t.Errorf("Expected 0.0 for zero norm vector, got %f", simZero)
	}

	// Dimension mismatch
	simDim := memory.CosineSimilarity([]float32{1, 2}, []float32{1, 2, 3})
	if simDim != 0.0 {
		t.Errorf("Expected 0.0 for dimension mismatch, got %f", simDim)
	}

	// NaN vector
	nanVal := float32(math.NaN())
	simNaN := memory.CosineSimilarity([]float32{nanVal, 1.0}, []float32{1.0, 1.0})
	if simNaN != 0.0 || math.IsNaN(simNaN) {
		t.Errorf("Expected 0.0 for NaN vector input, got %f", simNaN)
	}
}

func TestEmbedderHardening_InvalidURLScheme(t *testing.T) {
	ctx := context.Background()

	// Embedder with invalid scheme
	embedder := memory.NewEmbedder("ftp://localhost:11434", "test")
	_, err := embedder.GenerateEmbedding(ctx, "hello world")
	if err == nil {
		t.Fatalf("Expected error for non-http/https endpoint, got nil")
	}

	// Embedder with empty host
	embedderBad := memory.NewEmbedder("http://", "test")
	_, err = embedderBad.GenerateEmbedding(ctx, "hello world")
	if err == nil {
		t.Fatalf("Expected error for empty host URL, got nil")
	}
}
