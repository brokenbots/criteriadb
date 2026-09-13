package memory

import (
	"context"
	"fmt"
	"time"

	pb "github.com/brokenbots/criteriadb/pkg/pb/criteriadb/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// PruneExpired removes all memory nodes whose valid_to timestamp is before now.
func (e *MemoryEngine) PruneExpired(ctx context.Context) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	prunedCount := 0
	deletedIDs := make(map[string]struct{})

	for id, n := range e.nodes {
		if n.GetTemporal() != nil && n.GetTemporal().GetValidTo() != nil {
			vt := n.GetTemporal().GetValidTo().AsTime()
			if vt.Before(now) {
				delete(e.nodes, id)
				if e.store != nil {
					_ = e.store.DeleteNode(id)
				}
				deletedIDs[id] = struct{}{}
				prunedCount++
			}
		}
	}

	e.removeConnectedEdgesLocked(deletedIDs)
	return prunedCount, nil
}

// Consolidate merges repetitive event nodes sharing the same project and type into a consolidated fact node.
func (e *MemoryEngine) Consolidate(ctx context.Context, project string, nodeType string) (*pb.MemoryNode, int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	var matchingIDs []string

	for id, n := range e.nodes {
		pMatch := project == "" || n.GetDigitalLocation().GetProject() == project
		tMatch := nodeType == "" || n.GetType() == nodeType

		if pMatch && tMatch && n.GetType() != "consolidated_fact" {
			matchingIDs = append(matchingIDs, id)
		}
	}

	if len(matchingIDs) < 2 {
		return nil, 0, nil
	}

	// Create single consolidated fact node
	consID := fmt.Sprintf("fact-cons-%d", time.Now().UnixNano())
	summary := fmt.Sprintf("Consolidated %d events (%s) for project %s", len(matchingIDs), nodeType, project)

	consNode := &pb.MemoryNode{
		Id:      consID,
		Label:   fmt.Sprintf("Consolidated Fact: %s (%d items)", nodeType, len(matchingIDs)),
		Type:    "consolidated_fact",
		Summary: summary,
		Temporal: &pb.TemporalInfo{
			Timestamp: timestamppb.New(time.Now()),
			ValidFrom: timestamppb.New(time.Now()),
			Tense:     pb.Tense_TENSE_PAST,
		},
		DigitalLocation: &pb.DigitalLocation{
			Project: project,
		},
		AgentScope: &pb.AgentScope{
			Visibility: pb.VisibilityScope_VISIBILITY_GLOBAL,
		},
	}

	// Add consolidated node and remove raw events
	e.nodes[consID] = consNode
	if e.store != nil {
		_ = e.store.SaveNode(consNode)
	}

	deletedIDs := make(map[string]struct{})
	for _, id := range matchingIDs {
		delete(e.nodes, id)
		if e.store != nil {
			_ = e.store.DeleteNode(id)
		}
		deletedIDs[id] = struct{}{}
	}
	e.removeConnectedEdgesLocked(deletedIDs)

	return consNode, len(matchingIDs), nil
}

// removeConnectedEdgesLocked deletes any edges touching the deleted nodes. Must be called with e.mu held.
func (e *MemoryEngine) removeConnectedEdgesLocked(deletedIDs map[string]struct{}) {
	if len(deletedIDs) == 0 {
		return
	}
	for id, edge := range e.edges {
		_, srcDel := deletedIDs[edge.GetSourceId()]
		_, tgtDel := deletedIDs[edge.GetTargetId()]
		if srcDel || tgtDel {
			delete(e.edges, id)
			if e.store != nil {
				_ = e.store.DeleteEdge(edge.GetId())
			}
		}
	}
}

// GetStats returns summary statistics for the memory engine.
type EngineStats struct {
	TotalNodes   int            `json:"total_nodes"`
	TotalEdges   int            `json:"total_edges"`
	NodesByType  map[string]int `json:"nodes_by_type"`
	NodesByTense map[string]int `json:"nodes_by_tense"`
}

func (e *MemoryEngine) GetStats() EngineStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	byType := make(map[string]int)
	byTense := make(map[string]int)

	for _, n := range e.nodes {
		t := n.GetType()
		if t == "" {
			t = "unspecified"
		}
		byType[t]++

		tense := "unspecified"
		if n.GetTemporal() != nil {
			tense = n.GetTemporal().GetTense().String()
		}
		byTense[tense]++
	}

	return EngineStats{
		TotalNodes:   len(e.nodes),
		TotalEdges:   len(e.edges),
		NodesByType:  byType,
		NodesByTense: byTense,
	}
}
