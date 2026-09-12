package test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	v2 "github.com/brokenbots/criteria-adapter-proto/criteria/v2"
	"github.com/brokenbots/criteriadb/pkg/adapter"
	"github.com/brokenbots/criteriadb/pkg/memory"
	pb "github.com/brokenbots/criteriadb/pkg/pb/criteriadb/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type mockSender struct {
	events []*v2.ExecuteEvent
}

func (m *mockSender) Send(ev *v2.ExecuteEvent) error {
	m.events = append(m.events, ev)
	return nil
}

func TestCriteriaDBAdapter(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "adapter_test.db")

	engine, err := memory.NewMemoryEngine(memory.Config{
		StoragePath: dbPath,
	})
	if err != nil {
		t.Fatalf("Failed to init memory engine: %v", err)
	}
	defer engine.Close()

	ad := adapter.NewCriteriaDBAdapter(engine)

	// Test Info
	info, err := ad.Info(ctx, &v2.InfoRequest{})
	if err != nil || info.Name != "criteriadb" {
		t.Fatalf("Unexpected Info response: %v", info)
	}

	// Test Remember Action
	senderRem := &mockSender{}
	err = ad.Execute(ctx, &v2.ExecuteRequest{
		Input: map[string]string{
			"action":     "remember",
			"label":      "Build Criteria Engine",
			"summary":    "Compiled FSM graph engine in Go",
			"type":       "task_done",
			"project":    "criteria-core",
			"adapter_id": "copilot-1",
		},
	}, senderRem)
	if err != nil {
		t.Fatalf("Execute remember failed: %v", err)
	}

	if len(senderRem.events) == 0 || senderRem.events[0].GetResult().GetOutcome() != "remembered" {
		t.Fatalf("Expected remembered outcome event")
	}

	// Test Recall Action
	senderRec := &mockSender{}
	err = ad.Execute(ctx, &v2.ExecuteRequest{
		Input: map[string]string{
			"action":            "recall",
			"query_text":        "Compiled FSM engine",
			"caller_adapter_id": "copilot-1",
		},
	}, senderRec)
	if err != nil {
		t.Fatalf("Execute recall failed: %v", err)
	}

	if len(senderRec.events) == 0 || senderRec.events[0].GetResult().GetOutcome() != "recalled" {
		t.Fatalf("Expected recalled outcome event")
	}
}

func TestPersistenceAcrossCloseReopen(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "persistence_test.db")

	// Phase 1: Initialize DB, store 2 event nodes, and consolidate them
	eng1, err := memory.NewMemoryEngine(memory.Config{StoragePath: dbPath})
	if err != nil {
		t.Fatalf("Failed to init eng1: %v", err)
	}

	node1 := &pb.MemoryNode{
		Id:              "e1",
		Label:           "Event 1",
		Summary:         "Build step completed",
		Type:            "build_event",
		DigitalLocation: &pb.DigitalLocation{Project: "p1"},
	}
	node2 := &pb.MemoryNode{
		Id:              "e2",
		Label:           "Event 2",
		Summary:         "Test step completed",
		Type:            "build_event",
		DigitalLocation: &pb.DigitalLocation{Project: "p1"},
	}

	_, _ = eng1.Remember(ctx, node1, nil)
	_, _ = eng1.Remember(ctx, node2, nil)

	consNode, count, err := eng1.Consolidate(ctx, "p1", "build_event")
	if err != nil || count != 2 {
		t.Fatalf("Consolidation failed: %v, count=%d", err, count)
	}

	if err := eng1.Close(); err != nil {
		t.Fatalf("Close eng1 failed: %v", err)
	}

	// Phase 2: Reopen DB and verify raw events e1 and e2 are NOT resurrected
	eng2, err := memory.NewMemoryEngine(memory.Config{StoragePath: dbPath})
	if err != nil {
		t.Fatalf("Failed to init eng2: %v", err)
	}
	defer eng2.Close()

	stats := eng2.GetStats()
	if stats.TotalNodes != 1 {
		t.Fatalf("Expected exactly 1 consolidated node after reopen, got %d", stats.TotalNodes)
	}

	results, err := eng2.Recall(ctx, &pb.QueryRequest{
		ScopeFilter: &pb.AdapterScopeFilter{TargetAdapterIds: []string{"*"}},
		Limit:       10,
	})
	if err != nil || len(results) != 1 {
		t.Fatalf("Expected 1 recall result, got %d", len(results))
	}

	if results[0].Node.GetId() != consNode.GetId() {
		t.Fatalf("Expected node ID %s, got %s", consNode.GetId(), results[0].Node.GetId())
	}
}

func TestRememberRelationWithoutOverwritingNode(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "relation_test.db")

	eng, err := memory.NewMemoryEngine(memory.Config{StoragePath: dbPath})
	if err != nil {
		t.Fatalf("Failed to init memory engine: %v", err)
	}
	defer eng.Close()

	ad := adapter.NewCriteriaDBAdapter(eng)

	// Step 1: Create a source node with rich fields
	node := &pb.MemoryNode{
		Id:      "src-100",
		Label:   "Original Source Node Label",
		Summary: "Rich summary text for source node",
		Type:    "fact",
		Temporal: &pb.TemporalInfo{
			Timestamp: timestamppb.New(time.Now()),
			Tense:     pb.Tense_TENSE_PAST,
		},
	}
	_, err = eng.Remember(ctx, node, nil)
	if err != nil {
		t.Fatalf("Remember source node failed: %v", err)
	}

	// Step 2: Invoke remember_relation via adapter
	senderRel := &mockSender{}
	err = ad.Execute(ctx, &v2.ExecuteRequest{
		Input: map[string]string{
			"action":         "remember_relation",
			"source_node_id": "src-100",
			"target_node_id": "tgt-200",
			"relation":       "DEPENDS_ON",
			"weight":         "0.85",
		},
	}, senderRel)
	if err != nil {
		t.Fatalf("Execute remember_relation failed: %v", err)
	}

	// Step 3: Verify the original source node was NOT overwritten
	results, err := eng.Recall(ctx, &pb.QueryRequest{
		ScopeFilter: &pb.AdapterScopeFilter{TargetAdapterIds: []string{"*"}},
		Limit:       10,
	})
	if err != nil || len(results) == 0 {
		t.Fatalf("Recall failed: %v", err)
	}

	foundSrc := false
	for _, r := range results {
		if r.Node.GetId() == "src-100" {
			foundSrc = true
			if r.Node.GetLabel() != "Original Source Node Label" {
				t.Fatalf("Source node label was overwritten! Got: %s", r.Node.GetLabel())
			}
			if r.Node.GetSummary() != "Rich summary text for source node" {
				t.Fatalf("Source node summary was overwritten! Got: %s", r.Node.GetSummary())
			}
		}
	}
	if !foundSrc {
		t.Fatalf("Source node src-100 not found")
	}
}
