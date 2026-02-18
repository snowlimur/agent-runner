# Data Codemap

Updated: 2026-02-17

## Go Types

### Run Record (`stats/types.go`)

```
RunRecord
├── RunID              string
├── Timestamp          time.Time
├── Status             RunStatus (success | error | parse_error | exec_error)
├── DockerExitCode     int
├── CWD                string
├── Pipeline           *PipelineRunRecord (optional)
│   ├── Version        string
│   ├── Status         string
│   ├── IsError        bool
│   ├── StageCount     int
│   ├── CompletedStages int
│   ├── TaskCount      int
│   ├── FailedTaskCount int
│   └── Tasks[]        PipelineTaskRecord
│       ├── stage/task status metadata (ids, timing, exit, error)
│       └── Normalized *PipelineTaskNormalized (optional)
│           ├── InputTokens, CacheCreation/ReadTokens, OutputTokens
│           ├── CostUSD, WebSearchRequests
│           └── ByModel map[string]PipelineTaskModelMetric
├── AgentResult        *AgentResult (optional)
├── Normalized         NormalizedMetrics
│   ├── DurationMS, DurationAPIMS, NumTurns
│   ├── TotalCostUSD
│   ├── InputTokens, CacheCreation/ReadTokens, OutputTokens
│   └── ByModel        map[string]ModelMetric
├── ErrorType          string
└── ErrorMessage       string
```

### Stream Events (`result/stream_parser.go`)

```
StreamEvent
├── Raw       string
├── Type      string
├── System    *SystemEvent    (subtype, session_id, model)
├── Assistant *AssistantEvent (message_id, session_id, content[])
│   └── Content → AssistantToolUse (id, name, input: command/description/todos)
├── User      *UserEvent      (session_id, tool_results[], tool_use_result)
│   ├── ToolResults[]  (tool_use_id, type, content, is_error)
│   └── ToolUseResult  (stdout, stderr, interrupted, oldTodos, newTodos)
├── Pipeline  *PipelineEvent  (event, stage_id, task_id, session_id, status, error_message, idle_timeout_sec, reason)
└── Result    *AgentResult    (type, subtype, is_error, usage, modelUsage, ...)
```

### Config (`config/config.go`)

```
Config
├── Docker    (image, model, enable_dind, run_idle_timeout_sec, pipeline_task_idle_timeout_sec)
├── Auth      (github_token, claude_token)
├── Workspace (source_workspace_dir)
└── Git       (user_name, user_email)
```

## TypeScript Types (`types.ts`)

### Pipeline Plan

```
PipelinePlan
├── version    PipelineVersion ("v1")
├── defaults   PipelineDefaults (model, verbosity, onError, workspace, taskIdleTimeoutSec)
└── stages[]   PipelineStage
    ├── id, mode (sequential | parallel), maxParallel, taskIdleTimeoutSec
    ├── onError, workspace, model, verbosity
    └── tasks[]  PipelineTask
        ├── id, promptText, promptFile (PromptFileRef | null)
        ├── onError, workspace, model, verbosity, taskIdleTimeoutSec
        └── readOnly, allowSharedWrites
```

### Pipeline Results

```
PipelineResult (emitted as final JSON on stdout)
├── type: "pipeline_result"
├── version, status, is_error
├── stage_count, completed_stages, task_count, failed_task_count
└── tasks[]  PipelineTaskResult
    ├── stage_id, task_id, status, on_error
    ├── workspace, model, verbosity, prompt_source, prompt_file
    ├── exit_code, signal, started_at, finished_at, duration_ms
    └── error_message
```

Note: `agent-cli` enriches `stats.json` pipeline tasks with optional
`normalized` usage metrics when a task has bound `result` events.

### Pipeline Events (emitted on stdout during execution)

| Event | Key Fields |
|-------|------------|
| `plan_start` | version, started_at, stage_count |
| `stage_start` | stage_id, mode, task_count, max_parallel |
| `task_start` | stage_id, task_id, model, verbosity, workspace, task_idle_timeout_sec |
| `task_timeout` | stage_id, task_id, idle_timeout_sec, reason |
| `task_session_bind` | stage_id, task_id, session_id |
| `task_finish` | PipelineTaskResult |
| `stage_finish` | PipelineStageResult |
| `plan_finish` | status, duration_ms, stage/task counts |

## Storage Layout

```
<project>/.agent-cli/
├── config.toml                          # Project config
└── runs/
    └── <YYYYMMDDTHHMMSS>-<hex_id>/     # Per-run directory
        ├── stats.json                   # RunRecord
        ├── output.ndjson                # JSON object logs (NDJSON)
        └── output.log                   # Non-JSON-object lines
```
