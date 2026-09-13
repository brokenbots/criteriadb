package test

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/brokenbots/criteriadb/pkg/memory"
	pb "github.com/brokenbots/criteriadb/pkg/pb/criteriadb/v1"
	"github.com/brokenbots/criteriadb/pkg/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const bufSize = 1024 * 1024

func TestGRPCServer_RememberAndRecall(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "grpc_test.db")

	engine, err := memory.NewMemoryEngine(memory.Config{
		StoragePath: dbPath,
	})
	if err != nil {
		t.Fatalf("Failed to initialize engine: %v", err)
	}
	defer engine.Close()

	lis := bufconn.Listen(bufSize)
	gServer := grpc.NewServer()
	server.RegisterGRPCServer(gServer, engine)

	go func() {
		if err := gServer.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			t.Errorf("gServer error: %v", err)
		}
	}()
	defer gServer.GracefulStop()

	// Dial using bufconn listener
	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	client := pb.NewMemoryServiceClient(conn)

	// Step 1: Test Remember Node with Edge
	now := time.Now()
	remResp, err := client.Remember(ctx, &pb.RememberRequest{
		Node: &pb.MemoryNode{
			Id:      "grpc-node-1",
			Label:   "gRPC Architecture Decision",
			Summary: "CriteriaDB exposes pure-Go protobuf gRPC service for low-latency queries",
			Type:    "fact",
			Temporal: &pb.TemporalInfo{
				Timestamp: timestamppb.New(now),
				ValidFrom: timestamppb.New(now),
				Tense:     pb.Tense_TENSE_PRESENT,
			},
			DigitalLocation: &pb.DigitalLocation{
				Project: "criteriadb-grpc",
			},
			AgentScope: &pb.AgentScope{
				CreatorAdapterId: "test-client",
				Visibility:       pb.VisibilityScope_VISIBILITY_WORKFLOW,
			},
		},
		Edges: []*pb.MemoryEdge{
			{
				Id:       "edge-grpc-1",
				SourceId: "grpc-node-1",
				TargetId: "grpc-node-2",
				Relation: "EXPOSES",
				Weight:   1.0,
			},
		},
	})
	if err != nil {
		t.Fatalf("Remember gRPC call failed: %v", err)
	}
	if !remResp.GetSuccess() || remResp.GetNodeId() != "grpc-node-1" {
		t.Fatalf("Unexpected Remember response: %v", remResp)
	}

	// Step 2: Test Recall
	queryResp, err := client.Recall(ctx, &pb.QueryRequest{
		QueryText: "protobuf gRPC service",
		LocationFilter: &pb.LocationQuery{
			MatchProject: "criteriadb-grpc",
		},
		Limit: 5,
	})
	if err != nil {
		t.Fatalf("Recall gRPC call failed: %v", err)
	}
	if len(queryResp.GetResults()) == 0 {
		t.Fatalf("Expected at least 1 query result, got 0")
	}
	firstResult := queryResp.GetResults()[0]
	if firstResult.GetNode().GetId() != "grpc-node-1" {
		t.Fatalf("Expected node ID grpc-node-1, got %s", firstResult.GetNode().GetId())
	}
	if len(firstResult.GetConnectedEdges()) == 0 {
		t.Fatalf("Expected connected edge to be returned, got none")
	}

	// Step 3: Test Remember validation failure (empty node ID)
	errResp, err := client.Remember(ctx, &pb.RememberRequest{
		Node: &pb.MemoryNode{
			Id: "",
		},
	})
	if err == nil && (errResp == nil || errResp.GetSuccess()) {
		t.Fatalf("Expected error when remembering node with empty ID, got success")
	}
}
