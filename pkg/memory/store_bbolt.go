package memory

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

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

var _ Store = (*BBoltStore)(nil)

func NewBBoltStore(dbPath string) (*BBoltStore, error) {
	if dbPath == "" {
		return &BBoltStore{db: nil}, nil
	}

	dir := filepath.Dir(dbPath)
	if dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}

	opts := &bbolt.Options{
		Timeout: 3 * time.Second,
	}

	db, err := bbolt.Open(dbPath, 0600, opts)
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
		if b == nil {
			return fmt.Errorf("bucket nodes does not exist")
		}
		return b.Put([]byte(node.GetId()), data)
	})
}

func (s *BBoltStore) DeleteNode(id string) error {
	if s.db == nil || id == "" {
		return nil
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketNodes)
		if b == nil {
			return nil
		}
		return b.Delete([]byte(id))
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
		if b == nil {
			return fmt.Errorf("bucket edges does not exist")
		}
		return b.Put([]byte(edge.GetId()), data)
	})
}

func (s *BBoltStore) DeleteEdge(id string) error {
	if s.db == nil || id == "" {
		return nil
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketEdges)
		if b == nil {
			return nil
		}
		return b.Delete([]byte(id))
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
		if bNodes != nil {
			_ = bNodes.ForEach(func(k, v []byte) error {
				var node pb.MemoryNode
				if err := proto.Unmarshal(v, &node); err == nil {
					nodes = append(nodes, &node)
				} else {
					log.Printf("[BBoltStore] Warning: failed to unmarshal node %s: %v", string(k), err)
				}
				return nil
			})
		}

		bEdges := tx.Bucket(bucketEdges)
		if bEdges != nil {
			_ = bEdges.ForEach(func(k, v []byte) error {
				var edge pb.MemoryEdge
				if err := proto.Unmarshal(v, &edge); err == nil {
					edges = append(edges, &edge)
				} else {
					log.Printf("[BBoltStore] Warning: failed to unmarshal edge %s: %v", string(k), err)
				}
				return nil
			})
		}
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
