# Data Codemap

Updated: 2026-02-18

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
│   ├── EntryNode      string
│   ├── TerminalNode   string
│   ├── TerminalStatus string
│   ├── ExitCode       int
│   ├── Iterations     int
│   ├── NodeRunCount   int
│   ├── FailedNodeCount int
│   └── NodeRuns[]     PipelineNodeRunRecord
│       ├── node metadata (node_id, node_run_id, kind, status, timing, exit, error)
│       └── Normalized *PipelineNodeRunNormalized (optional)
│           ├── InputTokens, CacheCreation/ReadTokens, OutputTokens
│           ├── CostUSD, WebSearchRequests
│           └── ByModel map[string]PipelineNodeRunModelMetric
├── AgentResult        *AgentResult (optional)
├── Normalized         NormalizedMetrics
└── ErrorType/ErrorMessage
```

### Stream Events (`result/stream_parser.go`)

```
StreamEvent
├── System    (subtype, session_id, model)
├── Assistant (tool_use payloads)
├── User      (tool_result payloads)
├── Pipeline  (event, node_id, node_run_id, session_id, status, transition fields, exit/timeout fields)
└── Result    (final Claude result)
```

## TypeScript Types (`images/entrypoint/src/lib/types.ts`)

### Pipeline Plan v2

```
PipelinePlan
├── version: "v2"
├── entryNode
├── defaults
│   ├── model
│   ├── agentIdleTimeoutSec
│   └── commandTimeoutSec
├── limits
│   ├── maxIterations
│   └── maxSameNodeHits
└── nodes (mapping)
    ├── PipelineExecutableNode
    │   ├── run.kind = agent | command
    │   └── transitions[] (when -> to)
    └── PipelineTerminalNode
        ├── terminal = true
        ├── terminalStatus
        └── exitCode
```

### Pipeline Result v2

```
PipelineResult
├── type: "pipeline_result"
├── version: "v2"
├── status, is_error
├── entry_node, terminal_node, terminal_status, exit_code
├── iterations, node_run_count, failed_node_count
└── node_runs[]
```

### Pipeline Events v2

| Event | Key Fields |
|-------|------------|
| `plan_start` | version, entry_node, node_count |
| `node_start` | node_id, node_run_id, kind, model/cmd, attempt, iteration |
| `node_session_bind` | node_id, node_run_id, session_id |
| `node_timeout` | node_id, node_run_id, idle_timeout_sec, reason |
| `node_finish` | full node run payload |
| `transition_taken` | from_node, to_node, when, node_run_id |
| `plan_finish` | status, iterations, node_run_count, failed_node_count, terminal fields, exit_code |

## Storage Layout

```
<project>/.agent-cli/
├── config.toml
└── runs/
    └── <YYYYMMDDTHHMMSS>-<hex_id>/
        ├── stats.json
        ├── output.ndjson
        └── output.log
```
