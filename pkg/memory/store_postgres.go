package memory

import (
	"context"
	"fmt"
	"log"
	"time"

	pb "github.com/brokenbots/criteriadb/pkg/pb/criteriadb/v1"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"
)

// PostgresStore handles persistent distributed storage of CriteriaDB nodes and edges in PostgreSQL / CockroachDB.
type PostgresStore struct {
	pool *pgxpool.Pool
}

var _ Store = (*PostgresStore)(nil)

// NewPostgresStore initializes a PostgreSQL/CockroachDB connection pool and ensures schema existence.
func NewPostgresStore(connString string) (*PostgresStore, error) {
	if connString == "" {
		return &PostgresStore{pool: nil}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse postgres connection string: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping postgres database: %w", err)
	}

	store := &PostgresStore{pool: pool}
	if err := store.initSchema(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to initialize postgres schema: %w", err)
	}

	return store, nil
}

func (s *PostgresStore) initSchema(ctx context.Context) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS criteria_nodes (
			id VARCHAR(255) PRIMARY KEY,
			label TEXT NOT NULL DEFAULT '',
			type VARCHAR(100) NOT NULL DEFAULT '',
			project VARCHAR(255) NOT NULL DEFAULT '',
			data BYTEA NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
		`CREATE TABLE IF NOT EXISTS criteria_edges (
			id VARCHAR(255) PRIMARY KEY,
			source_id VARCHAR(255) NOT NULL DEFAULT '',
			target_id VARCHAR(255) NOT NULL DEFAULT '',
			relation VARCHAR(100) NOT NULL DEFAULT '',
			data BYTEA NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
		`CREATE INDEX IF NOT EXISTS idx_criteria_edges_source ON criteria_edges(source_id);`,
		`CREATE INDEX IF NOT EXISTS idx_criteria_edges_target ON criteria_edges(target_id);`,
		`CREATE INDEX IF NOT EXISTS idx_criteria_nodes_project ON criteria_nodes(project);`,
	}

	for _, q := range queries {
		if _, err := s.pool.Exec(ctx, q); err != nil {
			return fmt.Errorf("error executing schema query (%s): %w", q, err)
		}
	}
	return nil
}

func (s *PostgresStore) SaveNode(node *pb.MemoryNode) error {
	if s.pool == nil || node == nil || node.GetId() == "" {
		return nil
	}

	data, err := proto.Marshal(node)
	if err != nil {
		return fmt.Errorf("failed to marshal node proto: %w", err)
	}

	project := ""
	if node.GetDigitalLocation() != nil {
		project = node.GetDigitalLocation().GetProject()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		INSERT INTO criteria_nodes (id, label, type, project, data, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (id) DO UPDATE SET
			label = EXCLUDED.label,
			type = EXCLUDED.type,
			project = EXCLUDED.project,
			data = EXCLUDED.data,
			updated_at = NOW();`

	_, err = s.pool.Exec(ctx, query, node.GetId(), node.GetLabel(), node.GetType(), project, data)
	return err
}

func (s *PostgresStore) DeleteNode(id string) error {
	if s.pool == nil || id == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := s.pool.Exec(ctx, "DELETE FROM criteria_nodes WHERE id = $1;", id)
	return err
}

func (s *PostgresStore) SaveEdge(edge *pb.MemoryEdge) error {
	if s.pool == nil || edge == nil || edge.GetId() == "" {
		return nil
	}

	data, err := proto.Marshal(edge)
	if err != nil {
		return fmt.Errorf("failed to marshal edge proto: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		INSERT INTO criteria_edges (id, source_id, target_id, relation, data, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (id) DO UPDATE SET
			source_id = EXCLUDED.source_id,
			target_id = EXCLUDED.target_id,
			relation = EXCLUDED.relation,
			data = EXCLUDED.data,
			updated_at = NOW();`

	_, err = s.pool.Exec(ctx, query, edge.GetId(), edge.GetSourceId(), edge.GetTargetId(), edge.GetRelation(), data)
	return err
}

func (s *PostgresStore) DeleteEdge(id string) error {
	if s.pool == nil || id == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := s.pool.Exec(ctx, "DELETE FROM criteria_edges WHERE id = $1;", id)
	return err
}

func (s *PostgresStore) LoadAll() ([]*pb.MemoryNode, []*pb.MemoryEdge, error) {
	if s.pool == nil {
		return nil, nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var nodes []*pb.MemoryNode
	var edges []*pb.MemoryEdge

	nodeRows, err := s.pool.Query(ctx, "SELECT id, data FROM criteria_nodes;")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query criteria_nodes: %w", err)
	}
	defer nodeRows.Close()

	for nodeRows.Next() {
		var id string
		var data []byte
		if err := nodeRows.Scan(&id, &data); err != nil {
			log.Printf("[PostgresStore] Warning: error scanning node row: %v", err)
			continue
		}
		var node pb.MemoryNode
		if err := proto.Unmarshal(data, &node); err == nil {
			nodes = append(nodes, &node)
		} else {
			log.Printf("[PostgresStore] Warning: failed to unmarshal node %s: %v", id, err)
		}
	}

	edgeRows, err := s.pool.Query(ctx, "SELECT id, data FROM criteria_edges;")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query criteria_edges: %w", err)
	}
	defer edgeRows.Close()

	for edgeRows.Next() {
		var id string
		var data []byte
		if err := edgeRows.Scan(&id, &data); err != nil {
			log.Printf("[PostgresStore] Warning: error scanning edge row: %v", err)
			continue
		}
		var edge pb.MemoryEdge
		if err := proto.Unmarshal(data, &edge); err == nil {
			edges = append(edges, &edge)
		} else {
			log.Printf("[PostgresStore] Warning: failed to unmarshal edge %s: %v", id, err)
		}
	}

	return nodes, edges, nil
}

func (s *PostgresStore) Close() error {
	if s.pool != nil {
		s.pool.Close()
	}
	return nil
}
