# CriteriaDB

> **Portable, Pure-Go Agent Memory Graph Database for AI Workflows**

CriteriaDB (`github.com/brokenbots/criteriadb`) is a portable, pure-Go agent memory graph database designed for AI workflows running in **Criteria** (`github.com/brokenbots/criteria`). It provides multi-dimensional context indexing across temporal timelines, digital/physical location hierarchies, 768D float32 SIMD vector embeddings, zero-LLM lexical search, and directed property graph edges — with strict **zero CGO** portability (`CGO_ENABLED=0`) and **Protobuf-first** schemas.

---

## Architecture Diagram

```text
┌──────────────────────────────────────────────────────────────────────────────────┐
│                             Criteria Agent Workflow                              │
│                               (HCL Workflow Step)                                │
└────────────────────────────────────────┬─────────────────────────────────────────┘
                                         │ Criteria Adapter Protocol (v2)
                                         ▼
┌──────────────────────────────────────────────────────────────────────────────────┐
│                   CriteriaDB Adapter (cmd/criteria-adapter-criteriadb)            │
│                     Actions: remember, remember_relation, recall, fact_lookup     │
└────────────────────────────────────────┬─────────────────────────────────────────┘
                                         │ gRPC / Direct Go API
                                         ▼
┌──────────────────────────────────────────────────────────────────────────────────┐
│                             CriteriaDB Memory Engine                             │
│                           (pkg/memory/ - CGO_ENABLED=0)                          │
├─────────────────────────┬─────────────────────────┬──────────────────────────────┤
│    Lexical Inverted     │  768D Vector Cosine     │   Directed Property Graph    │
│    Index (Zero-LLM)     │   Similarity Engine     │    (Nodes & Directed Edges)  │
├─────────────────────────┴─────────────────────────┴──────────────────────────────┤
│                      Pure-Go bbolt Storage (go.etcd.io/bbolt)                    │
└────────────────────────────────────────┬─────────────────────────────────────────┘
                                         │ HTTP REST API (port 8081)
                                         ▼
┌──────────────────────────────────────────────────────────────────────────────────┐
│            Interactive D3.js 2D Force-Graph Visualizer Dashboard                 │
│               (Real-Time Filters, Search & Node Inspection Side Panel)           │
└──────────────────────────────────────────────────────────────────────────────────┘
```

---

## Published OCI Adapter Image & Releases

CriteriaDB adapter binaries are automatically cross-compiled for multiple platforms (`linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`), signed keylessly with Cosign, and published to GitHub Container Registry (GHCR):

- **OCI Image Target**: `ghcr.io/brokenbots/criteria-adapter-criteriadb:0.1.0`
- **Release Tag**: [`v0.1.0`](https://github.com/brokenbots/criteriadb/releases/tag/v0.1.0)
- **Signature Verification**:
  ```bash
  cosign verify ghcr.io/brokenbots/criteria-adapter-criteriadb:0.1.0 \
    --certificate-identity-regexp="https://github.com/brokenbots/criteriadb/.*" \
    --certificate-oidc-issuer="https://token.actions.githubusercontent.com"
  ```

---

## Performance Benchmarks

Engine performance measured on Apple M4 Max (`go test -bench=. -benchmem ./test/...`):

| Benchmark Operation | Throughput / Latency | Allocations | Description |
| :--- | :--- | :--- | :--- |
| `BenchmarkEngine_Remember` | `9.04 ms/op` | `28.7 KB / 63 allocs` | Persistent bbolt transaction write |
| `BenchmarkEngine_Recall_Vector_1k` | **`1.31 ms/op`** | `3.2 MB / 2,014 allocs` | 768D Float32 vector similarity search over 1k nodes |
| `BenchmarkEngine_Recall_Lexical_1k` | **`1.62 ms/op`** | `2.6 MB / 16,013 allocs` | Zero-LLM lexical token search over 1k nodes |
| `BenchmarkEngine_Consolidate` | **`302.5 ns/op`** | `0 B / 0 allocs` | Event node consolidation & expired node pruning |

---

## Quickstart & Build

```bash
# Clone repository
git clone https://github.com/brokenbots/criteriadb.git
cd criteriadb

# Build all binaries (bin/criteriadb, bin/criteria-adapter-criteriadb, bin/criteriadb-ingest)
make build

# Run unit test suite
make test

# Run performance benchmark suite
go test -bench=. -benchmem ./test/...
```

---

## CLI Subcommands (`criteriadb`)

```bash
criteriadb <command> [options]
```

### 1. `serve` — Start gRPC Server & Visualizer
```bash
criteriadb serve --port 8080 --web-port 8081 --db-path criteriadb.db
```

### 2. `query` — Terminal Search
```bash
criteriadb query --db-path criteriadb.db --text "Zero-CGO persistence" --project criteriadb-demo
```

### 3. `list` — List All Stored Memory Nodes
```bash
criteriadb list --db-path criteriadb.db
```

### 4. `stats` — Memory Graph Statistics
```bash
criteriadb stats --db-path criteriadb.db
```

### 5. `consolidate` — Memory Pruning & Event Merging
```bash
criteriadb consolidate --db-path criteriadb.db --project criteriadb-demo --type fact
```

### 6. `export` — Export Graph to JSON Backup
```bash
criteriadb export --db-path criteriadb.db --output backup.json
```

### 7. `import` — Import Graph from JSON Backup
```bash
criteriadb import --db-path new_db.db --input backup.json
```

### 8. `viz` — Standalone Interactive D3 Visualizer
```bash
criteriadb viz --db-path criteriadb.db --port 8081
```

---

## Criteria HCL Workflow Example

Reference CriteriaDB inside a Criteria workflow (`examples/criteriadb_memory_demo.hcl`):

```hcl
workflow {
  name          = "criteriadb_memory_demo"
  version       = "1"
  initial_state = "remember_architectural_decision"
  target_state  = "memory_demo_completed"
}

adapter "criteriadb" "memory" {
  config {
    db_path            = ".criteria/demo_memory.db"
    embedding_endpoint = "http://localhost:11434/v1/embeddings"
    embedding_model    = "embeddinggemma:latest"
  }
}

step "remember_architectural_decision" {
  target = adapter.criteriadb.memory
  input {
    action      = "remember"
    label       = "Zero-CGO Persistence"
    summary     = "CriteriaDB relies exclusively on pure-Go bbolt storage for cross-platform portability."
    type        = "fact"
    project     = "criteriadb-demo"
    adapter_id  = "arch-agent"
  }
  outcome "remembered" { next = state.recall_semantic_context }
}

step "recall_semantic_context" {
  target = adapter.criteriadb.memory
  input {
    action            = "recall"
    query_text        = "pure-Go single-file storage and cross-platform portability"
    project           = "criteriadb-demo"
    caller_adapter_id = "reasoning-agent"
  }
  outcome "recalled" { next = state.memory_demo_completed }
}

state "memory_demo_completed" { terminal = true }
```

Run with `criteria`:
```bash
criteria apply examples/criteriadb_memory_demo.hcl
```

---

## License

Apache 2.0 / MIT
