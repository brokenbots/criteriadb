package memory

import (
	"context"
	"fmt"
	"sync"

	pb "github.com/brokenbots/criteriadb/pkg/pb/criteriadb/v1"
)

// Engine defines the core Go interface for CriteriaDB.
type Engine interface {
	Remember(ctx context.Context, node *pb.MemoryNode, edges []*pb.MemoryEdge) (string, error)
	Recall(ctx context.Context, req *pb.QueryRequest) ([]*pb.MemoryQueryResult, error)
	Close() error
}

type Config struct {
	StoragePath           string // Path to bbolt database file
	EmbeddingEndpoint     string // e.g. "http://localhost:11434/v1/embeddings"
	EmbeddingModel        string // e.g. "nomic-embed-text"
	EnableZeroLLMFallback bool   // Fallback to lexical matching if no vector or embedder
}

type MemoryEngine struct {
	mu          sync.RWMutex
	nodes       map[string]*pb.MemoryNode
	edges       []*pb.MemoryEdge
	store       *BBoltStore
	embedder    *Embedder
	queryEngine *QueryEngine
	cfg         Config
}

func NewMemoryEngine(cfg Config) (*MemoryEngine, error) {
	store, err := NewBBoltStore(cfg.StoragePath)
	if err != nil {
		return nil, fmt.Errorf("failed to init bbolt store: %w", err)
	}

	nodesMap := make(map[string]*pb.MemoryNode)
	var edgesSlice []*pb.MemoryEdge

	if store != nil {
		loadedNodes, loadedEdges, err := store.LoadAll()
		if err != nil {
			return nil, fmt.Errorf("failed to load persistent memory: %w", err)
		}
		for _, n := range loadedNodes {
			nodesMap[n.GetId()] = n
		}
		edgesSlice = loadedEdges
	}

	var embedder *Embedder
	if cfg.EmbeddingEndpoint != "" {
		embedder = NewEmbedder(cfg.EmbeddingEndpoint, cfg.EmbeddingModel)
	}

	return &MemoryEngine{
		nodes:       nodesMap,
		edges:       edgesSlice,
		store:       store,
		embedder:    embedder,
		queryEngine: NewQueryEngine(),
		cfg:         cfg,
	}, nil
}

func (e *MemoryEngine) Remember(ctx context.Context, node *pb.MemoryNode, edges []*pb.MemoryEdge) (string, error) {
	if node == nil || node.GetId() == "" {
		return "", fmt.Errorf("node cannot be nil and must have an ID")
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

	e.mu.Lock()
	defer e.mu.Unlock()

	e.nodes[node.GetId()] = node
	if e.store != nil {
		if err := e.store.SaveNode(node); err != nil {
			return "", fmt.Errorf("failed to save node to storage: %w", err)
		}
	}

	for _, edge := range edges {
		if edge.GetId() != "" {
			e.edges = append(e.edges, edge)
			if e.store != nil {
				_ = e.store.SaveEdge(edge)
			}
		}
	}

	return node.GetId(), nil
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
