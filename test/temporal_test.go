package test

import (
	"testing"
	"time"

	"github.com/brokenbots/criteriadb/pkg/memory"
	pb "github.com/brokenbots/criteriadb/pkg/pb/criteriadb/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestEvaluateTemporal_NilQueryAndNilTemporal(t *testing.T) {
	node := &pb.MemoryNode{Id: "test-node"}
	// Nil query should return 1.0
	if score := memory.EvaluateTemporal(node, nil); score != 1.0 {
		t.Fatalf("Expected 1.0 for nil query, got %f", score)
	}

	// Nil temporal info with a query should return 0.0 because filter cannot be satisfied
	query := &pb.TemporalQuery{TenseFilter: pb.Tense_TENSE_PAST}
	if score := memory.EvaluateTemporal(node, query); score != 0.0 {
		t.Fatalf("Expected 0.0 for nil temporal node, got %f", score)
	}
}

func TestEvaluateTemporal_TenseFilter(t *testing.T) {
	nodePast := &pb.MemoryNode{
		Id: "node-past",
		Temporal: &pb.TemporalInfo{
			Tense: pb.Tense_TENSE_PAST,
		},
	}

	queryPast := &pb.TemporalQuery{TenseFilter: pb.Tense_TENSE_PAST}
	if score := memory.EvaluateTemporal(nodePast, queryPast); score != 1.0 {
		t.Fatalf("Expected 1.0 for matching tense, got %f", score)
	}

	queryFuture := &pb.TemporalQuery{TenseFilter: pb.Tense_TENSE_FUTURE}
	if score := memory.EvaluateTemporal(nodePast, queryFuture); score != 0.0 {
		t.Fatalf("Expected 0.0 for mismatching tense, got %f", score)
	}
}

func TestEvaluateTemporal_ValidAt_BoundsAndDecay(t *testing.T) {
	now := time.Now()
	vf := now.Add(-10 * time.Hour)
	vt := now.Add(10 * time.Hour)
	ts := now.Add(-5 * time.Hour)

	node := &pb.MemoryNode{
		Id: "node-bounded",
		Temporal: &pb.TemporalInfo{
			Timestamp: timestamppb.New(ts),
			ValidFrom: timestamppb.New(vf),
			ValidTo:   timestamppb.New(vt),
			Tense:     pb.Tense_TENSE_PRESENT,
		},
	}

	// 1. ValidAt within window
	qWithin := &pb.TemporalQuery{
		ValidAt: timestamppb.New(now),
	}
	scoreWithin := memory.EvaluateTemporal(node, qWithin)
	if scoreWithin <= 0.0 || scoreWithin > 1.0 {
		t.Fatalf("Expected score within (0, 1], got %f", scoreWithin)
	}

	// 2. ValidAt before valid_from
	qBefore := &pb.TemporalQuery{
		ValidAt: timestamppb.New(now.Add(-20 * time.Hour)),
	}
	if score := memory.EvaluateTemporal(node, qBefore); score != 0.0 {
		t.Fatalf("Expected 0.0 for valid_at before valid_from, got %f", score)
	}

	// 3. ValidAt after valid_to
	qAfter := &pb.TemporalQuery{
		ValidAt: timestamppb.New(now.Add(20 * time.Hour)),
	}
	if score := memory.EvaluateTemporal(node, qAfter); score != 0.0 {
		t.Fatalf("Expected 0.0 for valid_at after valid_to, got %f", score)
	}

	// 4. Test decay calculation without timestamp
	nodeNoTs := &pb.MemoryNode{
		Id: "node-no-ts",
		Temporal: &pb.TemporalInfo{
			ValidFrom: timestamppb.New(vf),
			ValidTo:   timestamppb.New(vt),
		},
	}
	if score := memory.EvaluateTemporal(nodeNoTs, qWithin); score != 1.0 {
		t.Fatalf("Expected 1.0 when valid_at matches and timestamp is nil, got %f", score)
	}
}

func TestEvaluateTemporal_TimeRangeFiltering(t *testing.T) {
	now := time.Now()
	node := &pb.MemoryNode{
		Id: "node-ts",
		Temporal: &pb.TemporalInfo{
			Timestamp: timestamppb.New(now),
		},
	}

	// Query start before, end after -> match
	qMatch := &pb.TemporalQuery{
		StartTime: timestamppb.New(now.Add(-1 * time.Hour)),
		EndTime:   timestamppb.New(now.Add(1 * time.Hour)),
	}
	if score := memory.EvaluateTemporal(node, qMatch); score != 1.0 {
		t.Fatalf("Expected 1.0 for timestamp in range, got %f", score)
	}

	// Query start after node timestamp -> fail
	qAfter := &pb.TemporalQuery{
		StartTime: timestamppb.New(now.Add(1 * time.Hour)),
	}
	if score := memory.EvaluateTemporal(node, qAfter); score != 0.0 {
		t.Fatalf("Expected 0.0 for node before start time, got %f", score)
	}

	// Query end before node timestamp -> fail
	qBefore := &pb.TemporalQuery{
		EndTime: timestamppb.New(now.Add(-1 * time.Hour)),
	}
	if score := memory.EvaluateTemporal(node, qBefore); score != 0.0 {
		t.Fatalf("Expected 0.0 for node after end time, got %f", score)
	}

	// Node with nil timestamp matching time range returns 0.0 because range cannot be evaluated
	nodeNilTs := &pb.MemoryNode{
		Id:       "node-nil-ts",
		Temporal: &pb.TemporalInfo{},
	}
	if score := memory.EvaluateTemporal(nodeNilTs, qMatch); score != 0.0 {
		t.Fatalf("Expected 0.0 for nil timestamp on range query, got %f", score)
	}
}
