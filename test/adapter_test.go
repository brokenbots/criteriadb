package test

import (
	"context"
	"path/filepath"
	"testing"

	v2 "github.com/brokenbots/criteria-adapter-proto/criteria/v2"
	"github.com/brokenbots/criteriadb/pkg/adapter"
	"github.com/brokenbots/criteriadb/pkg/memory"
)

type mockSender struct {
	events []*v2.ExecuteEvent
}

func (m *mockSender) Send(ev *v2.ExecuteEvent) error {
	m.events = append(m.events, ev)
	return nil
}

func TestCriteriaDBAdapter(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "adapter_test.db")

	engine, err := memory.NewMemoryEngine(memory.Config{
		StoragePath: dbPath,
	})
	if err != nil {
		t.Fatalf("Failed to init memory engine: %v", err)
	}
	defer engine.Close()

	ad := adapter.NewCriteriaDBAdapter(engine)

	// Test Info
	info, err := ad.Info(ctx, &v2.InfoRequest{})
	if err != nil || info.Name != "criteriadb" {
		t.Fatalf("Unexpected Info response: %v", info)
	}

	// Test Remember Action
	senderRem := &mockSender{}
	err = ad.Execute(ctx, &v2.ExecuteRequest{
		Input: map[string]string{
			"action":     "remember",
			"label":      "Build Criteria Engine",
			"summary":    "Compiled FSM graph engine in Go",
			"type":       "task_done",
			"project":    "criteria-core",
			"adapter_id": "copilot-1",
		},
	}, senderRem)
	if err != nil {
		t.Fatalf("Execute remember failed: %v", err)
	}

	if len(senderRem.events) == 0 || senderRem.events[0].GetResult().GetOutcome() != "remembered" {
		t.Fatalf("Expected remembered outcome event")
	}

	// Test Recall Action
	senderRec := &mockSender{}
	err = ad.Execute(ctx, &v2.ExecuteRequest{
		Input: map[string]string{
			"action":            "recall",
			"query_text":        "Compiled FSM engine",
			"caller_adapter_id": "copilot-1",
		},
	}, senderRec)
	if err != nil {
		t.Fatalf("Execute recall failed: %v", err)
	}

	if len(senderRec.events) == 0 || senderRec.events[0].GetResult().GetOutcome() != "recalled" {
		t.Fatalf("Expected recalled outcome event")
	}
}
