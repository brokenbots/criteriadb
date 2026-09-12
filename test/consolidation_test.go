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

	// 1. Insert an expired node (valid_to in the past)
	expiredNode := &pb.MemoryNode{
		Id:    "exp-1",
		Label: "Expired Temporary Event",
		Type:  "temp_event",
		Temporal: &pb.TemporalInfo{
			ValidTo: timestamppb.New(now.Add(-1 * time.Hour)),
		},
	}
	_, _ = engine.Remember(ctx, expiredNode, nil)

	// 2. Insert 3 repetitive event nodes for consolidation
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

	// 3. Test Stats before pruning
	statsBefore := engine.GetStats()
	if statsBefore.TotalNodes != 4 {
		t.Fatalf("Expected 4 total nodes before pruning, got %d", statsBefore.TotalNodes)
	}

	// 4. Test Pruning
	pruned, err := engine.PruneExpired(ctx)
	if err != nil || pruned != 1 {
		t.Fatalf("Expected 1 node pruned, got %d (err: %v)", pruned, err)
	}

	// 5. Test Consolidation
	consNode, count, err := engine.Consolidate(ctx, "criteria-core", "build_log")
	if err != nil || count != 3 || consNode == nil {
		t.Fatalf("Expected 3 nodes consolidated into 1, got %d (err: %v)", count, err)
	}

	// 6. Test Stats after consolidation
	statsAfter := engine.GetStats()
	if statsAfter.TotalNodes != 1 {
		t.Errorf("Expected 1 consolidated node remaining, got %d", statsAfter.TotalNodes)
	}
}

func f(format string, a ...any) string {
	return fmt.Sprintf(format, a...)
}
