package adapter

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	v2 "github.com/brokenbots/criteria-adapter-proto/criteria/v2"
	"github.com/brokenbots/criteria-go-adapter-sdk/adapterhost"
	"github.com/brokenbots/criteriadb/pkg/memory"
	pb "github.com/brokenbots/criteriadb/pkg/pb/criteriadb/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type CriteriaDBAdapter struct {
	adapterhost.UnimplementedPermissions
	mu     sync.Mutex
	engine memory.Engine
}

func NewCriteriaDBAdapter(engine memory.Engine) *CriteriaDBAdapter {
	return &CriteriaDBAdapter{
		engine: engine,
	}
}

func (a *CriteriaDBAdapter) Info(context.Context, *v2.InfoRequest) (*v2.InfoResponse, error) {
	return &v2.InfoResponse{
		Name:         "criteriadb",
		Version:      "0.1.0",
		Description:  "CriteriaDB Agent Memory Graph Adapter for Criteria workflows",
		SourceUrl:    "https://github.com/brokenbots/criteriadb",
		Platforms:    []string{"linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64"},
		Capabilities: []string{"execute", "multi_turn", "structured_events"},
	}, nil
}

func (a *CriteriaDBAdapter) OpenSession(_ context.Context, req *v2.OpenSessionRequest) (*v2.OpenSessionResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.engine == nil {
		cfg := req.GetConfig()
		dbPath := cfg["db_path"]
		if dbPath == "" {
			dbPath = ".criteria/memory.db"
		}
		embedEndpoint := cfg["embedding_endpoint"]
		embedModel := cfg["embedding_model"]
		storageBackend := cfg["storage_backend"]
		postgresConn := cfg["postgres_conn_string"]

		eng, err := memory.NewMemoryEngine(memory.Config{
			StorageBackend:     storageBackend,
			StoragePath:        dbPath,
			PostgresConnString: postgresConn,
			EmbeddingEndpoint:  embedEndpoint,
			EmbeddingModel:     embedModel,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to open CriteriaDB memory engine: %w", err)
		}
		a.engine = eng
	}

	return &v2.OpenSessionResponse{}, nil
}

func (a *CriteriaDBAdapter) Execute(ctx context.Context, req *v2.ExecuteRequest, sender adapterhost.ExecuteEventSender) error {
	a.mu.Lock()
	eng := a.engine
	a.mu.Unlock()

	if eng == nil {
		return fmt.Errorf("CriteriaDB engine is not initialized")
	}

	inputs := req.GetInput()
	action := inputs["action"]
	if action == "" {
		action = "recall"
	}

	switch action {
	case "remember":
		label := inputs["label"]
		if label == "" {
			label = "Workflow Memory"
		}
		summary := inputs["summary"]
		nodeType := inputs["type"]
		if nodeType == "" {
			nodeType = "fact"
		}
		project := inputs["project"]
		folderPath := inputs["folder_path"]
		adapterID := inputs["adapter_id"]

		nodeID := inputs["node_id"]
		if nodeID == "" {
			nodeID = inputs["id"]
		}
		if nodeID == "" {
			nodeID = fmt.Sprintf("mem-%d", memory.NowTimestamp().UnixNano())
		}

		tense := pb.Tense_TENSE_PAST
		if tStr := inputs["tense"]; tStr != "" {
			switch strings.ToUpper(strings.TrimSpace(tStr)) {
			case "PRESENT", "TENSE_PRESENT":
				tense = pb.Tense_TENSE_PRESENT
			case "FUTURE", "TENSE_FUTURE":
				tense = pb.Tense_TENSE_FUTURE
			case "PLANNED", "TENSE_PLANNED":
				tense = pb.Tense_TENSE_PLANNED
			case "CONDITIONAL", "TENSE_CONDITIONAL":
				tense = pb.Tense_TENSE_CONDITIONAL
			}
		}

		visibility := pb.VisibilityScope_VISIBILITY_WORKFLOW
		if vStr := inputs["visibility"]; vStr != "" {
			switch strings.ToUpper(strings.TrimSpace(vStr)) {
			case "PRIVATE", "VISIBILITY_PRIVATE":
				visibility = pb.VisibilityScope_VISIBILITY_PRIVATE
			case "GLOBAL", "VISIBILITY_GLOBAL":
				visibility = pb.VisibilityScope_VISIBILITY_GLOBAL
			}
		}

		node := &pb.MemoryNode{
			Id:      nodeID,
			Label:   label,
			Summary: summary,
			Type:    nodeType,
			Temporal: &pb.TemporalInfo{
				Timestamp: timestamppb.New(memory.NowTimestamp()),
				ValidFrom: timestamppb.New(memory.NowTimestamp()),
				Tense:     tense,
			},
			DigitalLocation: &pb.DigitalLocation{
				Project:    project,
				FolderPath: folderPath,
			},
			AgentScope: &pb.AgentScope{
				CreatorAdapterId: adapterID,
				Visibility:       visibility,
			},
		}

		id, err := eng.Remember(ctx, node, nil)
		if err != nil {
			return err
		}

		ev, err := v2.NewExecuteResultEvent("remembered", map[string]any{
			"node_id": id,
			"label":   label,
			"success": true,
		})
		if err != nil {
			return err
		}
		return sender.Send(ev)

	case "remember_relation":
		sourceID := inputs["source_node_id"]
		targetID := inputs["target_node_id"]
		if sourceID == "" || targetID == "" {
			return fmt.Errorf("source_node_id and target_node_id are required for remember_relation")
		}
		relation := inputs["relation"]
		if relation == "" {
			relation = "INVOLVES"
		}
		weightStr := inputs["weight"]
		weight := 1.0
		if weightStr != "" {
			if w, err := strconv.ParseFloat(weightStr, 64); err == nil {
				weight = w
			}
		}

		edgeID := inputs["edge_id"]
		if edgeID == "" {
			edgeID = inputs["id"]
		}
		if edgeID == "" {
			edgeID = fmt.Sprintf("edge-%d", memory.NowTimestamp().UnixNano())
		}
		edge := &pb.MemoryEdge{
			Id:       edgeID,
			SourceId: sourceID,
			TargetId: targetID,
			Relation: relation,
			Weight:   weight,
			Temporal: &pb.TemporalInfo{
				Timestamp: timestamppb.New(memory.NowTimestamp()),
				ValidFrom: timestamppb.New(memory.NowTimestamp()),
				Tense:     pb.Tense_TENSE_PRESENT,
			},
		}

		err := eng.RememberEdge(ctx, edge)
		if err != nil {
			return fmt.Errorf("failed to remember edge: %w", err)
		}

		ev, err := v2.NewExecuteResultEvent("connected", map[string]any{
			"edge_id":        edgeID,
			"source_node_id": sourceID,
			"target_node_id": targetID,
			"relation":       relation,
			"success":        true,
		})
		if err != nil {
			return err
		}
		return sender.Send(ev)

	case "recall":
		queryText := inputs["query_text"]
		project := inputs["project"]
		callerID := inputs["caller_adapter_id"]

		scopeFilter := &pb.AdapterScopeFilter{
			CallerAdapterId:       callerID,
			IncludeWorkflowShared: true,
		}
		if targetID := inputs["target_adapter_id"]; targetID != "" {
			scopeFilter.TargetAdapterIds = []string{targetID}
		}

		var locFilter *pb.LocationQuery
		if project != "" {
			locFilter = &pb.LocationQuery{
				MatchProject: project,
			}
		}

		limit := int32(10)
		if limStr := inputs["limit"]; limStr != "" {
			if l, err := strconv.Atoi(limStr); err == nil && l > 0 {
				limit = int32(l)
			}
		}

		results, err := eng.Recall(ctx, &pb.QueryRequest{
			QueryText:      queryText,
			ScopeFilter:    scopeFilter,
			LocationFilter: locFilter,
			Limit:          limit,
		})
		if err != nil {
			return err
		}

		var memories []map[string]any
		for _, r := range results {
			memories = append(memories, map[string]any{
				"id":      r.Node.GetId(),
				"label":   r.Node.GetLabel(),
				"summary": r.Node.GetSummary(),
				"score":   r.Score,
			})
		}

		ev, err := v2.NewExecuteResultEvent("recalled", map[string]any{
			"count":    len(memories),
			"memories": memories,
			"success":  true,
		})
		if err != nil {
			return err
		}
		return sender.Send(ev)

	case "fact_lookup":
		queryText := inputs["query_text"]
		targetLabel := inputs["target_label"]
		relationStr := inputs["relation_types"]
		var relTypes []string
		if relationStr != "" {
			relTypes = strings.Split(relationStr, ",")
		}

		limit := int32(10)
		if limStr := inputs["limit"]; limStr != "" {
			if l, err := strconv.Atoi(limStr); err == nil && l > 0 {
				limit = int32(l)
			}
		}

		maxHops := int32(2)
		if hStr := inputs["max_hops"]; hStr != "" {
			if h, err := strconv.Atoi(hStr); err == nil && h > 0 {
				maxHops = int32(h)
			}
		}

		results, err := eng.Recall(ctx, &pb.QueryRequest{
			QueryText: queryText,
			FactFilter: &pb.FactQuery{
				RelationTypes: relTypes,
				TargetLabel:   targetLabel,
				MaxHops:       maxHops,
			},
			Limit: limit,
		})
		if err != nil {
			return err
		}

		var memories []map[string]any
		for _, r := range results {
			memories = append(memories, map[string]any{
				"id":      r.Node.GetId(),
				"label":   r.Node.GetLabel(),
				"summary": r.Node.GetSummary(),
				"score":   r.Score,
			})
		}

		ev, err := v2.NewExecuteResultEvent("found", map[string]any{
			"count":    len(memories),
			"memories": memories,
			"success":  true,
		})
		if err != nil {
			return err
		}
		return sender.Send(ev)

	default:
		return fmt.Errorf("unknown action: %s", action)
	}
}

func (a *CriteriaDBAdapter) Log(context.Context, *v2.LogRequest, adapterhost.LogEventSender) error {
	return nil
}

func (a *CriteriaDBAdapter) CloseSession(context.Context, *v2.CloseSessionRequest) (*v2.CloseSessionResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.engine != nil {
		_ = a.engine.Close()
		a.engine = nil
	}
	return &v2.CloseSessionResponse{}, nil
}
