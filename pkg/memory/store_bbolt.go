package memory

import (
	"fmt"
	"os"
	"path/filepath"

	pb "github.com/brokenbots/criteriadb/pkg/pb/criteriadb/v1"
	bbolt "go.etcd.io/bbolt"
	"google.golang.org/protobuf/proto"
)

var (
	bucketNodes = []byte("nodes")
	bucketEdges = []byte("edges")
)

// BBoltStore handles persistent pure-Go disk storage of memory nodes and edges.
type BBoltStore struct {
	db *bbolt.DB
}

func NewBBoltStore(dbPath string) (*BBoltStore, error) {
	if dbPath == "" {
		return &BBoltStore{db: nil}, nil
	}

	dir := filepath.Dir(dbPath)
	if dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}

	db, err := bbolt.Open(dbPath, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open bbolt db at %s: %w", dbPath, err)
	}

	err = db.Update(func(tx *bbolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(bucketNodes); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(bucketEdges); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to create buckets: %w", err)
	}

	return &BBoltStore{db: db}, nil
}

func (s *BBoltStore) SaveNode(node *pb.MemoryNode) error {
	if s.db == nil || node == nil || node.GetId() == "" {
		return nil
	}
	data, err := proto.Marshal(node)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketNodes)
		return b.Put([]byte(node.GetId()), data)
	})
}

func (s *BBoltStore) SaveEdge(edge *pb.MemoryEdge) error {
	if s.db == nil || edge == nil || edge.GetId() == "" {
		return nil
	}
	data, err := proto.Marshal(edge)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketEdges)
		return b.Put([]byte(edge.GetId()), data)
	})
}

func (s *BBoltStore) LoadAll() ([]*pb.MemoryNode, []*pb.MemoryEdge, error) {
	if s.db == nil {
		return nil, nil, nil
	}

	var nodes []*pb.MemoryNode
	var edges []*pb.MemoryEdge

	err := s.db.View(func(tx *bbolt.Tx) error {
		bNodes := tx.Bucket(bucketNodes)
		_ = bNodes.ForEach(func(k, v []byte) error {
			var node pb.MemoryNode
			if err := proto.Unmarshal(v, &node); err == nil {
				nodes = append(nodes, &node)
			}
			return nil
		})

		bEdges := tx.Bucket(bucketEdges)
		_ = bEdges.ForEach(func(k, v []byte) error {
			var edge pb.MemoryEdge
			if err := proto.Unmarshal(v, &edge); err == nil {
				edges = append(edges, &edge)
			}
			return nil
		})
		return nil
	})

	if err != nil {
		return nil, nil, err
	}

	return nodes, edges, nil
}

func (s *BBoltStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
