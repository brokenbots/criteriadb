package memory

import (
	pb "github.com/brokenbots/criteriadb/pkg/pb/criteriadb/v1"
)

// EvaluateScope checks if targetNode and targetEdge match the requested AdapterScopeFilter.
func MatchesScope(node *pb.MemoryNode, filter *pb.AdapterScopeFilter) bool {
	if filter == nil {
		return true
	}

	scope := node.GetAgentScope()
	if scope == nil {
		// Unscoped nodes are readable unless restricted
		return true
	}

	// 1. Global / Supervisory Scope ("*")
	for _, target := range filter.GetTargetAdapterIds() {
		if target == "*" {
			return true
		}
	}

	// 2. Workflow-shared visibility
	if filter.GetIncludeWorkflowShared() && scope.GetVisibility() == pb.VisibilityScope_VISIBILITY_WORKFLOW {
		return true
	}

	// 3. Global visibility
	if scope.GetVisibility() == pb.VisibilityScope_VISIBILITY_GLOBAL {
		return true
	}

	// 4. Match explicit target adapter IDs or caller identity
	callerID := filter.GetCallerAdapterId()
	creatorID := scope.GetCreatorAdapterId()

	if len(filter.GetTargetAdapterIds()) == 0 {
		// Default: Private to caller or matching creator
		if callerID != "" && callerID == creatorID {
			return true
		}
		// If caller ID not specified, allow if visibility is not strictly private
		return scope.GetVisibility() != pb.VisibilityScope_VISIBILITY_PRIVATE
	}

	for _, target := range filter.GetTargetAdapterIds() {
		if target == "self" && callerID != "" && creatorID == callerID {
			return true
		}
		if target == creatorID {
			return true
		}
	}

	return false
}
