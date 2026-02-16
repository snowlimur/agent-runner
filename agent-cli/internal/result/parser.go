package result

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type AgentResult struct {
	Type          string                `json:"type"`
	Subtype       string                `json:"subtype"`
	IsError       bool                  `json:"is_error"`
	DurationMS    int64                 `json:"duration_ms"`
	DurationAPIMS int64                 `json:"duration_api_ms"`
	NumTurns      int64                 `json:"num_turns"`
	Result        string                `json:"result"`
	StopReason    any                   `json:"stop_reason"`
	SessionID     string                `json:"session_id"`
	TotalCostUSD  float64               `json:"total_cost_usd"`
	Usage         Usage                 `json:"usage"`
	ModelUsage    map[string]ModelUsage `json:"modelUsage"`
	UUID          string                `json:"uuid"`
}

type Usage struct {
	InputTokens              int64       `json:"input_tokens"`
	CacheCreationInputTokens int64       `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64       `json:"cache_read_input_tokens"`
	OutputTokens             int64       `json:"output_tokens"`
	ServerToolUse            ServerTools `json:"server_tool_use"`
	ServiceTier              string      `json:"service_tier"`
}

type ServerTools struct {
	WebSearchRequests int64 `json:"web_search_requests"`
	WebFetchRequests  int64 `json:"web_fetch_requests"`
}

type ModelUsage struct {
	InputTokens              int64   `json:"inputTokens"`
	OutputTokens             int64   `json:"outputTokens"`
	CacheReadInputTokens     int64   `json:"cacheReadInputTokens"`
	CacheCreationInputTokens int64   `json:"cacheCreationInputTokens"`
	WebSearchRequests        int64   `json:"webSearchRequests"`
	CostUSD                  float64 `json:"costUSD"`
}

type ParsedResult struct {
	Raw     json.RawMessage
	Agent   AgentResult
	Metrics NormalizedMetrics
}

type NormalizedMetrics struct {
	DurationMS               int64                  `json:"duration_ms"`
	DurationAPIMS            int64                  `json:"duration_api_ms"`
	NumTurns                 int64                  `json:"num_turns"`
	TotalCostUSD             float64                `json:"total_cost_usd"`
	InputTokens              int64                  `json:"input_tokens"`
	CacheCreationInputTokens int64                  `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64                  `json:"cache_read_input_tokens"`
	OutputTokens             int64                  `json:"output_tokens"`
	ByModel                  map[string]ModelMetric `json:"by_model"`
}

type ModelMetric struct {
	InputTokens              int64   `json:"input_tokens"`
	OutputTokens             int64   `json:"output_tokens"`
	CacheReadInputTokens     int64   `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int64   `json:"cache_creation_input_tokens"`
	WebSearchRequests        int64   `json:"web_search_requests"`
	CostUSD                  float64 `json:"cost_usd"`
}

func ParseAgentResult(rawOutput string) (*ParsedResult, error) {
	trimmed := strings.TrimSpace(rawOutput)
	if trimmed == "" {
		return nil, errors.New("agent output is empty")
	}

	var agent AgentResult
	if err := json.Unmarshal([]byte(trimmed), &agent); err != nil {
		return nil, fmt.Errorf("decode agent result JSON: %w", err)
	}
	if strings.TrimSpace(agent.Type) != "result" {
		return nil, errors.New("JSON is not a final result event")
	}

	metrics := ExtractMetrics(agent)

	return &ParsedResult{
		Raw:     json.RawMessage([]byte(trimmed)),
		Agent:   agent,
		Metrics: metrics,
	}, nil
}

func ExtractMetrics(agent AgentResult) NormalizedMetrics {
	byModel := make(map[string]ModelMetric, len(agent.ModelUsage))
	for name, usage := range agent.ModelUsage {
		byModel[name] = ModelMetric{
			InputTokens:              usage.InputTokens,
			OutputTokens:             usage.OutputTokens,
			CacheReadInputTokens:     usage.CacheReadInputTokens,
			CacheCreationInputTokens: usage.CacheCreationInputTokens,
			WebSearchRequests:        usage.WebSearchRequests,
			CostUSD:                  usage.CostUSD,
		}
	}

	return NormalizedMetrics{
		DurationMS:               agent.DurationMS,
		DurationAPIMS:            agent.DurationAPIMS,
		NumTurns:                 agent.NumTurns,
		TotalCostUSD:             agent.TotalCostUSD,
		InputTokens:              agent.Usage.InputTokens,
		CacheCreationInputTokens: agent.Usage.CacheCreationInputTokens,
		CacheReadInputTokens:     agent.Usage.CacheReadInputTokens,
		OutputTokens:             agent.Usage.OutputTokens,
		ByModel:                  byModel,
	}
}
