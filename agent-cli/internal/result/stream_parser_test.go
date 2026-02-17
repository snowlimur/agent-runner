package result

import "testing"

func TestParseStreamLineSystemInit(t *testing.T) {
	t.Parallel()

	line := `{"type":"system","subtype":"init","session_id":"s1","model":"claude-sonnet"}`
	event, kind, err := ParseStreamLine(line)
	if err != nil {
		t.Fatalf("parse line: %v", err)
	}
	if kind != StreamLineJSONEvent {
		t.Fatalf("unexpected kind: %s", kind)
	}
	if event == nil || event.System == nil {
		t.Fatal("expected system event")
	}
	if event.System.SessionID != "s1" {
		t.Fatalf("unexpected session id: %s", event.System.SessionID)
	}
}

func TestParseStreamLineAssistantToolUse(t *testing.T) {
	t.Parallel()

	line := `{"type":"assistant","session_id":"s1","message":{"id":"m1","content":[{"type":"tool_use","id":"tool1","name":"Bash","input":{"command":"go version","description":"Print Go version"}}]}}`
	event, kind, err := ParseStreamLine(line)
	if err != nil {
		t.Fatalf("parse line: %v", err)
	}
	if kind != StreamLineJSONEvent {
		t.Fatalf("unexpected kind: %s", kind)
	}
	if event.Assistant == nil || len(event.Assistant.Content) != 1 {
		t.Fatalf("unexpected assistant content: %+v", event.Assistant)
	}
	toolUse := event.Assistant.Content[0].ToolUse
	if toolUse == nil {
		t.Fatal("expected tool use content")
	}
	if toolUse.Name != "Bash" || toolUse.Input.Command != "go version" {
		t.Fatalf("unexpected tool use: %+v", toolUse)
	}
}

func TestParseStreamLineUserToolResult(t *testing.T) {
	t.Parallel()

	line := `{"type":"user","session_id":"s1","message":{"content":[{"tool_use_id":"tool1","type":"tool_result","content":"ok","is_error":false}]},"tool_use_result":{"stdout":"ok","stderr":"","interrupted":false,"isImage":false,"noOutputExpected":false}}`
	event, kind, err := ParseStreamLine(line)
	if err != nil {
		t.Fatalf("parse line: %v", err)
	}
	if kind != StreamLineJSONEvent {
		t.Fatalf("unexpected kind: %s", kind)
	}
	if event.User == nil || len(event.User.ToolResults) != 1 {
		t.Fatalf("unexpected user event: %+v", event.User)
	}
	if event.User.ToolResults[0].ToolUseID != "tool1" {
		t.Fatalf("unexpected tool_use_id: %s", event.User.ToolResults[0].ToolUseID)
	}
}

func TestParseStreamLineFinalResult(t *testing.T) {
	t.Parallel()

	line := `{"type":"result","subtype":"success","is_error":false,"duration_ms":1,"duration_api_ms":2,"num_turns":3,"result":"done","stop_reason":null,"session_id":"s1","total_cost_usd":0.1,"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1,"server_tool_use":{"web_search_requests":0,"web_fetch_requests":0},"service_tier":"standard"},"modelUsage":{},"uuid":"u1"}`
	event, kind, err := ParseStreamLine(line)
	if err != nil {
		t.Fatalf("parse line: %v", err)
	}
	if kind != StreamLineJSONEvent {
		t.Fatalf("unexpected kind: %s", kind)
	}
	if event.Result == nil {
		t.Fatal("expected final result event")
	}
	if event.Result.SessionID != "s1" {
		t.Fatalf("unexpected session id: %s", event.Result.SessionID)
	}
}

func TestParseStreamLinePipelineEvent(t *testing.T) {
	t.Parallel()

	line := `{"type":"pipeline_event","event":"task_finish","version":"v1","stage_id":"main","task_id":"build","session_id":"s1","status":"error","mode":"parallel","model":"opus","verbosity":"vv","workspace":"shared","on_error":"fail_fast","prompt_source":"prompt","prompt_file":"","exit_code":124,"signal":"","started_at":"2026-02-17T17:25:31Z","finished_at":"2026-02-17T17:55:31Z","duration_ms":1800000,"error_message":"idle timeout","stage_count":2,"completed_stages":1,"task_count":3,"failed_task_count":1,"completed_tasks":2,"failed_tasks":1,"max_parallel":2,"task_idle_timeout_sec":30,"idle_timeout_sec":30,"reason":"idle timeout after 30 seconds without task output"}`
	event, kind, err := ParseStreamLine(line)
	if err != nil {
		t.Fatalf("parse line: %v", err)
	}
	if kind != StreamLineJSONEvent {
		t.Fatalf("unexpected kind: %s", kind)
	}
	if event.Pipeline == nil {
		t.Fatal("expected pipeline event")
	}
	if event.Pipeline.Event != "task_finish" {
		t.Fatalf("unexpected pipeline event type: %s", event.Pipeline.Event)
	}
	if event.Pipeline.StageID != "main" || event.Pipeline.TaskID != "build" {
		t.Fatalf("unexpected pipeline task ref: %+v", event.Pipeline)
	}
	if event.Pipeline.Version != "v1" {
		t.Fatalf("unexpected version: %s", event.Pipeline.Version)
	}
	if event.Pipeline.SessionID != "s1" {
		t.Fatalf("unexpected session id: %s", event.Pipeline.SessionID)
	}
	if event.Pipeline.Status != "error" {
		t.Fatalf("unexpected status: %s", event.Pipeline.Status)
	}
	if event.Pipeline.Mode != "parallel" {
		t.Fatalf("unexpected mode: %s", event.Pipeline.Mode)
	}
	if event.Pipeline.Model != "opus" {
		t.Fatalf("unexpected model: %s", event.Pipeline.Model)
	}
	if event.Pipeline.Verbosity != "vv" {
		t.Fatalf("unexpected verbosity: %s", event.Pipeline.Verbosity)
	}
	if event.Pipeline.Workspace != "shared" {
		t.Fatalf("unexpected workspace: %s", event.Pipeline.Workspace)
	}
	if event.Pipeline.OnError != "fail_fast" {
		t.Fatalf("unexpected on_error: %s", event.Pipeline.OnError)
	}
	if event.Pipeline.PromptSource != "prompt" {
		t.Fatalf("unexpected prompt_source: %s", event.Pipeline.PromptSource)
	}
	if event.Pipeline.ExitCode != 124 {
		t.Fatalf("unexpected exit code: %d", event.Pipeline.ExitCode)
	}
	if event.Pipeline.DurationMS != 1800000 {
		t.Fatalf("unexpected duration: %d", event.Pipeline.DurationMS)
	}
	if event.Pipeline.StageCount != 2 {
		t.Fatalf("unexpected stage_count: %d", event.Pipeline.StageCount)
	}
	if event.Pipeline.CompletedStages != 1 {
		t.Fatalf("unexpected completed_stages: %d", event.Pipeline.CompletedStages)
	}
	if event.Pipeline.TaskCount != 3 {
		t.Fatalf("unexpected task_count: %d", event.Pipeline.TaskCount)
	}
	if event.Pipeline.FailedTaskCount != 1 {
		t.Fatalf("unexpected failed_task_count: %d", event.Pipeline.FailedTaskCount)
	}
	if event.Pipeline.CompletedTasks != 2 {
		t.Fatalf("unexpected completed_tasks: %d", event.Pipeline.CompletedTasks)
	}
	if event.Pipeline.FailedTasks != 1 {
		t.Fatalf("unexpected failed_tasks: %d", event.Pipeline.FailedTasks)
	}
	if event.Pipeline.MaxParallel != 2 {
		t.Fatalf("unexpected max_parallel: %d", event.Pipeline.MaxParallel)
	}
	if event.Pipeline.TaskIdleTimeoutSec != 30 {
		t.Fatalf("unexpected task_idle_timeout_sec: %d", event.Pipeline.TaskIdleTimeoutSec)
	}
	if event.Pipeline.ErrorMessage != "idle timeout" {
		t.Fatalf("unexpected error message: %s", event.Pipeline.ErrorMessage)
	}
	if event.Pipeline.IdleTimeoutSec != 30 {
		t.Fatalf("unexpected idle timeout sec: %d", event.Pipeline.IdleTimeoutSec)
	}
	if event.Pipeline.Reason == "" {
		t.Fatal("expected reason")
	}
}

func TestParseStreamLineNonJSON(t *testing.T) {
	t.Parallel()

	event, kind, err := ParseStreamLine("task: [go] docker compose run")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kind != StreamLineNonJSON {
		t.Fatalf("unexpected kind: %s", kind)
	}
	if event != nil {
		t.Fatalf("expected nil event, got %+v", event)
	}
}

func TestParseStreamLineInvalidJSON(t *testing.T) {
	t.Parallel()

	event, kind, err := ParseStreamLine("{not-json")
	if err == nil {
		t.Fatal("expected error")
	}
	if kind != StreamLineInvalidJSON {
		t.Fatalf("unexpected kind: %s", kind)
	}
	if event != nil {
		t.Fatalf("expected nil event, got %+v", event)
	}
}

func TestExtractFinalResultFromStream(t *testing.T) {
	t.Parallel()

	lines := []string{
		"task: [go] docker compose run",
		`{"type":"system","subtype":"init","session_id":"s1","model":"claude-sonnet"}`,
		`{"type":"result","subtype":"success","is_error":false,"duration_ms":10,"duration_api_ms":20,"num_turns":2,"result":"ok","stop_reason":null,"session_id":"s1","total_cost_usd":0.5,"usage":{"input_tokens":10,"cache_creation_input_tokens":3,"cache_read_input_tokens":4,"output_tokens":5,"server_tool_use":{"web_search_requests":1,"web_fetch_requests":0},"service_tier":"standard"},"modelUsage":{"claude-sonnet":{"inputTokens":10,"outputTokens":5,"cacheReadInputTokens":4,"cacheCreationInputTokens":3,"webSearchRequests":1,"costUSD":0.5}},"uuid":"u1"}`,
	}

	parsed, err := ExtractFinalResultFromStream(lines)
	if err != nil {
		t.Fatalf("extract final result: %v", err)
	}
	if parsed.Agent.SessionID != "s1" {
		t.Fatalf("unexpected session id: %s", parsed.Agent.SessionID)
	}
}

func TestExtractFinalResultFromStreamFallbackSingleJSON(t *testing.T) {
	t.Parallel()

	lines := []string{
		`{"type":"result","subtype":"success","is_error":false,"duration_ms":1,"duration_api_ms":1,"num_turns":1,"result":"ok","stop_reason":null,"session_id":"s1","total_cost_usd":0.1,"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1,"server_tool_use":{"web_search_requests":0,"web_fetch_requests":0},"service_tier":"standard"},"modelUsage":{},"uuid":"u1"}`,
	}

	parsed, err := ExtractFinalResultFromStream(lines)
	if err != nil {
		t.Fatalf("extract final result: %v", err)
	}
	if parsed.Agent.Subtype != "success" {
		t.Fatalf("unexpected subtype: %s", parsed.Agent.Subtype)
	}
}

func TestExtractFinalResultFromStreamMissingResult(t *testing.T) {
	t.Parallel()

	_, err := ExtractFinalResultFromStream([]string{"hello", "{\"type\":\"assistant\"}"})
	if err == nil {
		t.Fatal("expected error")
	}
}
