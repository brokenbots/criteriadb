workflow {
  name          = "criteriadb_memory_demo"
  version       = "1"
  initial_state = "remember_task"
  target_state  = "done"
}

adapter "criteriadb" "memory" {
  config {
    db_path = ".criteria/memory.db"
  }
}

step "remember_task" {
  target = adapter.criteriadb.memory
  input {
    action     = "remember"
    label      = "Auth Module Compiled"
    summary    = "FSM graph engine compiled successfully with zero CGO errors"
    type       = "task_done"
    project    = "criteria-auth"
    adapter_id = "copilot-dev-1"
  }
  outcome "remembered" { next = state.recall_task }
  outcome "failure"    { next = state.failed }
}

step "recall_task" {
  target = adapter.criteriadb.memory
  input {
    action            = "recall"
    query_text        = "Auth Module"
    project           = "criteria-auth"
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
