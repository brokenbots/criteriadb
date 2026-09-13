package test

import (
	"context"
	"os"
	"testing"

	"github.com/brokenbots/criteriadb/pkg/memory"
	pb "github.com/brokenbots/criteriadb/pkg/pb/criteriadb/v1"
)

func TestPostgresStore_NilConnString(t *testing.T) {
	store, err := memory.NewPostgresStore("")
	if err != nil {
		t.Fatalf("Expected nil error for empty postgres conn string, got: %v", err)
	}
	defer store.Close()

	node := &pb.MemoryNode{Id: "test-1", Label: "Test Node"}
	if err := store.SaveNode(node); err != nil {
		t.Fatalf("SaveNode on nil pool returned error: %v", err)
	}

	nodes, edges, err := store.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll on nil pool returned error: %v", err)
	}
	if len(nodes) != 0 || len(edges) != 0 {
		t.Fatalf("Expected 0 nodes and 0 edges on nil pool, got %d nodes, %d edges", len(nodes), len(edges))
	}
}

func TestPostgresStore_IntegrationIfConfigured(t *testing.T) {
	connStr := os.Getenv("CRITERIADB_POSTGRES_TEST_URL")
	if connStr == "" {
		t.Skip("Skipping live Postgres integration test (CRITERIADB_POSTGRES_TEST_URL not set)")
	}

	ctx := context.Background()
	engine, err := memory.NewMemoryEngine(memory.Config{
		StorageBackend:     "postgres",
		PostgresConnString: connStr,
	})
	if err != nil {
		t.Fatalf("Failed to initialize MemoryEngine with PostgresStore: %v", err)
	}
	defer engine.Close()

	node := &pb.MemoryNode{
		Id:      "pg-test-node-1",
		Label:   "Postgres Test Label",
		Summary: "Testing Postgres persistence in CriteriaDB",
		Type:    "fact",
		DigitalLocation: &pb.DigitalLocation{
			Project: "castle-ha",
		},
	}

	id, err := engine.Remember(ctx, node, nil)
	if err != nil {
		t.Fatalf("Remember failed on PostgresStore: %v", err)
	}
	if id != "pg-test-node-1" {
		t.Fatalf("Expected node ID pg-test-node-1, got %s", id)
	}

	// Verify recall
	results, err := engine.Recall(ctx, &pb.QueryRequest{
		QueryText: "Postgres",
	})
	if err != nil {
		t.Fatalf("Recall failed on PostgresStore engine: %v", err)
	}
	if len(results) == 0 {
		t.Fatalf("Expected at least 1 query result from PostgresStore recall")
	}
}
