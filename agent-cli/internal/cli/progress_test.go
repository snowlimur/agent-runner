package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"agent-cli/internal/result"
)

func TestProgressPrinterInitAndToolLifecycle(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	printer := NewProgressPrinter(&out)
	setSteppedClock(printer, time.Unix(0, 0), 2*time.Second)

	initLine := `{"type":"system","subtype":"init","session_id":"s1","model":"claude-sonnet"}`
	assistantLine := `{"type":"assistant","session_id":"s1","message":{"id":"m1","content":[{"type":"tool_use","id":"tool1","name":"Bash","input":{"command":"go version","description":"Print Go version"}}]}}`
	userLine := `{"type":"user","session_id":"s1","message":{"content":[{"tool_use_id":"tool1","type":"tool_result","content":"go version go1.26.0","is_error":false}]},"tool_use_result":{"stdout":"go version go1.26.0 linux/arm64","stderr":"","interrupted":false,"isImage":false,"noOutputExpected":false}}`
	resultLine := `{"type":"result","subtype":"success","is_error":false,"duration_ms":10,"duration_api_ms":12,"num_turns":2,"result":"Build completed","stop_reason":null,"session_id":"s1","total_cost_usd":0.1,"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1,"server_tool_use":{"web_search_requests":0,"web_fetch_requests":0},"service_tier":"standard"},"modelUsage":{},"uuid":"u1"}`

	for _, line := range []string{initLine, assistantLine, userLine, resultLine} {
		event, kind, err := result.ParseStreamLine(line)
		if err != nil {
			t.Fatalf("parse stream line: %v", err)
		}
		if kind != result.StreamLineJSONEvent {
			t.Fatalf("unexpected stream kind: %s", kind)
		}
		printer.HandleEvent(event)
	}

	output := out.String()
	assertContains(t, output, "[init] session=s1 model=claude-sonnet")
	assertContains(t, output, "[Bash#tool1:start] go version")
	assertContains(t, output, "[Bash#tool1:done] ok | 2s")
	assertContains(t, output, "[Bash#tool1:output] go version go1.26.0 linux/arm64")
	assertContains(t, output, "[result] Build completed")
}

func TestProgressPrinterTodoTransitions(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	printer := NewProgressPrinter(&out)
	setSteppedClock(printer, time.Unix(0, 0), 2*time.Second)

	firstTodoLine := `{"type":"user","session_id":"s1","message":{"content":[{"tool_use_id":"todo1","type":"tool_result","content":"ok","is_error":false}]},"tool_use_result":{"oldTodos":[],"newTodos":[{"content":"Build project","status":"in_progress","activeForm":"Building project"},{"content":"Run tests","status":"pending","activeForm":"Running tests"}]}}`
	secondTodoLine := `{"type":"user","session_id":"s1","message":{"content":[{"tool_use_id":"todo2","type":"tool_result","content":"ok","is_error":false}]},"tool_use_result":{"oldTodos":[{"content":"Build project","status":"in_progress","activeForm":"Building project"},{"content":"Run tests","status":"pending","activeForm":"Running tests"}],"newTodos":[{"content":"Build project","status":"completed","activeForm":"Building project"},{"content":"Run tests","status":"in_progress","activeForm":"Running tests"}]}}`

	for _, line := range []string{firstTodoLine, secondTodoLine} {
		event, kind, err := result.ParseStreamLine(line)
		if err != nil {
			t.Fatalf("parse stream line: %v", err)
		}
		if kind != result.StreamLineJSONEvent {
			t.Fatalf("unexpected stream kind: %s", kind)
		}
		printer.HandleEvent(event)
	}

	output := out.String()
	assertContains(t, output, "[todo] Build project: none -> in_progress")
	assertContains(t, output, "[todo] Build project: in_progress -> completed")
	assertContains(t, output, "[todo] Run tests: pending -> in_progress")
}

func TestProgressPrinterErrorSnippet(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	printer := NewProgressPrinter(&out)
	setSteppedClock(printer, time.Unix(0, 0), 2*time.Second)

	assistantLine := `{"type":"assistant","session_id":"s1","message":{"id":"m1","content":[{"type":"tool_use","id":"tool-err","name":"Bash","input":{"command":"go build"}}]}}`
	userLine := `{"type":"user","session_id":"s1","message":{"content":[{"tool_use_id":"tool-err","type":"tool_result","content":"failed","is_error":true}]},"tool_use_result":{"stdout":"","stderr":"line1\nline2\nline3","interrupted":false,"isImage":false,"noOutputExpected":false}}`

	for _, line := range []string{assistantLine, userLine} {
		event, kind, err := result.ParseStreamLine(line)
		if err != nil {
			t.Fatalf("parse stream line: %v", err)
		}
		if kind != result.StreamLineJSONEvent {
			t.Fatalf("unexpected stream kind: %s", kind)
		}
		printer.HandleEvent(event)
	}

	output := out.String()
	assertContains(t, output, "[Bash#tool-err:done] error | 2s")
	assertContains(t, output, "[Bash#tool-err:output] line1")
	assertContains(t, output, "[Bash#tool-err:output] line2")
	assertContains(t, output, "[Bash#tool-err:output] line3")
}

func TestProgressPrinterReadDoneWithoutContent(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	printer := NewProgressPrinter(&out)
	setSteppedClock(printer, time.Unix(0, 0), 2*time.Second)

	assistantLine := `{"type":"assistant","session_id":"s1","message":{"id":"m1","content":[{"type":"tool_use","id":"tool-read","name":"Read","input":{"description":"Read file"}}]}}`
	userLine := `{"type":"user","session_id":"s1","message":{"content":[{"tool_use_id":"tool-read","type":"tool_result","content":"1→module example.com/hello-go\n2→go 1.26","is_error":false}]},"tool_use_result":{"stdout":"","stderr":"","interrupted":false,"isImage":false,"noOutputExpected":false}}`

	for _, line := range []string{assistantLine, userLine} {
		event, kind, err := result.ParseStreamLine(line)
		if err != nil {
			t.Fatalf("parse stream line: %v", err)
		}
		if kind != result.StreamLineJSONEvent {
			t.Fatalf("unexpected stream kind: %s", kind)
		}
		printer.HandleEvent(event)
	}

	output := out.String()
	assertContains(t, output, "[Read#tool-read:done] ok | 2s")
	assertNotContains(t, output, "module example.com/hello-go")
}

func TestProgressPrinterSkipsTodoWriteLines(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	printer := NewProgressPrinter(&out)
	setSteppedClock(printer, time.Unix(0, 0), 2*time.Second)

	assistantLine := `{"type":"assistant","session_id":"s1","message":{"id":"m1","content":[{"type":"tool_use","id":"tool-todo","name":"TodoWrite","input":{"todos":[{"content":"Task A","status":"in_progress","activeForm":"Doing task A"}]}}]}}`
	userLine := `{"type":"user","session_id":"s1","message":{"content":[{"tool_use_id":"tool-todo","type":"tool_result","content":"Todos have been modified successfully","is_error":false}]},"tool_use_result":{"oldTodos":[{"content":"Task A","status":"pending","activeForm":"Doing task A"}],"newTodos":[{"content":"Task A","status":"completed","activeForm":"Doing task A"}]}}`

	for _, line := range []string{assistantLine, userLine} {
		event, kind, err := result.ParseStreamLine(line)
		if err != nil {
			t.Fatalf("parse stream line: %v", err)
		}
		if kind != result.StreamLineJSONEvent {
			t.Fatalf("unexpected stream kind: %s", kind)
		}
		printer.HandleEvent(event)
	}

	output := out.String()
	assertNotContains(t, output, "[TodoWrite#tool-todo:start]")
	assertNotContains(t, output, "[TodoWrite#tool-todo:done]")
	assertNotContains(t, output, "[todo] Task A")
}

func TestProgressPrinterPipelineTaskBinding(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	printer := NewProgressPrinter(&out)
	setSteppedClock(printer, time.Unix(0, 0), 1*time.Second)

	lines := []string{
		`{"type":"pipeline_event","event":"task_session_bind","stage_id":"main","task_id":"build_run","session_id":"s1"}`,
		`{"type":"system","subtype":"init","session_id":"s1","model":"claude-opus"}`,
		`{"type":"assistant","session_id":"s1","message":{"id":"m1","content":[{"type":"tool_use","id":"tool1","name":"Bash","input":{"command":"go version"}}]}}`,
		`{"type":"user","session_id":"s1","message":{"content":[{"tool_use_id":"tool1","type":"tool_result","content":"go version go1.26.0","is_error":false}]},"tool_use_result":{"stdout":"go version go1.26.0 linux/arm64","stderr":"","interrupted":false,"isImage":false,"noOutputExpected":false}}`,
		`{"type":"result","subtype":"success","is_error":false,"duration_ms":10,"duration_api_ms":12,"num_turns":2,"result":"Go version printed","stop_reason":null,"session_id":"s1","total_cost_usd":0.1,"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1,"server_tool_use":{"web_search_requests":0,"web_fetch_requests":0},"service_tier":"standard"},"modelUsage":{},"uuid":"u1"}`,
	}

	for _, line := range lines {
		event, kind, err := result.ParseStreamLine(line)
		if err != nil {
			t.Fatalf("parse stream line: %v", err)
		}
		if kind != result.StreamLineJSONEvent {
			t.Fatalf("unexpected stream kind: %s", kind)
		}
		printer.HandleEvent(event)
	}

	output := out.String()
	assertContains(t, output, "[main/build_run] [init] session=s1 model=claude-opus")
	assertContains(t, output, "[main/build_run] [Bash#tool1:start] go version")
	assertContains(t, output, "[main/build_run] [Bash#tool1:done] ok | 1s")
	assertContains(t, output, "[main/build_run] [Bash#tool1:output] go version go1.26.0 linux/arm64")
	assertContains(t, output, "[main/build_run] [result] Go version printed")
}

func TestProgressPrinterPipelineInterleavingBySession(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	printer := NewProgressPrinter(&out)
	setSteppedClock(printer, time.Unix(0, 0), 1*time.Second)

	lines := []string{
		`{"type":"pipeline_event","event":"task_session_bind","stage_id":"main","task_id":"task_a","session_id":"s1"}`,
		`{"type":"pipeline_event","event":"task_session_bind","stage_id":"main","task_id":"task_b","session_id":"s2"}`,
		`{"type":"assistant","session_id":"s1","message":{"id":"m1","content":[{"type":"tool_use","id":"tool-a","name":"Bash","input":{"command":"echo a"}}]}}`,
		`{"type":"assistant","session_id":"s2","message":{"id":"m2","content":[{"type":"tool_use","id":"tool-b","name":"Bash","input":{"command":"echo b"}}]}}`,
		`{"type":"user","session_id":"s2","message":{"content":[{"tool_use_id":"tool-b","type":"tool_result","content":"b","is_error":false}]},"tool_use_result":{"stdout":"b","stderr":"","interrupted":false,"isImage":false,"noOutputExpected":false}}`,
		`{"type":"user","session_id":"s1","message":{"content":[{"tool_use_id":"tool-a","type":"tool_result","content":"a","is_error":false}]},"tool_use_result":{"stdout":"a","stderr":"","interrupted":false,"isImage":false,"noOutputExpected":false}}`,
	}

	for _, line := range lines {
		event, kind, err := result.ParseStreamLine(line)
		if err != nil {
			t.Fatalf("parse stream line: %v", err)
		}
		if kind != result.StreamLineJSONEvent {
			t.Fatalf("unexpected stream kind: %s", kind)
		}
		printer.HandleEvent(event)
	}

	output := out.String()
	assertContains(t, output, "[main/task_a] [Bash#tool-a:start] echo a")
	assertContains(t, output, "[main/task_b] [Bash#tool-b:start] echo b")
	assertContains(t, output, "[main/task_b] [Bash#tool-b:done] ok | 1s")
	assertContains(t, output, "[main/task_a] [Bash#tool-a:done] ok | 3s")
	assertNotContains(t, output, "[unbound] [Bash#tool-a")
	assertNotContains(t, output, "[unbound] [Bash#tool-b")
}

func TestProgressPrinterPipelineTimeoutAndTaskError(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	printer := NewProgressPrinter(&out)
	setSteppedClock(printer, time.Unix(0, 0), 1*time.Second)

	lines := []string{
		`{"type":"pipeline_event","event":"task_timeout","stage_id":"main","task_id":"print_version","idle_timeout_sec":30,"reason":"idle timeout after 30 seconds without task output"}`,
		`{"type":"pipeline_event","event":"task_finish","stage_id":"main","task_id":"print_version","status":"error","error_message":"idle timeout after 30 seconds without task output"}`,
	}

	for _, line := range lines {
		event, kind, err := result.ParseStreamLine(line)
		if err != nil {
			t.Fatalf("parse stream line: %v", err)
		}
		if kind != result.StreamLineJSONEvent {
			t.Fatalf("unexpected stream kind: %s", kind)
		}
		printer.HandleEvent(event)
	}

	output := out.String()
	assertContains(t, output, "[main/print_version] [timeout] idle timeout after 30 seconds without task output")
	assertContains(t, output, "[main/print_version] [task:error] idle timeout after 30 seconds without task output")
}

func assertContains(t *testing.T, output, expected string) {
	t.Helper()
	if !strings.Contains(output, expected) {
		t.Fatalf("expected output to contain %q, got %q", expected, output)
	}
}

func assertNotContains(t *testing.T, output, expected string) {
	t.Helper()
	if strings.Contains(output, expected) {
		t.Fatalf("expected output to not contain %q, got %q", expected, output)
	}
}

func setSteppedClock(printer *ProgressPrinter, start time.Time, step time.Duration) {
	current := start.Add(-step)
	printer.now = func() time.Time {
		current = current.Add(step)
		return current
	}
}
