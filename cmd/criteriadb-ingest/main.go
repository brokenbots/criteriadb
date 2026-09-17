package main

import (
	"bufio"
	"context"
	"flag"
	"io"
	"log"
	"os"
	"time"

	"github.com/brokenbots/criteriadb/pkg/criteria"
	"github.com/brokenbots/criteriadb/pkg/memory"
)

func main() {
	dbPath := flag.String("db-path", "criteriadb.db", "Path to CriteriaDB bbolt database file")
	eventsFile := flag.String("events-file", "-", "Path to Criteria ND-JSON event log file (or '-' for stdin)")
	flag.Parse()

	log.Printf("Starting CriteriaDB Live Event Ingester (CGO_ENABLED=0)...")

	engine, err := memory.NewMemoryEngine(memory.Config{
		StoragePath: *dbPath,
	})
	if err != nil {
		log.Fatalf("Failed to initialize CriteriaDB engine: %v", err)
	}
	defer engine.Close()

	ingester := criteria.NewIngester(engine)
	ctx := context.Background()

	var reader io.Reader
	if *eventsFile == "-" {
		reader = os.Stdin
		log.Printf("Reading ND-JSON events from STDIN...")
	} else {
		f, err := os.Open(*eventsFile)
		if err != nil {
			log.Fatalf("Failed to open events file %s: %v", *eventsFile, err)
		}
		defer f.Close()
		reader = f
		log.Printf("Tailing ND-JSON events from file %s...", *eventsFile)
	}

	scanner := bufio.NewScanner(reader)
	scanBuf := make([]byte, 64*1024)
	scanner.Buffer(scanBuf, 10*1024*1024) // Support large event payloads up to 10MB
	count := 0

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		nodeID, err := ingester.IngestNDJSONEvent(ctx, line)
		if err != nil {
			log.Printf("[Ingest Warning] skipping invalid event line: %v", err)
			continue
		}

		count++
		log.Printf("[Ingested #%d] Node ID: %s", count, nodeID)
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		log.Printf("Scanner error: %v", err)
	}

	log.Printf("Completed ingesting %d events into CriteriaDB.", count)
	time.Sleep(100 * time.Millisecond)
}
