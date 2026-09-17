package memory

import (
	"context"
	"fmt"
	"strings"
	"sync"

	pb "github.com/brokenbots/criteriadb/pkg/pb/criteriadb/v1"
	"google.golang.org/protobuf/proto"
)

// Engine defines the core Go interface for CriteriaDB.
type Engine interface {
	Remember(ctx context.Context, node *pb.MemoryNode, edges []*pb.MemoryEdge) (string, error)
	RememberEdge(ctx context.Context, edge *pb.MemoryEdge) error
	Recall(ctx context.Context, req *pb.QueryRequest) ([]*pb.MemoryQueryResult, error)
	Close() error
}

type Config struct {
	StorageBackend        string // "bbolt" (default) or "postgres"
	StoragePath           string // Path to bbolt database file
	PostgresConnString    string // Connection string for PostgreSQL / CockroachDB
	EmbeddingEndpoint     string // e.g. "http://localhost:11434/v1/embeddings"
	EmbeddingModel        string // e.g. "nomic-embed-text"
	EnableZeroLLMFallback bool   // Fallback to lexical matching if no vector or embedder
}

type MemoryEngine struct {
	mu          sync.RWMutex
	nodes       map[string]*pb.MemoryNode
	edges       map[string]*pb.MemoryEdge
	store       Store
	embedder    *Embedder
	queryEngine *QueryEngine
	cfg         Config
}

func NewMemoryEngine(cfg Config) (*MemoryEngine, error) {
	var store Store
	var err error

	if cfg.PostgresConnString != "" || cfg.StorageBackend == "postgres" {
		store, err = NewPostgresStore(cfg.PostgresConnString)
		if err != nil {
			return nil, fmt.Errorf("failed to init postgres store: %w", err)
		}
	} else {
		store, err = NewBBoltStore(cfg.StoragePath)
		if err != nil {
			return nil, fmt.Errorf("failed to init bbolt store: %w", err)
		}
	}

	nodesMap := make(map[string]*pb.MemoryNode)
	edgesMap := make(map[string]*pb.MemoryEdge)

	if store != nil {
		loadedNodes, loadedEdges, err := store.LoadAll()
		if err != nil {
			return nil, fmt.Errorf("failed to load persistent memory: %w", err)
		}
		for _, n := range loadedNodes {
			nodesMap[n.GetId()] = n
		}
		for _, e := range loadedEdges {
			edgesMap[e.GetId()] = e
		}
	}

	var embedder *Embedder
	if cfg.EmbeddingEndpoint != "" {
		embedder = NewEmbedder(cfg.EmbeddingEndpoint, cfg.EmbeddingModel)
	}

	return &MemoryEngine{
		nodes:       nodesMap,
		edges:       edgesMap,
		store:       store,
		embedder:    embedder,
		queryEngine: NewQueryEngine(),
		cfg:         cfg,
	}, nil
}

func (e *MemoryEngine) Remember(ctx context.Context, node *pb.MemoryNode, edges []*pb.MemoryEdge) (string, error) {
	if node == nil || strings.TrimSpace(node.GetId()) == "" {
		return "", fmt.Errorf("node cannot be nil and must have a valid ID")
	}

	// Auto-generate vector embedding via local HTTP endpoint if text is present but vector is empty
	if len(node.GetPackedEmbedding()) == 0 && e.embedder != nil {
		textToEmbed := node.GetLabel() + " " + node.GetSummary()
		vec, err := e.embedder.GenerateEmbedding(ctx, textToEmbed)
		if err == nil && len(vec) > 0 {
			node.PackedEmbedding = PackFloats(vec)
			node.VectorDimension = int32(len(vec))
		}
	}

	clonedNode := proto.Clone(node).(*pb.MemoryNode)

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.store != nil {
		if err := e.store.SaveNode(clonedNode); err != nil {
			return "", fmt.Errorf("failed to save node to storage: %w", err)
		}
	}
	e.nodes[clonedNode.GetId()] = clonedNode

	for _, edge := range edges {
		if edge == nil || strings.TrimSpace(edge.GetId()) == "" {
			continue
		}
		if strings.TrimSpace(edge.GetSourceId()) == "" || strings.TrimSpace(edge.GetTargetId()) == "" {
			return "", fmt.Errorf("edge %s must specify both source_id and target_id", edge.GetId())
		}
		clonedEdge := proto.Clone(edge).(*pb.MemoryEdge)
		if e.store != nil {
			if err := e.store.SaveEdge(clonedEdge); err != nil {
				return "", fmt.Errorf("failed to save edge %s to storage: %w", edge.GetId(), err)
			}
		}
		e.edges[clonedEdge.GetId()] = clonedEdge
	}

	return clonedNode.GetId(), nil
}

func (e *MemoryEngine) RememberEdge(ctx context.Context, edge *pb.MemoryEdge) error {
	if edge == nil || strings.TrimSpace(edge.GetId()) == "" {
		return fmt.Errorf("edge cannot be nil and must have a valid ID")
	}
	if strings.TrimSpace(edge.GetSourceId()) == "" || strings.TrimSpace(edge.GetTargetId()) == "" {
		return fmt.Errorf("edge must specify both source_id and target_id")
	}

	clonedEdge := proto.Clone(edge).(*pb.MemoryEdge)

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.store != nil {
		if err := e.store.SaveEdge(clonedEdge); err != nil {
			return fmt.Errorf("failed to save edge to storage: %w", err)
		}
	}
	e.edges[clonedEdge.GetId()] = clonedEdge
	return nil
}

func (e *MemoryEngine) Recall(ctx context.Context, req *pb.QueryRequest) ([]*pb.MemoryQueryResult, error) {
	if req == nil {
		req = &pb.QueryRequest{}
	}

	var queryVec []float32

	// Extract or generate vector embedding for query text
	if len(req.GetPackedQueryEmbedding()) > 0 {
		queryVec = UnpackFloats(req.GetPackedQueryEmbedding())
	} else if req.GetQueryText() != "" && e.embedder != nil && !req.GetDisableVectorSearch() {
		vec, err := e.embedder.GenerateEmbedding(ctx, req.GetQueryText())
		if err == nil && len(vec) > 0 {
			queryVec = vec
		}
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	results := e.queryEngine.ExecuteQuery(e.nodes, e.edges, req, queryVec)
	return results, nil
}

func (e *MemoryEngine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.store != nil {
		return e.store.Close()
	}
	return nil
}
