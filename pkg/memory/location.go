package memory

import (
	"strings"

	pb "github.com/brokenbots/criteriadb/pkg/pb/criteriadb/v1"
)

// EvaluateLocation calculates location match score [0.0, 1.0] for a MemoryNode.
func EvaluateLocation(node *pb.MemoryNode, query *pb.LocationQuery) float64 {
	if query == nil {
		return 1.0
	}

	dig := node.GetDigitalLocation()
	phys := node.GetPhysicalLocation()

	matchesCount := 0
	totalFilters := 0

	if query.GetMatchProject() != "" {
		totalFilters++
		if dig != nil && strings.EqualFold(dig.GetProject(), query.GetMatchProject()) {
			matchesCount++
		}
	}

	if query.GetMatchFolderPrefix() != "" {
		totalFilters++
		if dig != nil && strings.HasPrefix(strings.ToLower(dig.GetFolderPath()), strings.ToLower(query.GetMatchFolderPrefix())) {
			matchesCount++
		}
	}

	if query.GetMatchMachine() != "" {
		totalFilters++
		if dig != nil && strings.EqualFold(dig.GetMachine(), query.GetMatchMachine()) {
			matchesCount++
		}
	}

	if query.GetMatchVenue() != "" {
		totalFilters++
		if phys != nil && strings.Contains(strings.ToLower(phys.GetVenue()), strings.ToLower(query.GetMatchVenue())) {
			matchesCount++
		}
	}

	if totalFilters == 0 {
		return 1.0
	}

	if matchesCount < totalFilters {
		return 0.0
	}

	return 1.0
}
