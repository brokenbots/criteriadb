package memory

import (
	"sort"
	"strings"

	pb "github.com/brokenbots/criteriadb/pkg/pb/criteriadb/v1"
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
	edges []*pb.MemoryEdge,
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

	// Build adjacency mapping for edges
	nodeEdges := make(map[string][]*pb.MemoryEdge)
	for _, e := range edges {
		nodeEdges[e.GetSourceId()] = append(nodeEdges[e.GetSourceId()], e)
		nodeEdges[e.GetTargetId()] = append(nodeEdges[e.GetTargetId()], e)
	}

	for _, node := range nodes {
		// 1. Adapter & Agent Scope Filter
		if !MatchesScope(node, req.GetScopeFilter()) {
			continue
		}

		// 2. Lexical Score (Zero-LLM mode)
		lexicalScore := qe.lexical.EvaluateLexical(node, req.GetQueryText())

		// 3. Vector Score (if present)
		vectorScore := 0.0
		if !req.GetDisableVectorSearch() && len(queryVec) > 0 && len(node.GetPackedEmbedding()) > 0 {
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
		if len(queryVec) > 0 && !req.GetDisableVectorSearch() && len(node.GetPackedEmbedding()) > 0 {
			textScore = 0.7*vectorScore + 0.3*lexicalScore
		} else {
			textScore = lexicalScore
		}

		// Combined composite score
		finalScore := 0.5*textScore + 0.3*temporalScore + 0.2*locationScore

		// Filter out results with 0 text match if a query text/vec was provided
		if (req.GetQueryText() != "" || len(queryVec) > 0) && textScore <= 0.0 {
			continue
		}

		// Retrieve connected edges
		conn := nodeEdges[node.GetId()]

		// Factual graph label filtering if specified
		if req.GetFactFilter() != nil && req.GetFactFilter().GetTargetLabel() != "" {
			matchFact := false
			targetLbl := strings.ToLower(req.GetFactFilter().GetTargetLabel())
			for _, e := range conn {
				otherID := e.GetSourceId()
				if otherID == node.GetId() {
					otherID = e.GetTargetId()
				}
				if otherNode, ok := nodes[otherID]; ok {
					if strings.Contains(strings.ToLower(otherNode.GetLabel()), targetLbl) {
						matchFact = true
						break
					}
				}
			}
			if !matchFact {
				continue
			}
		}

		results = append(results, &pb.MemoryQueryResult{
			Node:           node,
			ConnectedEdges: conn,
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
