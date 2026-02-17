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
	Version         string               `json:"version"`
	Status          string               `json:"status"`
	IsError         bool                 `json:"is_error"`
	StageCount      int                  `json:"stage_count"`
	CompletedStages int                  `json:"completed_stages"`
	TaskCount       int                  `json:"task_count"`
	FailedTaskCount int                  `json:"failed_task_count"`
	Tasks           []PipelineTaskRecord `json:"tasks"`
}

type PipelineTaskRecord struct {
	StageID      string                  `json:"stage_id"`
	TaskID       string                  `json:"task_id"`
	Status       string                  `json:"status"`
	OnError      string                  `json:"on_error"`
	Workspace    string                  `json:"workspace"`
	Model        string                  `json:"model"`
	Verbosity    string                  `json:"verbosity"`
	PromptSource string                  `json:"prompt_source"`
	PromptFile   string                  `json:"prompt_file,omitempty"`
	ExitCode     int                     `json:"exit_code"`
	Signal       string                  `json:"signal,omitempty"`
	StartedAt    time.Time               `json:"started_at"`
	FinishedAt   time.Time               `json:"finished_at"`
	DurationMS   int64                   `json:"duration_ms"`
	Normalized   *PipelineTaskNormalized `json:"normalized,omitempty"`
	ErrorMessage string                  `json:"error_message,omitempty"`
}

type PipelineTaskNormalized struct {
	InputTokens              int64                              `json:"input_tokens"`
	CacheCreationInputTokens int64                              `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64                              `json:"cache_read_input_tokens"`
	OutputTokens             int64                              `json:"output_tokens"`
	CostUSD                  float64                            `json:"cost_usd"`
	WebSearchRequests        int64                              `json:"web_search_requests"`
	ByModel                  map[string]PipelineTaskModelMetric `json:"by_model"`
}

type PipelineTaskModelMetric struct {
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
