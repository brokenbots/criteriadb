package memory

import (
	"math"
	"time"

	pb "github.com/brokenbots/criteriadb/pkg/pb/criteriadb/v1"
)

// NowTimestamp returns current time as time.Time.
func NowTimestamp() time.Time {
	return time.Now()
}

// EvaluateTemporal calculates the temporal relevance score [0.0, 1.0] for a MemoryNode.
func EvaluateTemporal(node *pb.MemoryNode, query *pb.TemporalQuery) float64 {
	if query == nil {
		return 1.0
	}

	temp := node.GetTemporal()
	hasFilter := query.GetTenseFilter() != pb.Tense_TENSE_UNSPECIFIED ||
		query.GetValidAt() != nil ||
		query.GetStartTime() != nil ||
		query.GetEndTime() != nil

	if hasFilter && temp == nil {
		return 0.0
	}
	if temp == nil {
		return 1.0
	}

	// 1. Tense Filter Matching
	if query.GetTenseFilter() != pb.Tense_TENSE_UNSPECIFIED {
		if temp.GetTense() != query.GetTenseFilter() {
			return 0.0
		}
	}

	// 2. Point-in-Time Relevancy (ValidAt)
	if query.GetValidAt() != nil {
		validAt := query.GetValidAt().AsTime()

		if temp.GetValidFrom() != nil {
			vf := temp.GetValidFrom().AsTime()
			if validAt.Before(vf) {
				return 0.0
			}
		}

		if temp.GetValidTo() != nil {
			vt := temp.GetValidTo().AsTime()
			if validAt.After(vt) {
				return 0.0
			}
		}

		// Decay based on timestamp distance if timestamp is set
		if temp.GetTimestamp() != nil {
			ts := temp.GetTimestamp().AsTime()
			diffHours := math.Abs(validAt.Sub(ts).Hours())
			// Decay over 30 days (720 hours)
			return math.Exp(-diffHours / 720.0)
		}
		return 1.0
	}

	// 3. Time Range Filtering
	if query.GetStartTime() != nil || query.GetEndTime() != nil {
		if temp.GetTimestamp() == nil {
			return 0.0
		}
		ts := temp.GetTimestamp().AsTime()

		if query.GetStartTime() != nil {
			st := query.GetStartTime().AsTime()
			if ts.Before(st) {
				return 0.0
			}
		}

		if query.GetEndTime() != nil {
			et := query.GetEndTime().AsTime()
			if ts.After(et) {
				return 0.0
			}
		}

		return 1.0
	}

	return 1.0
}
