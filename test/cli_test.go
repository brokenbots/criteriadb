package test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/brokenbots/criteriadb/pkg/memory"
	pb "github.com/brokenbots/criteriadb/pkg/pb/criteriadb/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestCLI_ExportImportCycle_PreservesEdges(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPathA := filepath.Join(tmpDir, "source.db")
	dbPathB := filepath.Join(tmpDir, "target.db")
	exportFile := filepath.Join(tmpDir, "backup.json")

	// Phase 1: Populate source database
	engA, err := memory.NewMemoryEngine(memory.Config{StoragePath: dbPathA})
	if err != nil {
		t.Fatalf("Failed to create engA: %v", err)
	}

	node1 := &pb.MemoryNode{
		Id:      "export-node-1",
		Label:   "Primary Node",
		Summary: "Test node 1 for backup",
		Type:    "fact",
	}
	node2 := &pb.MemoryNode{
		Id:      "export-node-2",
		Label:   "Secondary Node",
		Summary: "Test node 2 for backup",
		Type:    "fact",
	}
	_, _ = engA.Remember(ctx, node1, nil)
	_, _ = engA.Remember(ctx, node2, nil)

	edge := &pb.MemoryEdge{
		Id:       "export-edge-1",
		SourceId: "export-node-1",
		TargetId: "export-node-2",
		Relation: "LINKS_TO",
		Weight:   0.75,
	}
	_ = engA.RememberEdge(ctx, edge)
	_ = engA.Close()

	// Phase 2: Perform Export logic
	engExport, err := memory.NewMemoryEngine(memory.Config{StoragePath: dbPathA})
	if err != nil {
		t.Fatalf("Failed to open engExport: %v", err)
	}
	results, err := engExport.Recall(ctx, &pb.QueryRequest{
		ScopeFilter: &pb.AdapterScopeFilter{TargetAdapterIds: []string{"*"}},
		Limit:       1000,
	})
	if err != nil {
		t.Fatalf("Export recall failed: %v", err)
	}

	var rawNodes []json.RawMessage
	var rawEdges []json.RawMessage
	seenEdges := make(map[string]bool)
	m := protojson.MarshalOptions{UseProtoNames: true}

	for _, r := range results {
		if b, err := m.Marshal(r.Node); err == nil {
			rawNodes = append(rawNodes, json.RawMessage(b))
		}
		for _, e := range r.ConnectedEdges {
			if !seenEdges[e.GetId()] {
				seenEdges[e.GetId()] = true
				if eb, err := m.Marshal(e); err == nil {
					rawEdges = append(rawEdges, json.RawMessage(eb))
				}
			}
		}
	}
	_ = engExport.Close()

	exportDoc := map[string]any{
		"nodes": rawNodes,
		"edges": rawEdges,
	}
	data, err := json.MarshalIndent(exportDoc, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal export JSON: %v", err)
	}
	if err := os.WriteFile(exportFile, data, 0644); err != nil {
		t.Fatalf("Failed to write export file: %v", err)
	}

	// Phase 3: Perform Import logic into fresh Database B
	importData, err := os.ReadFile(exportFile)
	if err != nil {
		t.Fatalf("Failed to read export file: %v", err)
	}

	var importedDoc struct {
		Nodes []json.RawMessage `json:"nodes"`
		Edges []json.RawMessage `json:"edges"`
	}
	if err := json.Unmarshal(importData, &importedDoc); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	engB, err := memory.NewMemoryEngine(memory.Config{StoragePath: dbPathB})
	if err != nil {
		t.Fatalf("Failed to create engB: %v", err)
	}

	um := protojson.UnmarshalOptions{DiscardUnknown: true}
	importedNodes := 0
	for _, rawN := range importedDoc.Nodes {
		var node pb.MemoryNode
		if err := um.Unmarshal(rawN, &node); err == nil {
			_, _ = engB.Remember(ctx, &node, nil)
			importedNodes++
		}
	}

	importedEdges := 0
	for _, rawE := range importedDoc.Edges {
		var edge pb.MemoryEdge
		if err := um.Unmarshal(rawE, &edge); err == nil {
			_ = engB.RememberEdge(ctx, &edge)
			importedEdges++
		}
	}

	if importedNodes != 2 {
		t.Errorf("Expected 2 imported nodes, got %d", importedNodes)
	}
	if importedEdges != 1 {
		t.Errorf("Expected 1 imported edge, got %d", importedEdges)
	}

	// Phase 4: Query Database B to verify full graph topology restored
	recalled, err := engB.Recall(ctx, &pb.QueryRequest{
		QueryText: "Primary",
		FactFilter: &pb.FactQuery{
			TargetLabel: "Secondary",
			MaxHops:     1,
		},
	})
	if err != nil || len(recalled) == 0 {
		t.Fatalf("Expected database B to successfully recall connected primary node via fact filter, got %d results (err: %v)", len(recalled), err)
	}

	if len(recalled[0].ConnectedEdges) != 1 {
		t.Errorf("Expected 1 connected edge on primary node, got %d", len(recalled[0].ConnectedEdges))
	}
	if recalled[0].ConnectedEdges[0].GetRelation() != "LINKS_TO" {
		t.Errorf("Expected LINKS_TO relation, got %s", recalled[0].ConnectedEdges[0].GetRelation())
	}

	_ = engB.Close()
}
