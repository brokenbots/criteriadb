package test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/brokenbots/criteriadb/pkg/memory"
	pb "github.com/brokenbots/criteriadb/pkg/pb/criteriadb/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestConsolidationAndPruning(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "consolidation_test.db")

	engine, err := memory.NewMemoryEngine(memory.Config{
		StoragePath: dbPath,
	})
	if err != nil {
		t.Fatalf("Failed to initialize engine: %v", err)
	}
	defer engine.Close()

	now := time.Now()

	// 1. Insert an expired node (valid_to in the past) and an edge connected to it
	expiredNode := &pb.MemoryNode{
		Id:    "exp-1",
		Label: "Expired Temporary Event",
		Type:  "temp_event",
		Temporal: &pb.TemporalInfo{
			ValidTo: timestamppb.New(now.Add(-1 * time.Hour)),
		},
	}
	_, _ = engine.Remember(ctx, expiredNode, nil)

	// 2. Insert 3 repetitive event nodes for consolidation with edges
	for i := 1; i <= 3; i++ {
		n := &pb.MemoryNode{
			Id:    f("evt-%d", i),
			Label: f("Build Log Event #%d", i),
			Type:  "build_log",
			DigitalLocation: &pb.DigitalLocation{
				Project: "criteria-core",
			},
		}
		_, _ = engine.Remember(ctx, n, nil)
	}

	// Connect expired node to evt-1
	_ = engine.RememberEdge(ctx, &pb.MemoryEdge{
		Id:       "edge-exp",
		SourceId: "exp-1",
		TargetId: "evt-1",
		Relation: "CAUSED",
	})
	// Connect evt-1 to evt-2
	_ = engine.RememberEdge(ctx, &pb.MemoryEdge{
		Id:       "edge-evt-1-2",
		SourceId: "evt-1",
		TargetId: "evt-2",
		Relation: "FOLLOWED_BY",
	})

	// 3. Test Stats before pruning
	statsBefore := engine.GetStats()
	if statsBefore.TotalNodes != 4 {
		t.Fatalf("Expected 4 total nodes before pruning, got %d", statsBefore.TotalNodes)
	}
	if statsBefore.TotalEdges != 2 {
		t.Fatalf("Expected 2 total edges before pruning, got %d", statsBefore.TotalEdges)
	}

	// 4. Test Pruning (should prune exp-1 node AND edge-exp connected edge)
	pruned, err := engine.PruneExpired(ctx)
	if err != nil || pruned != 1 {
		t.Fatalf("Expected 1 node pruned, got %d (err: %v)", pruned, err)
	}
	statsAfterPrune := engine.GetStats()
	if statsAfterPrune.TotalEdges != 1 {
		t.Fatalf("Expected 1 edge remaining after pruning expired node, got %d", statsAfterPrune.TotalEdges)
	}

	// 5. Test Consolidation (should consolidate evt-1, evt-2, evt-3 and remove edge-evt-1-2)
	consNode, count, err := engine.Consolidate(ctx, "criteria-core", "build_log")
	if err != nil || count != 3 || consNode == nil {
		t.Fatalf("Expected 3 nodes consolidated into 1, got %d (err: %v)", count, err)
	}

	// 6. Test Stats after consolidation: 1 consolidated node and 0 orphaned edges remaining
	statsAfter := engine.GetStats()
	if statsAfter.TotalNodes != 1 {
		t.Errorf("Expected 1 consolidated node remaining, got %d", statsAfter.TotalNodes)
	}
	if statsAfter.TotalEdges != 0 {
		t.Errorf("Expected 0 dangling edges remaining after consolidation, got %d", statsAfter.TotalEdges)
	}
}

func f(format string, a ...any) string {
	return fmt.Sprintf(format, a...)
}
