package stats

import (
	"time"

	"agent-cli/internal/result"
)

type RunStatus string

const (
	RunStatusSuccess    RunStatus = "success"
	RunStatusError      RunStatus = "error"
	RunStatusParseError RunStatus = "parse_error"
	RunStatusExecError  RunStatus = "exec_error"
)

type RunRecord struct {
	RunID          string                   `json:"run_id"`
	Timestamp      time.Time                `json:"timestamp"`
	Status         RunStatus                `json:"status"`
	DockerExitCode int                      `json:"docker_exit_code"`
	CWD            string                   `json:"cwd"`
	Pipeline       *PipelineRunRecord       `json:"pipeline,omitempty"`
	AgentResult    *result.AgentResult      `json:"agent_result,omitempty"`
	Normalized     result.NormalizedMetrics `json:"normalized"`
	ErrorType      string                   `json:"error_type,omitempty"`
	ErrorMessage   string                   `json:"error_message,omitempty"`
}

type PipelineRunRecord struct {
	Version         string                  `json:"version"`
	Status          string                  `json:"status"`
	IsError         bool                    `json:"is_error"`
	EntryNode       string                  `json:"entry_node"`
	TerminalNode    string                  `json:"terminal_node,omitempty"`
	TerminalStatus  string                  `json:"terminal_status,omitempty"`
	ExitCode        int                     `json:"exit_code"`
	Iterations      int                     `json:"iterations"`
	NodeRunCount    int                     `json:"node_run_count"`
	FailedNodeCount int                     `json:"failed_node_count"`
	NodeRuns        []PipelineNodeRunRecord `json:"node_runs"`
}

type PipelineNodeRunRecord struct {
	NodeID       string                     `json:"node_id"`
	NodeRunID    string                     `json:"node_run_id"`
	Kind         string                     `json:"kind"`
	Status       string                     `json:"status"`
	Model        string                     `json:"model"`
	PromptSource string                     `json:"prompt_source"`
	PromptFile   string                     `json:"prompt_file,omitempty"`
	Cmd          string                     `json:"cmd,omitempty"`
	CWD          string                     `json:"cwd,omitempty"`
	ExitCode     int                        `json:"exit_code"`
	Signal       string                     `json:"signal,omitempty"`
	TimedOut     bool                       `json:"timed_out,omitempty"`
	StartedAt    time.Time                  `json:"started_at"`
	FinishedAt   time.Time                  `json:"finished_at"`
	DurationMS   int64                      `json:"duration_ms"`
	Normalized   *PipelineNodeRunNormalized `json:"normalized,omitempty"`
	ErrorMessage string                     `json:"error_message,omitempty"`
}

type PipelineNodeRunNormalized struct {
	InputTokens              int64                                 `json:"input_tokens"`
	CacheCreationInputTokens int64                                 `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64                                 `json:"cache_read_input_tokens"`
	OutputTokens             int64                                 `json:"output_tokens"`
	CostUSD                  float64                               `json:"cost_usd"`
	WebSearchRequests        int64                                 `json:"web_search_requests"`
	ByModel                  map[string]PipelineNodeRunModelMetric `json:"by_model"`
}

type PipelineNodeRunModelMetric struct {
	InputTokens              int64   `json:"input_tokens"`
	OutputTokens             int64   `json:"output_tokens"`
	CacheReadInputTokens     int64   `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int64   `json:"cache_creation_input_tokens"`
	CostUSD                  float64 `json:"cost_usd"`
	WebSearchRequests        int64   `json:"web_search_requests"`
}

type Aggregate struct {
	TotalRuns      int                       `json:"total_runs"`
	SuccessRuns    int                       `json:"success_runs"`
	ErrorRuns      int                       `json:"error_runs"`
	ParseErrorRuns int                       `json:"parse_error_runs"`
	FirstRunAt     *time.Time                `json:"first_run_at,omitempty"`
	LastRunAt      *time.Time                `json:"last_run_at,omitempty"`
	Sums           AggregateMetrics          `json:"sums"`
	ByModel        map[string]ModelAggregate `json:"by_model"`
	SkippedFiles   []string                  `json:"skipped_files"`
}

type AggregateMetrics struct {
	DurationMS               int64   `json:"duration_ms"`
	DurationAPIMS            int64   `json:"duration_api_ms"`
	NumTurns                 int64   `json:"num_turns"`
	TotalCostUSD             float64 `json:"total_cost_usd"`
	InputTokens              int64   `json:"input_tokens"`
	CacheCreationInputTokens int64   `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64   `json:"cache_read_input_tokens"`
	OutputTokens             int64   `json:"output_tokens"`
}

type ModelAggregate struct {
	InputTokens              int64   `json:"input_tokens"`
	OutputTokens             int64   `json:"output_tokens"`
	CacheReadInputTokens     int64   `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int64   `json:"cache_creation_input_tokens"`
	WebSearchRequests        int64   `json:"web_search_requests"`
	CostUSD                  float64 `json:"cost_usd"`
}
