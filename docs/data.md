# Data Codemap

Updated: 2026-02-19

## Go Types

### RunRecord (`stats/types.go`)

```
RunRecord
├── RunID              string
├── Timestamp          time.Time
├── Status             RunStatus (success | error | parse_error | exec_error)
├── DockerExitCode     int
├── CWD                string
├── Pipeline           *PipelineRunRecord (pipeline mode only)
│   ├── Version, Status, IsError
│   ├── EntryNode, TerminalNode, TerminalStatus, ExitCode
│   ├── Iterations, NodeRunCount, FailedNodeCount
│   └── NodeRuns[]     PipelineNodeRunRecord
│       ├── NodeID, NodeRunID, Kind, Status
│       ├── Model, PromptSource, PromptFile, Cmd, CWD
│       ├── ExitCode, Signal, TimedOut, StartedAt, FinishedAt, DurationMS
│       ├── ErrorMessage
│       └── Normalized *PipelineNodeRunNormalized
│           ├── InputTokens, CacheCreationInputTokens, CacheReadInputTokens, OutputTokens
│           ├── CostUSD, WebSearchRequests
│           └── ByModel map[string]PipelineNodeRunModelMetric
├── AgentResult        *result.AgentResult (single-prompt mode only)
├── Normalized         result.NormalizedMetrics
├── ErrorType          string
└── ErrorMessage       string
```

### Stream Events (`result/stream_parser.go`)

```
StreamEvent
├── Type       string
├── System     *SystemEvent    (subtype, session_id, model)
├── Assistant  *AssistantEvent (message_id, session_id, content[])
├── User       *UserEvent      (session_id, tool_results[], tool_use_result)
├── Pipeline   *PipelineEvent  (event, node_id, node_run_id, session_id, ...)
└── Result     *AgentResult    (final Claude result)
```

**`PipelineEvent.event` values:** `plan_start`, `node_start`, `node_session_bind`, `node_timeout`, `node_finish`, `transition_taken`, `plan_finish`

## TypeScript Types (`images/entrypoint/src/lib/types.ts`)

### Pipeline Plan v2

```
PipelinePlan
├── version: "v2"
├── entryNode
├── defaults: { model, agentIdleTimeoutSec, commandTimeoutSec }
├── limits: { maxIterations, maxSameNodeHits }
├── nodeOrder: string[]               // user-declared order only (implicit built-ins excluded)
└── nodes: Record<string, PipelineNode>
    ├── PipelineExecutableNode
    │   ├── run: PipelineAgentRun | PipelineCommandRun
    │   └── transitions[]: { when, to }
    └── PipelineTerminalNode
        ├── terminal: true
        ├── terminalStatus: success | blocked | failed | canceled
        ├── exitCode: 0..255
        └── message: string
```

Resolved plans always contain terminal nodes `success` and `fail`. If they are not defined in YAML, parser injects defaults; if defined, they must remain terminal nodes.

### PipelineAgentRun

```
PipelineAgentRun
├── kind: "agent"
├── model: sonnet | opus
├── promptText: string  (resolved, template-substituted)
├── promptFile: PromptFileRef | null
├── idleTimeoutSec: number
└── decision: { schemaFile: PromptFileRef, schema: JSONObject }
```

### Pipeline Events (stdout protocol)

| Event | Key Fields |
|-------|------------|
| `plan_start` | version, entry_node, node_count, started_at |
| `node_start` | node_id, node_run_id, kind, model, cmd, iteration, attempt, idle_timeout_sec, timeout_sec |
| `node_session_bind` | node_id, node_run_id, session_id |
| `node_timeout` | node_id, node_run_id, idle_timeout_sec, reason |
| `node_finish` | full PipelineNodeRunRecord |
| `transition_taken` | from_node, to_node, when, node_run_id, iteration |
| `plan_finish` | status, iterations, node_run_count, failed_node_count, terminal fields, exit_code |

### PipelineResult (final stdout JSON)

```
pipeline_result
├── type: "pipeline_result"
├── version: "v2"
├── status, is_error
├── entry_node, terminal_node, terminal_status, exit_code
├── iterations, node_run_count, failed_node_count
└── node_runs[]: PipelineNodeRunRecord
```

### PipelineConditionScope (transition evaluation context)

```
{
  decision: JSONObject,          // structured_output from agent
  run: { exit_code, signal, timed_out, status },
  node: { id, kind, attempt, run_id },
  pipeline: { iteration, total_node_runs }
}
```

## Config Schema (`.agent-cli/config.toml`)

```toml
[docker]
image = "claude:go"
model = "opus"                      # sonnet | opus (default: opus)
mode = "none"                       # none | dind | dood (default: none)
dind_storage_driver = "overlay2"    # overlay2 | vfs
run_idle_timeout_sec = 7200
pipeline_task_idle_timeout_sec = 1800

[auth]
github_token = "ghp_..."
claude_token = "sk-..."

[workspace]
source_workspace_dir = "/absolute/path/to/source"

[git]
user_name = "Your Name"
user_email = "you@example.com"
```

## Storage Layout

```
<project>/.agent-cli/
├── config.toml
└── runs/
    └── <YYYYMMDDTHHMMSS>-<hex_id>/
        ├── stats.json       # RunRecord (JSON)
        ├── output.ndjson    # JSON object lines from stdout
        └── output.log       # Non-JSON lines (stdout then stderr)
```
