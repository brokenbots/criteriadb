workflow {
  name          = "multi_agent_memory_sharing"
  version       = "1"
  initial_state = "shell_build_step"
  target_state  = "done"
}

adapter "criteriadb" "memory" {
  config {
    db_path = ".criteria/shared_memory.db"
  }
}

step "shell_build_step" {
  target = adapter.criteriadb.memory
  input {
    action     = "remember"
    label      = "Compiled Go Binary"
    summary    = "Built bin/criteriadb binary with CGO_ENABLED=0"
    type       = "build_artifact"
    project    = "criteria-suite"
    adapter_id = "shell-builder-1"
  }
  outcome "remembered" { next = state.agent_recall_step }
  outcome "failure"    { next = state.failed }
}

step "agent_recall_step" {
  target = adapter.criteriadb.memory
  input {
    action            = "recall"
    query_text        = "Compiled Go Binary"
    project           = "criteria-suite"
    caller_adapter_id = "copilot-agent-1"
    target_adapter_id = "shell-builder-1"
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
