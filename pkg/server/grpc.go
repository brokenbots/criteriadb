package server

import (
	"context"

	"github.com/brokenbots/criteriadb/pkg/memory"
	pb "github.com/brokenbots/criteriadb/pkg/pb/criteriadb/v1"
	"google.golang.org/grpc"
)

// MemoryGRPCServer implements the CriteriaDB MemoryServiceServer gRPC interface.
type MemoryGRPCServer struct {
	pb.UnimplementedMemoryServiceServer
	engine *memory.MemoryEngine
}

// NewMemoryGRPCServer creates a new gRPC service instance wrapping MemoryEngine.
func NewMemoryGRPCServer(engine *memory.MemoryEngine) *MemoryGRPCServer {
	return &MemoryGRPCServer{engine: engine}
}

// Remember stores or updates a MemoryNode and any associated directed edges in CriteriaDB.
func (s *MemoryGRPCServer) Remember(ctx context.Context, req *pb.RememberRequest) (*pb.RememberResponse, error) {
	nodeID, err := s.engine.Remember(ctx, req.GetNode(), req.GetEdges())
	if err != nil {
		return &pb.RememberResponse{Success: false}, err
	}
	return &pb.RememberResponse{NodeId: nodeID, Success: true}, nil
}

// Recall executes a multi-dimensional semantic, lexical, temporal, location, and fact query.
func (s *MemoryGRPCServer) Recall(ctx context.Context, req *pb.QueryRequest) (*pb.QueryResponse, error) {
	results, err := s.engine.Recall(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pb.QueryResponse{Results: results}, nil
}

// RegisterGRPCServer registers the MemoryServiceServer with a gRPC server.
func RegisterGRPCServer(gServer *grpc.Server, engine *memory.MemoryEngine) {
	pb.RegisterMemoryServiceServer(gServer, NewMemoryGRPCServer(engine))
}
