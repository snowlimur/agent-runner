package result

import "testing"

func TestParseAgentResultValid(t *testing.T) {
	t.Parallel()

	input := `{"type":"result","subtype":"success","is_error":false,"duration_ms":10,"duration_api_ms":20,"num_turns":2,"result":"ok","stop_reason":null,"session_id":"s1","total_cost_usd":0.5,"usage":{"input_tokens":10,"cache_creation_input_tokens":3,"cache_read_input_tokens":4,"output_tokens":5,"server_tool_use":{"web_search_requests":1,"web_fetch_requests":0},"service_tier":"standard"},"modelUsage":{"claude-sonnet":{"inputTokens":10,"outputTokens":5,"cacheReadInputTokens":4,"cacheCreationInputTokens":3,"webSearchRequests":1,"costUSD":0.5}},"uuid":"u1"}`

	parsed, err := ParseAgentResult(input)
	if err != nil {
		t.Fatalf("parse result: %v", err)
	}

	if parsed.Agent.SessionID != "s1" {
		t.Fatalf("unexpected session id: %s", parsed.Agent.SessionID)
	}
	if parsed.Metrics.DurationMS != 10 {
		t.Fatalf("unexpected duration: %d", parsed.Metrics.DurationMS)
	}
	if parsed.Metrics.ByModel["claude-sonnet"].CostUSD != 0.5 {
		t.Fatalf("unexpected model cost: %.2f", parsed.Metrics.ByModel["claude-sonnet"].CostUSD)
	}
}

func TestParseAgentResultInvalid(t *testing.T) {
	t.Parallel()

	_, err := ParseAgentResult("not-json")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseAgentResultIsError(t *testing.T) {
	t.Parallel()

	input := `{"type":"result","subtype":"error","is_error":true,"duration_ms":1,"duration_api_ms":1,"num_turns":1,"result":"fail","stop_reason":null,"session_id":"s1","total_cost_usd":0.0,"usage":{"input_tokens":0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":0,"server_tool_use":{"web_search_requests":0,"web_fetch_requests":0},"service_tier":"standard"},"modelUsage":{},"uuid":"u1"}`

	parsed, err := ParseAgentResult(input)
	if err != nil {
		t.Fatalf("parse result: %v", err)
	}
	if !parsed.Agent.IsError {
		t.Fatal("expected is_error=true")
	}
}
