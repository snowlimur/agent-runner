package cli

import (
	"strings"
	"testing"

	"agent-cli/internal/result"
	"agent-cli/internal/stats"

	tea "github.com/charmbracelet/bubbletea"
)

func TestProgressTUIModelPipelineHierarchyAndTaskResult(t *testing.T) {
	t.Parallel()

	model := newProgressTUIModel(true, nil)

	lines := []string{
		`{"type":"pipeline_event","event":"plan_start","stage_count":1}`,
		`{"type":"pipeline_event","event":"stage_start","stage_id":"main","mode":"sequential","task_count":2}`,
		`{"type":"pipeline_event","event":"task_start","stage_id":"main","task_id":"task_1","model":"opus","verbosity":"vv","workspace":"shared"}`,
		`{"type":"pipeline_event","event":"task_session_bind","stage_id":"main","task_id":"task_1","session_id":"s1"}`,
		`{"type":"assistant","session_id":"s1","message":{"id":"m1","content":[{"type":"tool_use","id":"tool-a","name":"Bash","input":{"command":"go test ./..."}}]}}`,
		`{"type":"pipeline_event","event":"task_start","stage_id":"main","task_id":"task_2","model":"sonnet","verbosity":"v","workspace":"shared"}`,
		`{"type":"pipeline_event","event":"task_session_bind","stage_id":"main","task_id":"task_2","session_id":"s2"}`,
		`{"type":"assistant","session_id":"s2","message":{"id":"m2","content":[{"type":"tool_use","id":"tool-b","name":"Bash","input":{"command":"rg TODO -n"}}]}}`,
		`{"type":"user","session_id":"s2","message":{"content":[{"tool_use_id":"tool-b","type":"tool_result","content":"done","is_error":false}]},"tool_use_result":{"stdout":"done","stderr":"","interrupted":false,"isImage":false,"noOutputExpected":false}}`,
		`{"type":"result","subtype":"success","is_error":false,"duration_ms":5,"duration_api_ms":5,"num_turns":1,"result":"Task 2 completed\nAll checks passed","stop_reason":null,"session_id":"s2","total_cost_usd":0.1,"usage":{"input_tokens":10,"cache_creation_input_tokens":1,"cache_read_input_tokens":2,"output_tokens":3,"server_tool_use":{"web_search_requests":0,"web_fetch_requests":0},"service_tier":"standard"},"modelUsage":{},"uuid":"u2"}`,
		`{"type":"pipeline_event","event":"task_finish","stage_id":"main","task_id":"task_2","status":"success","duration_ms":1000}`,
	}

	for _, line := range lines {
		model = applyStreamLine(t, model, line)
	}

	view := model.View()
	assertContains(t, view, "Running pipeline")
	assertContains(t, view, "main")
	assertContains(t, view, "task_1")
	assertContains(t, view, "Bash: go test ./...")
	assertContains(t, view, "task_2")
	assertContains(t, view, "Task 2 completed")
	assertContains(t, view, "All checks passed")
	assertNotContains(t, view, "rg TODO -n")
}

func TestProgressTUIModelCtrlOTogglesExpanded(t *testing.T) {
	t.Parallel()

	model := newProgressTUIModel(true, nil)
	if model.expanded {
		t.Fatal("expected compact mode by default")
	}

	nextModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	updated, ok := nextModel.(progressTUIModel)
	if !ok {
		t.Fatalf("unexpected model type: %T", nextModel)
	}
	if !updated.expanded {
		t.Fatal("expected expanded mode after ctrl+o")
	}

	nextModel, _ = updated.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	updated, ok = nextModel.(progressTUIModel)
	if !ok {
		t.Fatalf("unexpected model type: %T", nextModel)
	}
	if updated.expanded {
		t.Fatal("expected compact mode after second ctrl+o")
	}
}

func TestProgressTUIModelOutOfOrderResultBind(t *testing.T) {
	t.Parallel()

	model := newProgressTUIModel(true, nil)

	model = applyStreamLine(
		t,
		model,
		`{"type":"result","subtype":"success","is_error":false,"duration_ms":5,"duration_api_ms":5,"num_turns":1,"result":"Done before bind","stop_reason":null,"session_id":"s1","total_cost_usd":0.25,"usage":{"input_tokens":10,"cache_creation_input_tokens":1,"cache_read_input_tokens":2,"output_tokens":3,"server_tool_use":{"web_search_requests":0,"web_fetch_requests":0},"service_tier":"standard"},"modelUsage":{},"uuid":"u1"}`,
	)
	model = applyStreamLine(t, model, `{"type":"pipeline_event","event":"task_start","stage_id":"main","task_id":"task_1"}`)
	model = applyStreamLine(t, model, `{"type":"pipeline_event","event":"task_session_bind","stage_id":"main","task_id":"task_1","session_id":"s1"}`)

	task := model.lookupTask("main", "task_1")
	if task == nil {
		t.Fatal("expected task to exist")
	}

	if task.Tokens != 16 {
		t.Fatalf("unexpected token total: %d", task.Tokens)
	}
	if task.CacheReadTokens != 2 {
		t.Fatalf("unexpected cache_read token total: %d", task.CacheReadTokens)
	}
	if task.CostUSD != 0.25 {
		t.Fatalf("unexpected cost: %f", task.CostUSD)
	}
	if !strings.Contains(task.ResultText, "Done before bind") {
		t.Fatalf("unexpected result text: %q", task.ResultText)
	}
}

func TestProgressTUIModelPipelineStatsTableIncludesUsageColumns(t *testing.T) {
	t.Parallel()

	model := newProgressTUIModel(true, nil)
	model = applyStreamLine(t, model, `{"type":"pipeline_event","event":"task_start","stage_id":"dev","task_id":"task_a"}`)
	model = applyStreamLine(t, model, `{"type":"pipeline_event","event":"task_session_bind","stage_id":"dev","task_id":"task_a","session_id":"s1"}`)
	model = applyStreamLine(t, model, `{"type":"assistant","session_id":"s1","message":{"id":"m1","content":[{"type":"tool_use","id":"tool-a","name":"Bash","input":{"command":"go test"}}]}}`)

	normalized := stats.PipelineTaskNormalized{
		InputTokens:              10,
		CacheCreationInputTokens: 1,
		CacheReadInputTokens:     2,
		OutputTokens:             3,
		CostUSD:                  0.125,
		ByModel:                  map[string]stats.PipelineTaskModelMetric{},
	}
	model.finalRecord = &stats.RunRecord{
		Status: stats.RunStatusSuccess,
		Pipeline: &stats.PipelineRunRecord{
			Tasks: []stats.PipelineTaskRecord{
				{
					StageID:    "dev",
					TaskID:     "task_a",
					Status:     "success",
					DurationMS: 1500,
					Normalized: &normalized,
				},
			},
		},
	}

	table := strings.Join(model.renderPipelineStatsTable(), "\n")
	assertContains(t, table, "STAGE/TASK")
	assertContains(t, table, "STATUS")
	assertContains(t, table, "DURATION")
	assertContains(t, table, "TOOL_USES")
	assertContains(t, table, "TOKENS")
	assertContains(t, table, "CACHE_READ")
	assertContains(t, table, "COST_USD")
	assertContains(t, table, "dev/task_a")
	assertContains(t, table, "16")
	assertContains(t, table, "2")
	assertContains(t, table, "0.125000")
}

func TestProgressTUIModelNonPipelineSummaryFooter(t *testing.T) {
	t.Parallel()

	model := newProgressTUIModel(false, nil)
	model = applyStreamLine(t, model, `{"type":"system","subtype":"init","session_id":"s1","model":"claude-sonnet"}`)
	model = applyStreamLine(t, model, `{"type":"assistant","session_id":"s1","message":{"id":"m1","content":[{"type":"tool_use","id":"tool-a","name":"Bash","input":{"command":"go version"}}]}}`)
	model = applyStreamLine(t, model, `{"type":"result","subtype":"success","is_error":false,"duration_ms":5,"duration_api_ms":5,"num_turns":1,"result":"Go 1.26","stop_reason":null,"session_id":"s1","total_cost_usd":0.1,"usage":{"input_tokens":10,"cache_creation_input_tokens":1,"cache_read_input_tokens":2,"output_tokens":3,"server_tool_use":{"web_search_requests":0,"web_fetch_requests":0},"service_tier":"standard"},"modelUsage":{},"uuid":"u1"}`)

	finishedModel, _ := model.Update(runFinishedMsg{
		Record: &stats.RunRecord{
			Status: stats.RunStatusSuccess,
			Normalized: result.NormalizedMetrics{
				InputTokens:              10,
				CacheCreationInputTokens: 1,
				CacheReadInputTokens:     2,
				OutputTokens:             3,
			},
		},
	})

	updated, ok := finishedModel.(progressTUIModel)
	if !ok {
		t.Fatalf("unexpected model type: %T", finishedModel)
	}

	view := updated.View()
	assertContains(t, view, "Run completed")
	assertContains(t, view, "Go 1.26")
	assertContains(t, view, "status: success")
	assertContains(t, view, "input_tokens: 10")
	assertContains(t, view, "total_tokens: 16")
}

func TestFormatCompactTokens(t *testing.T) {
	t.Parallel()

	cases := []struct {
		tokens int64
		want   string
	}{
		{tokens: 999, want: "999"},
		{tokens: 1000, want: "1k"},
		{tokens: 156900, want: "156.9k"},
		{tokens: 1_000_000, want: "1m"},
		{tokens: 1_250_000, want: "1.2m"},
	}

	for _, tc := range cases {
		got := formatCompactTokens(tc.tokens)
		if got != tc.want {
			t.Fatalf("formatCompactTokens(%d) = %q, want %q", tc.tokens, got, tc.want)
		}
	}
}

func applyStreamLine(t *testing.T, model progressTUIModel, line string) progressTUIModel {
	t.Helper()

	event, kind, err := result.ParseStreamLine(line)
	if err != nil {
		t.Fatalf("parse stream line: %v", err)
	}
	if kind != result.StreamLineJSONEvent {
		t.Fatalf("unexpected stream kind: %s", kind)
	}

	nextModel, _ := model.Update(streamEventMsg{Event: event})
	updated, ok := nextModel.(progressTUIModel)
	if !ok {
		t.Fatalf("unexpected model type: %T", nextModel)
	}
	return updated
}

func assertContains(t *testing.T, output string, expected string) {
	t.Helper()
	if !strings.Contains(output, expected) {
		t.Fatalf("expected output to contain %q, got %q", expected, output)
	}
}

func assertNotContains(t *testing.T, output string, expected string) {
	t.Helper()
	if strings.Contains(output, expected) {
		t.Fatalf("expected output to not contain %q, got %q", expected, output)
	}
}
