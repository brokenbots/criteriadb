package test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/brokenbots/criteriadb/pkg/criteria"
	"github.com/brokenbots/criteriadb/pkg/memory"
	pb "github.com/brokenbots/criteriadb/pkg/pb/criteriadb/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestCriteriaDB_ZeroCGO_FullSuite(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_criteria.db")

	// 1. Initialize Memory Engine
	engine, err := memory.NewMemoryEngine(memory.Config{
		StoragePath: dbPath,
	})
	if err != nil {
		t.Fatalf("Failed to initialize memory engine: %v", err)
	}
	defer engine.Close()

	now := time.Now()

	// 2. Insert Node A (Copilot adapter, past event, zero-LLM text)
	nodeA := &pb.MemoryNode{
		Id:      "node-a",
		Label:   "Refactor Auth Module",
		Type:    "task_completed",
		Summary: "Refactored JWT verification logic in auth package",
		Temporal: &pb.TemporalInfo{
			Timestamp: timestamppb.New(now.Add(-24 * time.Hour)),
			ValidFrom: timestamppb.New(now.Add(-48 * time.Hour)),
			ValidTo:   timestamppb.New(now.Add(24 * time.Hour)),
			Tense:     pb.Tense_TENSE_PAST,
		},
		DigitalLocation: &pb.DigitalLocation{
			Machine:    "dev-macbook",
			Repository: "github.com/brokenbots/criteria",
			Project:    "criteria-auth",
			FolderPath: "/pkg/auth",
		},
		AgentScope: &pb.AgentScope{
			CreatorAdapterId:   "copilot-dev-1",
			CreatorAdapterType: "copilot",
			CreatorAgentId:     "agent-alpha",
			Visibility:         pb.VisibilityScope_VISIBILITY_PRIVATE,
		},
	}

	idA, err := engine.Remember(ctx, nodeA, nil)
	if err != nil || idA != "node-a" {
		t.Fatalf("Failed to remember nodeA: %v", err)
	}

	// 3. Insert Node B (Shell adapter, planned task, vector embedding attached)
	vecB := []float32{0.1, 0.8, 0.3, 0.5}
	nodeB := &pb.MemoryNode{
		Id:              "node-b",
		Label:           "Deploy Staging Environment",
		Type:            "planned_task",
		Summary:         "Run docker compose up for staging test environment",
		PackedEmbedding: memory.PackFloats(vecB),
		VectorDimension: int32(len(vecB)),
		Temporal: &pb.TemporalInfo{
			Timestamp: timestamppb.New(now.Add(2 * time.Hour)),
			ValidFrom: timestamppb.New(now),
			ValidTo:   timestamppb.New(now.Add(24 * time.Hour)),
			Tense:     pb.Tense_TENSE_PLANNED,
		},
		DigitalLocation: &pb.DigitalLocation{
			Machine:    "staging-server-1",
			Repository: "github.com/brokenbots/criteria",
			Project:    "criteria-auth",
			FolderPath: "/deploy/staging",
		},
		AgentScope: &pb.AgentScope{
			CreatorAdapterId:   "shell-runner-1",
			CreatorAdapterType: "shell",
			Visibility:         pb.VisibilityScope_VISIBILITY_WORKFLOW,
		},
	}

	idB, err := engine.Remember(ctx, nodeB, nil)
	if err != nil || idB != "node-b" {
		t.Fatalf("Failed to remember nodeB: %v", err)
	}

	// 4. Test Zero-LLM Lexical Query
	t.Run("Zero-LLM Lexical Query", func(t *testing.T) {
		res, err := engine.Recall(ctx, &pb.QueryRequest{
			QueryText: "Refactor JWT auth",
			ScopeFilter: &pb.AdapterScopeFilter{
				CallerAdapterId: "copilot-dev-1",
			},
		})
		if err != nil {
			t.Fatalf("Recall failed: %v", err)
		}
		if len(res) == 0 {
			t.Fatalf("Expected results for Zero-LLM lexical search, got 0")
		}
		if res[0].Node.Id != "node-a" {
			t.Errorf("Expected node-a top result, got %s", res[0].Node.Id)
		}
	})

	// 5. Test Adapter Scope Privacy Isolation
	t.Run("Adapter Scope Isolation", func(t *testing.T) {
		// Shell runner trying to query copilot-dev-1's private memory
		res, err := engine.Recall(ctx, &pb.QueryRequest{
			QueryText: "Refactor JWT auth",
			ScopeFilter: &pb.AdapterScopeFilter{
				CallerAdapterId:  "shell-runner-1",
				TargetAdapterIds: []string{"self"},
			},
		})
		if err != nil {
			t.Fatalf("Recall failed: %v", err)
		}
		if len(res) > 0 {
			t.Errorf("Expected 0 results due to private scope isolation, got %d", len(res))
		}

		// Supervisory query ("*")
		resSuper, err := engine.Recall(ctx, &pb.QueryRequest{
			QueryText: "Refactor JWT auth",
			ScopeFilter: &pb.AdapterScopeFilter{
				CallerAdapterId:  "admin-agent",
				TargetAdapterIds: []string{"*"},
			},
		})
		if err != nil || len(resSuper) == 0 {
			t.Fatalf("Expected supervisory query to return results")
		}
	})

	// 6. Test Vector Cosine Distance Query
	t.Run("Vector Embedding Search", func(t *testing.T) {
		queryVec := []float32{0.1, 0.75, 0.35, 0.48}
		res, err := engine.Recall(ctx, &pb.QueryRequest{
			QueryText:            "Deploy staging environment",
			PackedQueryEmbedding: memory.PackFloats(queryVec),
			ScopeFilter: &pb.AdapterScopeFilter{
				TargetAdapterIds: []string{"*"},
			},
		})
		if err != nil || len(res) == 0 {
			t.Fatalf("Vector recall failed: %v", err)
		}
		if res[0].Node.Id != "node-b" {
			t.Errorf("Expected node-b top vector result, got %s", res[0].Node.Id)
		}
	})

	// 7. Test Location Scoping
	t.Run("Digital Location Scoping", func(t *testing.T) {
		res, err := engine.Recall(ctx, &pb.QueryRequest{
			LocationFilter: &pb.LocationQuery{
				MatchProject:      "criteria-auth",
				MatchFolderPrefix: "/deploy",
			},
			ScopeFilter: &pb.AdapterScopeFilter{
				TargetAdapterIds: []string{"*"},
			},
		})
		if err != nil || len(res) == 0 {
			t.Fatalf("Location query failed: %v", err)
		}
		if res[0].Node.Id != "node-b" {
			t.Errorf("Expected node-b location match, got %s", res[0].Node.Id)
		}
	})

	// 8. Test Criteria Event Stream Ingestion
	t.Run("Criteria Event Ingestion", func(t *testing.T) {
		ing := criteria.NewIngester(engine)
		evtJSON := []byte(`{
			"event_id": "evt-101",
			"workflow": "deploy-pipeline",
			"state": "build",
			"step": "compile-binary",
			"adapter_id": "shell-builder",
			"adapter": "shell",
			"outcome": "success",
			"workspace": "/Users/dave/Projects/criteria-suite/criteria",
			"repository": "github.com/brokenbots/criteria"
		}`)

		nodeID, err := ing.IngestNDJSONEvent(ctx, evtJSON)
		if err != nil || nodeID != "evt-101" {
			t.Fatalf("Failed to ingest ND-JSON event: %v", err)
		}
	})
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
