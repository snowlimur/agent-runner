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
	Stream         StreamMetrics            `json:"stream,omitempty"`
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
	StageID      string    `json:"stage_id"`
	TaskID       string    `json:"task_id"`
	Status       string    `json:"status"`
	OnError      string    `json:"on_error"`
	Workspace    string    `json:"workspace"`
	Model        string    `json:"model"`
	Verbosity    string    `json:"verbosity"`
	PromptSource string    `json:"prompt_source"`
	PromptFile   string    `json:"prompt_file,omitempty"`
	ExitCode     int       `json:"exit_code"`
	Signal       string    `json:"signal,omitempty"`
	StartedAt    time.Time `json:"started_at"`
	FinishedAt   time.Time `json:"finished_at"`
	DurationMS   int64     `json:"duration_ms"`
	ErrorMessage string    `json:"error_message,omitempty"`
}

type Aggregate struct {
	TotalRuns      int                       `json:"total_runs"`
	SuccessRuns    int                       `json:"success_runs"`
	ErrorRuns      int                       `json:"error_runs"`
	ParseErrorRuns int                       `json:"parse_error_runs"`
	FirstRunAt     *time.Time                `json:"first_run_at,omitempty"`
	LastRunAt      *time.Time                `json:"last_run_at,omitempty"`
	Sums           AggregateMetrics          `json:"sums"`
	StreamSums     StreamMetrics             `json:"stream_sums"`
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

type StreamMetrics struct {
	TotalJSONEvents          int64            `json:"total_json_events"`
	EventCounts              map[string]int64 `json:"event_counts"`
	NonJSONLines             int64            `json:"non_json_lines"`
	InvalidJSONLines         int64            `json:"invalid_json_lines"`
	ToolUseTotal             int64            `json:"tool_use_total"`
	ToolUseByName            map[string]int64 `json:"tool_use_by_name"`
	ToolResultTotal          int64            `json:"tool_result_total"`
	ToolResultErrorTotal     int64            `json:"tool_result_error_total"`
	UnmatchedToolUseTotal    int64            `json:"unmatched_tool_use_total"`
	UnmatchedToolResultTotal int64            `json:"unmatched_tool_result_total"`
	TodoTransitionTotal      int64            `json:"todo_transition_total"`
	TodoCompletedTotal       int64            `json:"todo_completed_transition_total"`
}

func NewStreamMetrics() StreamMetrics {
	return StreamMetrics{
		EventCounts:   map[string]int64{},
		ToolUseByName: map[string]int64{},
	}
}

func (m *StreamMetrics) EnsureMaps() {
	if m.EventCounts == nil {
		m.EventCounts = map[string]int64{}
	}
	if m.ToolUseByName == nil {
		m.ToolUseByName = map[string]int64{}
	}
}
