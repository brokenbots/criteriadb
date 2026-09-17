package criteria

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/brokenbots/criteriadb/pkg/memory"
	pb "github.com/brokenbots/criteriadb/pkg/pb/criteriadb/v1"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// WorkflowEvent represents an ND-JSON event emitted by Criteria execution.
type WorkflowEvent struct {
	EventID    string         `json:"event_id"`
	Workflow   string         `json:"workflow"`
	State      string         `json:"state"`
	Step       string         `json:"step"`
	AdapterID  string         `json:"adapter_id"`
	Adapter    string         `json:"adapter"`
	Outcome    string         `json:"outcome"`
	Timestamp  time.Time      `json:"timestamp"`
	Workspace  string         `json:"workspace"`
	Repository string         `json:"repository"`
	Machine    string         `json:"machine"`
	Payload    map[string]any `json:"payload"`
}

// Ingester translates Criteria ND-JSON events into CriteriaDB MemoryNodes and MemoryEdges.
type Ingester struct {
	engine              memory.Engine
	mu                  sync.Mutex
	lastEventByWorkflow map[string]string
}

func NewIngester(engine memory.Engine) *Ingester {
	return &Ingester{
		engine:              engine,
		lastEventByWorkflow: make(map[string]string),
	}
}

// IngestNDJSONEvent parses an ND-JSON line and stores it in CriteriaDB.
func (ing *Ingester) IngestNDJSONEvent(ctx context.Context, line []byte) (string, error) {
	var evt WorkflowEvent
	if err := json.Unmarshal(line, &evt); err != nil {
		return "", fmt.Errorf("failed to unmarshal criteria event: %w", err)
	}

	if evt.EventID == "" {
		evt.EventID = fmt.Sprintf("evt-%d", time.Now().UnixNano())
	}

	ts := evt.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}

	var props *structpb.Struct
	if len(evt.Payload) > 0 {
		if sp, err := structpb.NewStruct(evt.Payload); err == nil {
			props = sp
		}
	}

	node := &pb.MemoryNode{
		Id:      evt.EventID,
		Label:   fmt.Sprintf("Step %s (%s)", evt.Step, evt.Outcome),
		Type:    "workflow_event",
		Summary: fmt.Sprintf("Workflow %s state %s executed step %s via adapter %s", evt.Workflow, evt.State, evt.Step, evt.Adapter),
		Temporal: &pb.TemporalInfo{
			Timestamp: timestamppb.New(ts),
			ValidFrom: timestamppb.New(ts),
			Tense:     pb.Tense_TENSE_PAST,
		},
		DigitalLocation: &pb.DigitalLocation{
			Machine:    evt.Machine,
			Repository: evt.Repository,
			Project:    evt.Workflow,
			FolderPath: evt.Workspace,
		},
		AgentScope: &pb.AgentScope{
			CreatorAdapterId:   evt.AdapterID,
			CreatorAdapterType: evt.Adapter,
			Visibility:         pb.VisibilityScope_VISIBILITY_WORKFLOW,
		},
		Properties: props,
	}

	var edges []*pb.MemoryEdge
	if evt.Workflow != "" {
		ing.mu.Lock()
		if prevID, ok := ing.lastEventByWorkflow[evt.Workflow]; ok && prevID != "" && prevID != evt.EventID {
			edgeID := fmt.Sprintf("edge-seq-%d", time.Now().UnixNano())
			edges = append(edges, &pb.MemoryEdge{
				Id:       edgeID,
				SourceId: prevID,
				TargetId: evt.EventID,
				Relation: "FOLLOWED_BY",
				Temporal: &pb.TemporalInfo{
					Timestamp: timestamppb.New(ts),
					Tense:     pb.Tense_TENSE_PAST,
				},
			})
		}
		ing.lastEventByWorkflow[evt.Workflow] = evt.EventID
		ing.mu.Unlock()
	}

	return ing.engine.Remember(ctx, node, edges)
}
