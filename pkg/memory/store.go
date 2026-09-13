package memory

import (
	pb "github.com/brokenbots/criteriadb/pkg/pb/criteriadb/v1"
)

// Store defines the persistent storage engine interface for CriteriaDB.
// Implemented by BBoltStore (embedded) and PostgresStore (distributed SQL).
type Store interface {
	SaveNode(node *pb.MemoryNode) error
	DeleteNode(id string) error
	SaveEdge(edge *pb.MemoryEdge) error
	DeleteEdge(id string) error
	LoadAll() ([]*pb.MemoryNode, []*pb.MemoryEdge, error)
	Close() error
}
