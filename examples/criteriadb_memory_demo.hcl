workflow {
  name          = "criteriadb_memory_demo"
  version       = "1"
  initial_state = "remember_architectural_decision"
  target_state  = "memory_demo_completed"

  policy {
    max_total_steps = 50
  }
}

// ── CriteriaDB Memory Adapter Declaration ───────────────────────────────────

adapter "criteriadb" "memory" {
  config {
    db_path            = ".criteria/demo_memory.db"
    embedding_endpoint = "http://localhost:11434/v1/embeddings"
    embedding_model    = "embeddinggemma:latest"
  }
}

// ── Workflow Steps ──────────────────────────────────────────────────────────

// Step 1: Remember Root Architectural Decision
step "remember_architectural_decision" {
  target = adapter.criteriadb.memory
  input {
    action      = "remember"
    node_id     = "mem-zero-cgo"
    label       = "Zero-CGO Persistence"
    summary     = "CriteriaDB relies exclusively on pure-Go bbolt single-file storage for maximum cross-platform portability."
    type        = "fact"
    project     = "criteriadb-demo"
    folder_path = "pkg/memory"
    adapter_id  = "arch-agent"
  }
  outcome "remembered" { next = state.remember_dependent_design }
  outcome "failure"    { next = state.demo_failed }
}

// Step 2: Remember Dependent Component Design
step "remember_dependent_design" {
  target = adapter.criteriadb.memory
  input {
    action      = "remember"
    node_id     = "mem-oci-publisher"
    label       = "Multi-Platform OCI Publisher"
    summary     = "The publish action cross-compiles linux and darwin binaries and packages them into a signed OCI container."
    type        = "fact"
    project     = "criteriadb-demo"
    folder_path = ".github/workflows"
    adapter_id  = "ci-agent"
  }
  outcome "remembered" { next = state.link_dependency_edge }
  outcome "failure"    { next = state.demo_failed }
}

// Step 3: Connect Graph Nodes via Directed Edge
step "link_dependency_edge" {
  target = adapter.criteriadb.memory
  input {
    action         = "remember_relation"
    source_node_id = "mem-zero-cgo"
    target_node_id = "mem-oci-publisher"
    relation       = "DEPENDS_ON"
    weight         = "0.95"
  }
  outcome "connected" { next = state.recall_semantic_context }
  outcome "failure"   { next = state.demo_failed }
}

// Step 4: Semantic & Lexical Recall
step "recall_semantic_context" {
  target = adapter.criteriadb.memory
  input {
    action            = "recall"
    query_text        = "pure-Go single-file storage and cross-platform portability"
    project           = "criteriadb-demo"
    caller_adapter_id = "reasoning-agent"
  }
  outcome "recalled" { next = state.graph_fact_lookup }
  outcome "failure"  { next = state.demo_failed }
}

// Step 5: Graph Relation Traversal Fact Lookup
step "graph_fact_lookup" {
  target = adapter.criteriadb.memory
  input {
    action         = "fact_lookup"
    query_text     = "cross-platform"
    target_label   = "Multi-Platform OCI Publisher"
    relation_types = "DEPENDS_ON,INVOLVES"
  }
  outcome "found"   { next = state.memory_demo_completed }
  outcome "failure" { next = state.demo_failed }
}

// ── Terminal States ──────────────────────────────────────────────────────────

state "memory_demo_completed" {
  terminal = true
}

state "demo_failed" {
  terminal = true
  success  = false
}
