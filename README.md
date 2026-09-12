# CriteriaDB

> **Portable, Pure-Go Agent Memory Graph Database for AI Workflows**

CriteriaDB (`github.com/brokenbots/criteriadb`) is an embedded and standalone memory graph database designed for agentic workflows running in **Criteria** (`github.com/brokenbots/criteria`). It provides multi-dimensional context indexing across temporal timelines, digital/physical location hierarchies, semantic vectors, and factual property graph edges — with strict **zero CGO** portability and **Protobuf-first** schemas.

---

## Key Features

- **100% Pure Go (`CGO_ENABLED=0`)**: Zero C dependencies. Uses pure-Go memory indexing and `go.etcd.io/bbolt` persistent single-file storage.
- **Protobuf-First Schemas (`proto/criteriadb/v1/criteriadb.proto`)**: Strict, language-agnostic Protocol Buffers definitions for nodes, edges, temporal info, location info, and gRPC/ConnectRPC interfaces.
- **Multi-Tier Embedding Strategies**:
  - **Zero-LLM Mode**: Pure-Go Lexical Token Search (BM25/Jaccard term matching) + Graph Traversal + Temporal Algebra + Location Scoping. Deterministic microsecond execution without LLM/vector requirements.
  - **Client / Adapter Vectors**: Pass pre-computed float32 vectors directly in Protobuf messages.
  - **Local HTTP Endpoint**: Auto-generate embeddings via local Ollama (`/v1/embeddings`) or local OpenAI-compatible APIs.
- **Adapter & Agent Scoping (RBAC & Privacy)**: Scoped access rules allowing agents to read private work, workflow-shared work, peer work, or supervisory cross-adapter graph memories.
- **Temporal & Tense Indexing**: Relevancy windows (`[ValidFrom, ValidTo]`), Due dates, Point-In-Time (`ValidAt`), and Tense matching (`past`, `present`, `future`, `planned`, `conditional`).
- **Digital Location Scoping**: Digital path hierarchy (`Machine`, `Repository`, `Project`, `FolderPath`, `FilePath`, `ServiceURL`).
- **Native Criteria Adapter**: Fully implements Criteria Adapter Protocol (v2) for seamless use inside HCL workflows.

---

## Quickstart & Build

```bash
# Clone and build binaries
git clone https://github.com/brokenbots/criteriadb.git
cd criteriadb

# Build binaries (bin/criteriadb & bin/criteria-adapter-criteriadb)
make build

# Run unit & integration test suite (CGO_ENABLED=0)
make test
```

---

## Using CriteriaDB in Criteria HCL Workflows

Reference CriteriaDB as an adapter block in your Criteria HCL workflow:

```hcl
workflow {
  name          = "agent_memory_demo"
  version       = "1"
  initial_state = "store_fact"
  target_state  = "done"
}

adapter "criteriadb" "memory" {
  source = "github.com/brokenbots/criteriadb"
  config {
    db_path = ".criteria/memory.db"
  }
}

step "store_fact" {
  target = adapter.criteriadb.memory
  input {
    action     = "remember"
    label      = "Auth Module Compiled"
    summary    = "FSM graph engine compiled successfully with zero CGO errors"
    type       = "task_done"
    project    = "criteria-auth"
    adapter_id = "copilot-dev-1"
  }
  outcome "remembered" { next = state.recall_context }
}

step "recall_context" {
  target = adapter.criteriadb.memory
  input {
    action            = "recall"
    query_text        = "Auth Module"
    project           = "criteria-auth"
    caller_adapter_id = "copilot-dev-1"
  }
  outcome "recalled" { next = state.done }
}

state "done" { terminal = true }
```

---

## Server & CLI Flags

Run the standalone gRPC server:

```bash
./bin/criteriadb \
  --port 8080 \
  --db-path .criteria/criteriadb.db \
  --embedding-endpoint http://localhost:11434/v1/embeddings \
  --embedding-model nomic-embed-text
```

| Flag | Default | Description |
| :--- | :--- | :--- |
| `--port` | `8080` | gRPC server port |
| `--db-path` | `criteriadb.db` | Path to persistent bbolt database file |
| `--embedding-endpoint` | `""` | Optional local HTTP embedding API (e.g. Ollama) |
| `--embedding-model` | `nomic-embed-text` | Model name for local embedding API |

---

## Developer Commands (`Makefile`)

```bash
make help          # Show all available Makefile targets
make build         # Build all binaries (CGO_ENABLED=0)
make test          # Run test suite
make proto         # Regenerate Go Protobuf structs from proto/
make lint          # Run fmt and vet checks
make clean         # Remove compiled binaries and temporary test databases
```

---

## License

Apache 2.0 / MIT
