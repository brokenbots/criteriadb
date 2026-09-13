package memory

import (
	"sort"
	"strings"

	pb "github.com/brokenbots/criteriadb/pkg/pb/criteriadb/v1"
	"google.golang.org/protobuf/proto"
)

type QueryEngine struct {
	lexical *LexicalIndex
}

func NewQueryEngine() *QueryEngine {
	return &QueryEngine{
		lexical: NewLexicalIndex(),
	}
}

// ExecuteQuery runs multi-dimensional scoring across nodes and returns ranked MemoryQueryResult slice.
func (qe *QueryEngine) ExecuteQuery(
	nodes map[string]*pb.MemoryNode,
	edges map[string]*pb.MemoryEdge,
	req *pb.QueryRequest,
	queryVec []float32,
) []*pb.MemoryQueryResult {
	if req == nil {
		req = &pb.QueryRequest{}
	}

	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = 20
	}

	var results []*pb.MemoryQueryResult

	// Build adjacency mapping for edges without duplicate self-loops
	nodeEdges := make(map[string][]*pb.MemoryEdge)
	for _, e := range edges {
		nodeEdges[e.GetSourceId()] = append(nodeEdges[e.GetSourceId()], e)
		if e.GetSourceId() != e.GetTargetId() {
			nodeEdges[e.GetTargetId()] = append(nodeEdges[e.GetTargetId()], e)
		}
	}

	hasTextQuery := strings.TrimSpace(req.GetQueryText()) != ""
	hasVectorQuery := len(queryVec) > 0 && !req.GetDisableVectorSearch()

	for _, node := range nodes {
		// 1. Adapter & Agent Scope Filter
		if !MatchesScope(node, req.GetScopeFilter()) {
			continue
		}

		// 2. Lexical Score (Zero-LLM mode)
		lexicalScore := qe.lexical.EvaluateLexical(node, req.GetQueryText())

		// 3. Vector Score (if present)
		vectorScore := 0.0
		nodeHasVec := len(node.GetPackedEmbedding()) > 0
		if hasVectorQuery && nodeHasVec {
			nodeVec := UnpackFloats(node.GetPackedEmbedding())
			vectorScore = CosineSimilarity(queryVec, nodeVec)
		}

		// 4. Temporal Score
		temporalScore := EvaluateTemporal(node, req.GetTemporalFilter())
		if temporalScore == 0.0 {
			// Hard temporal filter match failed
			continue
		}

		// 5. Location Score
		locationScore := EvaluateLocation(node, req.GetLocationFilter())
		if locationScore == 0.0 {
			// Hard location filter match failed
			continue
		}

		// 6. Calculate Hybrid Final Score
		var textScore float64
		if hasVectorQuery && nodeHasVec && hasTextQuery {
			textScore = 0.7*vectorScore + 0.3*lexicalScore
		} else if hasVectorQuery && nodeHasVec {
			textScore = vectorScore
		} else {
			textScore = lexicalScore
		}

		// Combined composite score
		var finalScore float64
		if hasTextQuery || (hasVectorQuery && nodeHasVec) {
			finalScore = 0.5*textScore + 0.3*temporalScore + 0.2*locationScore
		} else {
			finalScore = 0.6*temporalScore + 0.4*locationScore
		}

		// Filter out results with 0 text match if a query text/vec was provided
		if (hasTextQuery || (hasVectorQuery && nodeHasVec)) && textScore <= 0.0 {
			continue
		}

		// Retrieve connected edges
		conn := nodeEdges[node.GetId()]

		// Factual graph traversal and relation filtering if specified
		if req.GetFactFilter() != nil && !matchesFactFilter(node, req.GetFactFilter(), nodes, nodeEdges) {
			continue
		}

		clonedEdges := make([]*pb.MemoryEdge, len(conn))
		for i, e := range conn {
			clonedEdges[i] = proto.Clone(e).(*pb.MemoryEdge)
		}

		results = append(results, &pb.MemoryQueryResult{
			Node:           proto.Clone(node).(*pb.MemoryNode),
			ConnectedEdges: clonedEdges,
			Score:          finalScore,
			LexicalScore:   lexicalScore,
			VectorScore:    vectorScore,
			TemporalScore:  temporalScore,
			LocationScore:  locationScore,
		})
	}

	// Sort descending by Final Score
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results
}

// matchesFactFilter executes a multi-hop BFS graph traversal up to max_hops, filtering by relation_types and target_label.
func matchesFactFilter(
	startNode *pb.MemoryNode,
	filter *pb.FactQuery,
	nodes map[string]*pb.MemoryNode,
	nodeEdges map[string][]*pb.MemoryEdge,
) bool {
	if filter == nil {
		return true
	}

	targetLbl := strings.ToLower(strings.TrimSpace(filter.GetTargetLabel()))
	relTypes := filter.GetRelationTypes()
	if targetLbl == "" && len(relTypes) == 0 {
		return true
	}

	allowedRels := make(map[string]bool)
	for _, rt := range relTypes {
		trimmed := strings.ToUpper(strings.TrimSpace(rt))
		if trimmed != "" {
			allowedRels[trimmed] = true
		}
	}

	maxHops := int(filter.GetMaxHops())
	if maxHops <= 0 {
		maxHops = 1
	}
	if maxHops > 10 {
		maxHops = 10
	}

	type hopItem struct {
		id  string
		hop int
	}

	queue := []hopItem{{id: startNode.GetId(), hop: 0}}
	visited := map[string]bool{startNode.GetId(): true}

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		if curr.hop > 0 {
			if targetLbl != "" {
				if otherNode, ok := nodes[curr.id]; ok {
					if strings.Contains(strings.ToLower(otherNode.GetLabel()), targetLbl) {
						return true
					}
				}
			} else {
				// Only relation type filter specified, reached at least 1 valid connected hop
				return true
			}
		}

		if curr.hop >= maxHops {
			continue
		}

		for _, edge := range nodeEdges[curr.id] {
			if len(allowedRels) > 0 && !allowedRels[strings.ToUpper(strings.TrimSpace(edge.GetRelation()))] {
				continue
			}

			nextID := edge.GetSourceId()
			if nextID == curr.id {
				nextID = edge.GetTargetId()
			}

			if !visited[nextID] {
				visited[nextID] = true
				queue = append(queue, hopItem{id: nextID, hop: curr.hop + 1})
			}
		}
	}

	return false
}
