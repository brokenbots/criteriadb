package test

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/brokenbots/criteriadb/pkg/memory"
	pb "github.com/brokenbots/criteriadb/pkg/pb/criteriadb/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func createBenchmarkEngine(b *testing.B) (*memory.MemoryEngine, func()) {
	tmpDir, err := os.MkdirTemp("", "criteriadb-bench-*")
	if err != nil {
		b.Fatalf("failed to create temp dir: %v", err)
	}
	dbPath := fmt.Sprintf("%s/bench.db", tmpDir)

	eng, err := memory.NewMemoryEngine(memory.Config{
		StoragePath: dbPath,
	})
	if err != nil {
		os.RemoveAll(tmpDir)
		b.Fatalf("failed to create memory engine: %v", err)
	}

	cleanup := func() {
		_ = eng.Close()
		_ = os.RemoveAll(tmpDir)
	}
	return eng, cleanup
}

func generateRandomNode(id string) *pb.MemoryNode {
	t := memory.NowTimestamp()
	return &pb.MemoryNode{
		Id:      id,
		Label:   fmt.Sprintf("Benchmark Fact %s", id),
		Summary: fmt.Sprintf("Performance testing memory node %s with random tokens alpha beta gamma delta epsilon", id),
		Type:    "fact",
		Temporal: &pb.TemporalInfo{
			Timestamp: timestamppb.New(t),
			ValidFrom: timestamppb.New(t),
			Tense:     pb.Tense_TENSE_PRESENT,
		},
		DigitalLocation: &pb.DigitalLocation{
			Project:    "criteriadb-benchmarks",
			FolderPath: "pkg/memory",
			FilePath:   "engine.go",
		},
		AgentScope: &pb.AgentScope{
			CreatorAdapterId: "bench-agent",
			Visibility:       pb.VisibilityScope_VISIBILITY_WORKFLOW,
		},
	}
}

func generateRandomVector(dim int) []float32 {
	vec := make([]float32, dim)
	for i := range vec {
		vec[i] = rand.Float32()
	}
	return vec
}

// BenchmarkEngine_Remember measures write throughput for inserting memory nodes into bbolt.
func BenchmarkEngine_Remember(b *testing.B) {
	eng, cleanup := createBenchmarkEngine(b)
	defer cleanup()

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		nodeID := fmt.Sprintf("bench-node-%d", i)
		node := generateRandomNode(nodeID)
		_, err := eng.Remember(ctx, node, nil)
		if err != nil {
			b.Fatalf("Remember failed at index %d: %v", i, err)
		}
	}
}

// BenchmarkEngine_Recall_Lexical_1k measures zero-LLM lexical token query performance over 1,000 pre-populated nodes.
func BenchmarkEngine_Recall_Lexical_1k(b *testing.B) {
	eng, cleanup := createBenchmarkEngine(b)
	defer cleanup()

	ctx := context.Background()
	// Pre-populate 1,000 nodes
	for i := 0; i < 1000; i++ {
		nodeID := fmt.Sprintf("pre-1k-%d", i)
		node := generateRandomNode(nodeID)
		if i%10 == 0 {
			node.Summary += " special_token_query_target"
		}
		_, _ = eng.Remember(ctx, node, nil)
	}

	b.ResetTimer()

	req := &pb.QueryRequest{
		QueryText: "special_token_query_target performance alpha",
		Limit:     10,
	}

	for i := 0; i < b.N; i++ {
		results, err := eng.Recall(ctx, req)
		if err != nil || len(results) == 0 {
			b.Fatalf("Recall failed or returned zero results: %v", err)
		}
	}
}

// BenchmarkEngine_Recall_Vector_1k measures 768D float32 SIMD/goroutine vector cosine search over 1,000 vector nodes.
func BenchmarkEngine_Recall_Vector_1k(b *testing.B) {
	eng, cleanup := createBenchmarkEngine(b)
	defer cleanup()

	ctx := context.Background()
	// Pre-populate 1,000 nodes with 768D vectors
	for i := 0; i < 1000; i++ {
		nodeID := fmt.Sprintf("vec-1k-%d", i)
		node := generateRandomNode(nodeID)
		vec := generateRandomVector(768)
		node.PackedEmbedding = memory.PackFloats(vec)
		node.VectorDimension = int32(len(vec))
		_, _ = eng.Remember(ctx, node, nil)
	}

	b.ResetTimer()

	queryVec := generateRandomVector(768)
	req := &pb.QueryRequest{
		PackedQueryEmbedding: memory.PackFloats(queryVec),
		Limit:                10,
	}

	for i := 0; i < b.N; i++ {
		results, err := eng.Recall(ctx, req)
		if err != nil || len(results) == 0 {
			b.Fatalf("Vector recall failed: %v", err)
		}
	}
}

// BenchmarkEngine_Consolidate measures consolidation & pruning execution over a populated graph.
func BenchmarkEngine_Consolidate(b *testing.B) {
	eng, cleanup := createBenchmarkEngine(b)
	defer cleanup()

	ctx := context.Background()
	// Pre-populate 500 nodes with expired validity windows
	now := time.Now()
	for i := 0; i < 500; i++ {
		nodeID := fmt.Sprintf("expired-%d", i)
		node := generateRandomNode(nodeID)
		node.Temporal.ValidTo = timestamppb.New(now.Add(-10 * time.Minute))
		_, _ = eng.Remember(ctx, node, nil)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _, err := eng.Consolidate(ctx, "criteriadb-benchmarks", "fact")
		if err != nil {
			b.Fatalf("Consolidate failed: %v", err)
		}
	}
}
