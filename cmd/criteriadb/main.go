package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/brokenbots/criteriadb/pkg/memory"
	pb "github.com/brokenbots/criteriadb/pkg/pb/criteriadb/v1"
	"github.com/brokenbots/criteriadb/pkg/server"
	"google.golang.org/grpc"
)

type grpcServer struct {
	pb.UnimplementedMemoryServiceServer
	engine *memory.MemoryEngine
}

func (s *grpcServer) Remember(ctx context.Context, req *pb.RememberRequest) (*pb.RememberResponse, error) {
	nodeID, err := s.engine.Remember(ctx, req.GetNode(), req.GetEdges())
	if err != nil {
		return &pb.RememberResponse{Success: false}, err
	}
	return &pb.RememberResponse{NodeId: nodeID, Success: true}, nil
}

func (s *grpcServer) Recall(ctx context.Context, req *pb.QueryRequest) (*pb.QueryResponse, error) {
	results, err := s.engine.Recall(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pb.QueryResponse{Results: results}, nil
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	subcommand := os.Args[1]

	switch subcommand {
	case "serve":
		runServe(os.Args[2:])
	case "query":
		runQuery(os.Args[2:])
	case "list":
		runList(os.Args[2:])
	case "stats":
		runStats(os.Args[2:])
	case "consolidate":
		runConsolidate(os.Args[2:])
	default:
		// Fallback to serve for backward compatibility if unknown flag
		runServe(os.Args[1:])
	}
}

func printUsage() {
	fmt.Println("Usage: criteriadb <command> [options]")
	fmt.Println("\nAvailable Commands:")
	fmt.Println("  serve        Start the CriteriaDB gRPC server & web visualizer dashboard")
	fmt.Println("  query        Perform a multi-dimensional terminal memory search")
	fmt.Println("  list         List all stored memory nodes and details")
	fmt.Println("  stats        Display summary memory statistics")
	fmt.Println("  consolidate  Prune expired memories and consolidate event nodes")
}

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 8080, "gRPC server port")
	webPort := fs.Int("web-port", 8081, "Web visualizer dashboard HTTP port (0 to disable)")
	dbPath := fs.String("db-path", "criteriadb.db", "Path to bbolt database file")
	embedEndpoint := fs.String("embedding-endpoint", "", "Optional local HTTP embedding endpoint URL")
	embedModel := fs.String("embedding-model", "nomic-embed-text", "Embedding model name")
	_ = fs.Parse(args)

	log.Printf("Starting CriteriaDB Server (Pure Go, Zero CGO)...")
	engine, err := memory.NewMemoryEngine(memory.Config{
		StoragePath:       *dbPath,
		EmbeddingEndpoint: *embedEndpoint,
		EmbeddingModel:    *embedModel,
	})
	if err != nil {
		log.Fatalf("Failed to initialize engine: %v", err)
	}
	defer engine.Close()

	if *webPort > 0 {
		webAddr := fmt.Sprintf("127.0.0.1:%d", *webPort)
		go func() {
			if err := server.StartWebServer(webAddr, engine); err != nil {
				log.Printf("Web visualizer server error: %v", err)
			}
		}()
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("Failed to listen on port %d: %v", *port, err)
	}

	gServer := grpc.NewServer()
	pb.RegisterMemoryServiceServer(gServer, &grpcServer{engine: engine})

	log.Printf("CriteriaDB gRPC server listening on :%d", *port)
	if err := gServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}

func runQuery(args []string) {
	fs := flag.NewFlagSet("query", flag.ExitOnError)
	dbPath := fs.String("db-path", "criteriadb.db", "Path to bbolt database file")
	queryText := fs.String("text", "", "Query text to search")
	project := fs.String("project", "", "Filter by project")
	limit := fs.Int("limit", 10, "Max results")
	_ = fs.Parse(args)

	if *queryText == "" && len(fs.Args()) > 0 {
		*queryText = fs.Args()[0]
	}

	engine, err := memory.NewMemoryEngine(memory.Config{StoragePath: *dbPath})
	if err != nil {
		log.Fatalf("Failed to open DB: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()
	var locFilter *pb.LocationQuery
	if *project != "" {
		locFilter = &pb.LocationQuery{MatchProject: *project}
	}

	results, err := engine.Recall(ctx, &pb.QueryRequest{
		QueryText:      *queryText,
		LocationFilter: locFilter,
		ScopeFilter:    &pb.AdapterScopeFilter{TargetAdapterIds: []string{"*"}},
		Limit:          int32(*limit),
	})
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}

	fmt.Printf("\n--- CriteriaDB Search Results (%d matches) ---\n", len(results))
	for i, r := range results {
		fmt.Printf("[%d] Score: %.4f | ID: %s | Type: %s | Tense: %s\n",
			i+1, r.Score, r.Node.GetId(), r.Node.GetType(), r.Node.GetTemporal().GetTense().String())
		fmt.Printf("    Label:   %s\n", r.Node.GetLabel())
		fmt.Printf("    Summary: %s\n\n", r.Node.GetSummary())
	}
}

func runList(args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	dbPath := fs.String("db-path", "criteriadb.db", "Path to bbolt database file")
	_ = fs.Parse(args)

	engine, err := memory.NewMemoryEngine(memory.Config{StoragePath: *dbPath})
	if err != nil {
		log.Fatalf("Failed to open DB: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()
	results, err := engine.Recall(ctx, &pb.QueryRequest{
		ScopeFilter: &pb.AdapterScopeFilter{TargetAdapterIds: []string{"*"}},
		Limit:       1000,
	})
	if err != nil {
		log.Fatalf("List failed: %v", err)
	}

	fmt.Printf("\n--- CriteriaDB Memory Nodes (%d total) ---\n", len(results))
	for i, r := range results {
		fmt.Printf("[%d] ID: %-25s | Type: %-15s | Project: %s\n",
			i+1, r.Node.GetId(), r.Node.GetType(), r.Node.GetDigitalLocation().GetProject())
		fmt.Printf("    Label: %s\n", r.Node.GetLabel())
	}
}

func runStats(args []string) {
	fs := flag.NewFlagSet("stats", flag.ExitOnError)
	dbPath := fs.String("db-path", "criteriadb.db", "Path to bbolt database file")
	_ = fs.Parse(args)

	engine, err := memory.NewMemoryEngine(memory.Config{StoragePath: *dbPath})
	if err != nil {
		log.Fatalf("Failed to open DB: %v", err)
	}
	defer engine.Close()

	stats := engine.GetStats()
	fmt.Printf("\n--- CriteriaDB Memory Statistics ---\n")
	fmt.Printf("Total Memory Nodes: %d\n", stats.TotalNodes)
	fmt.Printf("Total Graph Edges:  %d\n", stats.TotalEdges)
	fmt.Println("\nNodes By Type:")
	for k, v := range stats.NodesByType {
		fmt.Printf("  • %-20s: %d\n", k, v)
	}
	fmt.Println("\nNodes By Tense:")
	for k, v := range stats.NodesByTense {
		fmt.Printf("  • %-20s: %d\n", k, v)
	}
}

func runConsolidate(args []string) {
	fs := flag.NewFlagSet("consolidate", flag.ExitOnError)
	dbPath := fs.String("db-path", "criteriadb.db", "Path to bbolt database file")
	project := fs.String("project", "", "Project to consolidate")
	nodeType := fs.String("type", "", "Node type to consolidate")
	_ = fs.Parse(args)

	engine, err := memory.NewMemoryEngine(memory.Config{StoragePath: *dbPath})
	if err != nil {
		log.Fatalf("Failed to open DB: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()
	pruned, err := engine.PruneExpired(ctx)
	if err != nil {
		log.Printf("Pruning warning: %v", err)
	} else if pruned > 0 {
		fmt.Printf("Pruned %d expired memory nodes.\n", pruned)
	}

	consNode, count, err := engine.Consolidate(ctx, *project, *nodeType)
	if err != nil {
		log.Fatalf("Consolidation failed: %v", err)
	}
	if count > 0 {
		fmt.Printf("Successfully consolidated %d nodes into new fact node %s!\n", count, consNode.GetId())
	} else {
		fmt.Println("No nodes required consolidation.")
	}
}
