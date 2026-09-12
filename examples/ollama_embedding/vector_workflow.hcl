workflow {
  name          = "ollama_vector_embedding_demo"
  version       = "1"
  initial_state = "remember_with_vectors"
  target_state  = "done"
}

# Configured with local HTTP embedding endpoint (Ollama / OpenAI API compatible)
adapter "criteriadb" "memory" {
  config {
    db_path            = ".criteria/vector_memory.db"
    embedding_endpoint = "http://localhost:11434/v1/embeddings"
    embedding_model    = "embeddinggemma:latest"
  }
}

step "remember_with_vectors" {
  target = adapter.criteriadb.memory
  input {
    action     = "remember"
    label      = "Golang Microservice Refactoring"
    summary    = "Optimized gRPC stream handlers and memory allocation in Go binary"
    type       = "task_done"
    project    = "criteria-core"
    adapter_id = "copilot-dev-1"
  }
  outcome "remembered" { next = state.vector_recall }
  outcome "failure"    { next = state.failed }
}

step "vector_recall" {
  target = adapter.criteriadb.memory
  input {
    action            = "recall"
    query_text        = "Go memory optimization and gRPC"
    project           = "criteria-core"
    caller_adapter_id = "copilot-dev-1"
  }
  outcome "recalled" { next = state.done }
  outcome "failure"  { next = state.failed }
}

state "done" {
  terminal = true
}

state "failed" {
  terminal = true
  success  = false
}
