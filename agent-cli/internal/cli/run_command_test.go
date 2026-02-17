package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-cli/internal/config"
	"agent-cli/internal/runner"
	"agent-cli/internal/stats"
)

func TestRunCommandSuccessStream(t *testing.T) {
	cwd := t.TempDir()
	writeTestConfig(t, cwd)

	lines := []string{
		`{"type":"system","subtype":"init","session_id":"s1","model":"claude-sonnet"}`,
		`{"type":"assistant","session_id":"s1","message":{"id":"m1","content":[{"type":"tool_use","id":"tool-bash","name":"Bash","input":{"command":"go version","description":"Print Go version"}}]}}`,
		`{"type":"user","session_id":"s1","message":{"content":[{"tool_use_id":"tool-bash","type":"tool_result","content":"go version","is_error":false}]},"tool_use_result":{"stdout":"go version go1.26.0 linux/arm64","stderr":"","interrupted":false,"isImage":false,"noOutputExpected":false}}`,
		`{"type":"assistant","session_id":"s1","message":{"id":"m2","content":[{"type":"tool_use","id":"tool-todo","name":"TodoWrite","input":{"todos":[{"content":"Build project","status":"in_progress","activeForm":"Building"}]}}]}}`,
		`{"type":"user","session_id":"s1","message":{"content":[{"tool_use_id":"tool-todo","type":"tool_result","content":"Todos updated","is_error":false}]},"tool_use_result":{"oldTodos":[{"content":"Build project","status":"pending","activeForm":"Building"}],"newTodos":[{"content":"Build project","status":"completed","activeForm":"Building"}]}}`,
		`{"type":"result","subtype":"success","is_error":false,"duration_ms":10,"duration_api_ms":12,"num_turns":2,"result":"ok","stop_reason":null,"session_id":"s1","total_cost_usd":0.5,"usage":{"input_tokens":10,"cache_creation_input_tokens":3,"cache_read_input_tokens":4,"output_tokens":5,"server_tool_use":{"web_search_requests":0,"web_fetch_requests":0},"service_tier":"standard"},"modelUsage":{"claude-sonnet":{"inputTokens":10,"outputTokens":5,"cacheReadInputTokens":4,"cacheCreationInputTokens":3,"webSearchRequests":0,"costUSD":0.5}},"uuid":"u1"}`,
	}

	restore := withRunCommandDeps(t, func(ctx context.Context, req runner.RunRequest, hooks runner.StreamHooks) (runner.RunOutput, error) {
		for _, line := range lines {
			if hooks.OnStdoutLine != nil {
				hooks.OnStdoutLine(line)
			}
		}
		return runner.RunOutput{
			Stdout:   strings.Join(lines, "\n") + "\n",
			Stderr:   "",
			ExitCode: 0,
		}, nil
	})
	defer restore()

	var out bytes.Buffer
	runOutputWriter = &out

	if err := RunCommand(context.Background(), cwd, []string{"build", "project"}); err != nil {
		t.Fatalf("run command: %v", err)
	}

	output := out.String()
	assertContains(t, output, "Run completed")
	assertContains(t, output, "run")
	assertContains(t, output, "prompt")
	assertContains(t, output, "prompt · success")
	assertContains(t, output, "ok")
	assertContains(t, output, "status: success")
	assertContains(t, output, "input_tokens: 10")
	assertContains(t, output, "cache_creation_input_tokens: 3")
	assertContains(t, output, "cache_read_input_tokens: 4")
	assertContains(t, output, "output_tokens: 5")
	assertContains(t, output, "total_tokens: 22")

	saved := loadSingleRunRecord(t, cwd)
	record := saved.Record
	expectedDirSuffix := "-" + record.RunID
	if !strings.HasSuffix(filepath.Base(saved.RunDir), expectedDirSuffix) {
		t.Fatalf("expected run dir to end with %q, got %q", expectedDirSuffix, filepath.Base(saved.RunDir))
	}

	if _, err := os.Stat(filepath.Join(saved.RunDir, "prompt.md")); !os.IsNotExist(err) {
		t.Fatalf("expected prompt.md to be absent, got err=%v", err)
	}

	outputContent, err := os.ReadFile(filepath.Join(saved.RunDir, "output.log"))
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if got := string(outputContent); got != strings.Join(lines, "\n")+"\n" {
		t.Fatalf("unexpected output log content: %q", got)
	}

	if strings.Contains(string(saved.RawStatsJSON), "\"stdout_raw\"") {
		t.Fatal("stats.json should not contain stdout_raw")
	}
	if strings.Contains(string(saved.RawStatsJSON), "\"stderr_raw\"") {
		t.Fatal("stats.json should not contain stderr_raw")
	}
	if strings.Contains(string(saved.RawStatsJSON), "\"prompt\":") {
		t.Fatal("stats.json should not contain prompt")
	}
}

func TestRunCommandParseError(t *testing.T) {
	cwd := t.TempDir()
	writeTestConfig(t, cwd)

	restore := withRunCommandDeps(t, func(ctx context.Context, req runner.RunRequest, hooks runner.StreamHooks) (runner.RunOutput, error) {
		if hooks.OnStdoutLine != nil {
			hooks.OnStdoutLine("plain text line")
		}
		return runner.RunOutput{
			Stdout:   "plain text line\n",
			Stderr:   "",
			ExitCode: 0,
		}, nil
	})
	defer restore()

	var out bytes.Buffer
	runOutputWriter = &out

	err := RunCommand(context.Background(), cwd, []string{"build"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "final result event not found") {
		t.Fatalf("unexpected error: %v", err)
	}

	record := loadSingleRunRecord(t, cwd).Record
	if record.Status != stats.RunStatusParseError {
		t.Fatalf("unexpected status: %s", record.Status)
	}
}

func TestRunCommandDockerExitError(t *testing.T) {
	cwd := t.TempDir()
	writeTestConfig(t, cwd)

	lines := []string{
		`{"type":"result","subtype":"success","is_error":false,"duration_ms":1,"duration_api_ms":2,"num_turns":1,"result":"ok","stop_reason":null,"session_id":"s1","total_cost_usd":0.1,"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1,"server_tool_use":{"web_search_requests":0,"web_fetch_requests":0},"service_tier":"standard"},"modelUsage":{},"uuid":"u1"}`,
	}

	restore := withRunCommandDeps(t, func(ctx context.Context, req runner.RunRequest, hooks runner.StreamHooks) (runner.RunOutput, error) {
		for _, line := range lines {
			if hooks.OnStdoutLine != nil {
				hooks.OnStdoutLine(line)
			}
		}
		return runner.RunOutput{
			Stdout:   strings.Join(lines, "\n") + "\n",
			Stderr:   "docker error",
			ExitCode: 17,
		}, errors.New("exit status 17")
	})
	defer restore()

	var out bytes.Buffer
	runOutputWriter = &out

	err := RunCommand(context.Background(), cwd, []string{"build"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "docker exited with code 17") {
		t.Fatalf("unexpected error: %v", err)
	}

	saved := loadSingleRunRecord(t, cwd)
	record := saved.Record
	if record.Status != stats.RunStatusError {
		t.Fatalf("unexpected status: %s", record.Status)
	}
	if record.ErrorType != "docker_exit_error" {
		t.Fatalf("unexpected error type: %s", record.ErrorType)
	}

	outputContent, err := os.ReadFile(filepath.Join(saved.RunDir, "output.log"))
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	expectedOutput := strings.Join(lines, "\n") + "\n" + "docker error"
	if got := string(outputContent); got != expectedOutput {
		t.Fatalf("unexpected output log content: %q", got)
	}
}

func TestRunCommandInterrupted(t *testing.T) {
	cwd := t.TempDir()
	writeTestConfig(t, cwd)

	restore := withRunCommandDeps(t, func(ctx context.Context, req runner.RunRequest, hooks runner.StreamHooks) (runner.RunOutput, error) {
		return runner.RunOutput{
			Stdout:   "",
			Stderr:   "",
			ExitCode: 130,
		}, fmt.Errorf("%w: run interrupted by signal", runner.ErrInterrupted)
	})
	defer restore()

	var out bytes.Buffer
	runOutputWriter = &out

	err := RunCommand(context.Background(), cwd, []string{"build"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "interrupted") {
		t.Fatalf("unexpected error: %v", err)
	}

	record := loadSingleRunRecord(t, cwd).Record
	if record.Status != stats.RunStatusError {
		t.Fatalf("unexpected status: %s", record.Status)
	}
	if record.ErrorType != "interrupted" {
		t.Fatalf("unexpected error type: %s", record.ErrorType)
	}
	if record.DockerExitCode != 130 {
		t.Fatalf("unexpected exit code: %d", record.DockerExitCode)
	}
}

func TestRunCommandIdleTimeout(t *testing.T) {
	cwd := t.TempDir()
	writeTestConfig(t, cwd)

	restore := withRunCommandDeps(t, func(ctx context.Context, req runner.RunRequest, hooks runner.StreamHooks) (runner.RunOutput, error) {
		return runner.RunOutput{
			Stdout:   "",
			Stderr:   "",
			ExitCode: -1,
		}, fmt.Errorf("%w: no log activity for 30m0s", runner.ErrIdleTimeout)
	})
	defer restore()

	var out bytes.Buffer
	runOutputWriter = &out

	err := RunCommand(context.Background(), cwd, []string{"build"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "no log activity") {
		t.Fatalf("unexpected error: %v", err)
	}

	record := loadSingleRunRecord(t, cwd).Record
	if record.Status != stats.RunStatusError {
		t.Fatalf("unexpected status: %s", record.Status)
	}
	if record.ErrorType != "timeout" {
		t.Fatalf("unexpected error type: %s", record.ErrorType)
	}
}

func TestRunCommandJSONOutputOnlyFinalResult(t *testing.T) {
	cwd := t.TempDir()
	writeTestConfig(t, cwd)

	resultLine := `{"type":"result","subtype":"success","is_error":false,"duration_ms":1,"duration_api_ms":2,"num_turns":1,"result":"ok","stop_reason":null,"session_id":"s1","total_cost_usd":0.1,"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1,"server_tool_use":{"web_search_requests":0,"web_fetch_requests":0},"service_tier":"standard"},"modelUsage":{},"uuid":"u1"}`
	lines := []string{
		`{"type":"system","subtype":"init","session_id":"s1","model":"claude-sonnet"}`,
		resultLine,
	}

	restore := withRunCommandDeps(t, func(ctx context.Context, req runner.RunRequest, hooks runner.StreamHooks) (runner.RunOutput, error) {
		for _, line := range lines {
			if hooks.OnStdoutLine != nil {
				hooks.OnStdoutLine(line)
			}
		}
		return runner.RunOutput{
			Stdout:   strings.Join(lines, "\n") + "\n",
			Stderr:   "",
			ExitCode: 0,
		}, nil
	})
	defer restore()

	var out bytes.Buffer
	runOutputWriter = &out

	if err := RunCommand(context.Background(), cwd, []string{"--json", "build"}); err != nil {
		t.Fatalf("run command: %v", err)
	}

	output := out.String()
	if output != resultLine+"\n" {
		t.Fatalf("unexpected json output: %q", output)
	}
}

func TestRunCommandFileInputDoesNotPersistPromptArtifact(t *testing.T) {
	cwd := t.TempDir()
	writeTestConfig(t, cwd)

	promptPath := filepath.Join(cwd, "prompt.txt")
	if err := os.WriteFile(promptPath, []byte("build from file"), 0o644); err != nil {
		t.Fatalf("write prompt file: %v", err)
	}

	resultLine := `{"type":"result","subtype":"success","is_error":false,"duration_ms":1,"duration_api_ms":2,"num_turns":1,"result":"ok","stop_reason":null,"session_id":"s1","total_cost_usd":0.1,"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1,"server_tool_use":{"web_search_requests":0,"web_fetch_requests":0},"service_tier":"standard"},"modelUsage":{},"uuid":"u1"}`
	lines := []string{resultLine}

	var capturedReq runner.RunRequest
	restore := withRunCommandDeps(t, func(ctx context.Context, req runner.RunRequest, hooks runner.StreamHooks) (runner.RunOutput, error) {
		capturedReq = req
		for _, line := range lines {
			if hooks.OnStdoutLine != nil {
				hooks.OnStdoutLine(line)
			}
		}
		return runner.RunOutput{
			Stdout:   strings.Join(lines, "\n") + "\n",
			Stderr:   "",
			ExitCode: 0,
		}, nil
	})
	defer restore()

	var out bytes.Buffer
	runOutputWriter = &out

	if err := RunCommand(context.Background(), cwd, []string{"--file", promptPath}); err != nil {
		t.Fatalf("run command: %v", err)
	}
	if capturedReq.Prompt != "build from file" {
		t.Fatalf("unexpected prompt passed to runner: %q", capturedReq.Prompt)
	}

	saved := loadSingleRunRecord(t, cwd)
	if _, err := os.Stat(filepath.Join(saved.RunDir, "prompt.md")); !os.IsNotExist(err) {
		t.Fatalf("expected prompt.md to be absent, got err=%v", err)
	}
	if strings.Contains(string(saved.RawStatsJSON), "\"prompt\":") {
		t.Fatal("stats.json should not contain prompt")
	}
}

func TestRunCommandPipelineJSONOutput(t *testing.T) {
	cwd := t.TempDir()
	writeTestConfig(t, cwd)

	planPath := filepath.Join(cwd, "pipeline.yaml")
	planContent := "version: v1\nstages:\n  - id: dev\n    mode: sequential\n    tasks:\n      - id: implement\n        prompt: hello\n"
	if err := os.WriteFile(planPath, []byte(planContent), 0o644); err != nil {
		t.Fatalf("write plan file: %v", err)
	}

	pipelineResultLine := `{"type":"pipeline_result","version":"v1","status":"success","is_error":false,"stage_count":1,"completed_stages":1,"task_count":1,"failed_task_count":0,"tasks":[{"stage_id":"dev","task_id":"implement","status":"success","on_error":"fail_fast","workspace":"shared","model":"opus","verbosity":"vv","prompt_source":"prompt","exit_code":0,"started_at":"2026-02-16T00:00:00Z","finished_at":"2026-02-16T00:00:01Z","duration_ms":1000}]}`
	lines := []string{
		`{"type":"pipeline_event","event":"plan_start"}`,
		pipelineResultLine,
	}

	var capturedReq runner.RunRequest
	restore := withRunCommandDeps(t, func(ctx context.Context, req runner.RunRequest, hooks runner.StreamHooks) (runner.RunOutput, error) {
		capturedReq = req
		for _, line := range lines {
			if hooks.OnStdoutLine != nil {
				hooks.OnStdoutLine(line)
			}
		}
		return runner.RunOutput{
			Stdout:   strings.Join(lines, "\n") + "\n",
			Stderr:   "",
			ExitCode: 0,
		}, nil
	})
	defer restore()

	var out bytes.Buffer
	runOutputWriter = &out

	if err := RunCommand(context.Background(), cwd, []string{"--json", "--pipeline", planPath}); err != nil {
		t.Fatalf("run command: %v", err)
	}

	if capturedReq.Pipeline != planPath {
		t.Fatalf("expected pipeline path in request, got %q", capturedReq.Pipeline)
	}
	if capturedReq.Prompt != "" {
		t.Fatalf("expected empty prompt for plan run, got %q", capturedReq.Prompt)
	}

	if output := out.String(); output != pipelineResultLine+"\n" {
		t.Fatalf("unexpected json output: %q", output)
	}

	saved := loadSingleRunRecord(t, cwd)
	if saved.Record.Pipeline == nil {
		t.Fatal("expected pipeline data in record")
	}
	if saved.Record.Pipeline.TaskCount != 1 {
		t.Fatalf("unexpected pipeline task count: %d", saved.Record.Pipeline.TaskCount)
	}

	if _, err := os.Stat(filepath.Join(saved.RunDir, "prompt.md")); !os.IsNotExist(err) {
		t.Fatalf("expected prompt.md to be absent, got err=%v", err)
	}
	if strings.Contains(string(saved.RawStatsJSON), "\"prompt\":") {
		t.Fatal("stats.json should not contain prompt")
	}
}

func TestRunCommandPipelineFailureReturnsTaskErrorDetails(t *testing.T) {
	cwd := t.TempDir()
	writeTestConfig(t, cwd)

	planPath := filepath.Join(cwd, "pipeline.yaml")
	planContent := "version: v1\nstages:\n  - id: main\n    mode: sequential\n    tasks:\n      - id: print_version\n        prompt: hi\n"
	if err := os.WriteFile(planPath, []byte(planContent), 0o644); err != nil {
		t.Fatalf("write plan file: %v", err)
	}

	pipelineResultLine := `{"type":"pipeline_result","version":"v1","status":"error","is_error":true,"stage_count":1,"completed_stages":1,"task_count":1,"failed_task_count":1,"tasks":[{"stage_id":"main","task_id":"print_version","status":"error","on_error":"fail_fast","workspace":"shared","model":"sonnet","verbosity":"vv","prompt_source":"prompt","exit_code":124,"started_at":"2026-02-17T17:25:31Z","finished_at":"2026-02-17T17:55:31Z","duration_ms":1800000,"error_message":"idle timeout after 30 seconds without task output"}]}`
	lines := []string{
		`{"type":"pipeline_event","event":"task_timeout","stage_id":"main","task_id":"print_version","idle_timeout_sec":30,"reason":"idle timeout after 30 seconds without task output"}`,
		pipelineResultLine,
	}

	restore := withRunCommandDeps(t, func(ctx context.Context, req runner.RunRequest, hooks runner.StreamHooks) (runner.RunOutput, error) {
		for _, line := range lines {
			if hooks.OnStdoutLine != nil {
				hooks.OnStdoutLine(line)
			}
		}
		return runner.RunOutput{
			Stdout:   strings.Join(lines, "\n") + "\n",
			Stderr:   "",
			ExitCode: 1,
		}, errors.New("container exited with code 1")
	})
	defer restore()

	var out bytes.Buffer
	runOutputWriter = &out

	err := RunCommand(context.Background(), cwd, []string{"--pipeline", planPath})
	if err == nil {
		t.Fatal("expected error")
	}
	expectedMessage := "pipeline failed at main/print_version: idle timeout after 30 seconds without task output"
	if !strings.Contains(err.Error(), expectedMessage) {
		t.Fatalf("unexpected error: %v", err)
	}

	record := loadSingleRunRecord(t, cwd).Record
	if record.ErrorType != "pipeline_timeout" {
		t.Fatalf("unexpected error type: %s", record.ErrorType)
	}
	if !strings.Contains(record.ErrorMessage, expectedMessage) {
		t.Fatalf("unexpected record error message: %q", record.ErrorMessage)
	}
}

func TestRunCommandPipelineSummaryShowsTaskStatsTable(t *testing.T) {
	cwd := t.TempDir()
	writeTestConfig(t, cwd)

	planPath := filepath.Join(cwd, "pipeline.yaml")
	planContent := "version: v1\nstages:\n  - id: dev\n    mode: sequential\n    tasks:\n      - id: implement\n        prompt: hello\n"
	if err := os.WriteFile(planPath, []byte(planContent), 0o644); err != nil {
		t.Fatalf("write plan file: %v", err)
	}

	resultLineA := `{"type":"result","subtype":"success","is_error":false,"duration_ms":10,"duration_api_ms":12,"num_turns":1,"result":"ok","stop_reason":null,"session_id":"s1","total_cost_usd":0.1,"usage":{"input_tokens":10,"cache_creation_input_tokens":1,"cache_read_input_tokens":2,"output_tokens":3,"server_tool_use":{"web_search_requests":0,"web_fetch_requests":0},"service_tier":"standard"},"modelUsage":{},"uuid":"u1"}`
	resultLineB := `{"type":"result","subtype":"success","is_error":false,"duration_ms":5,"duration_api_ms":6,"num_turns":1,"result":"ok","stop_reason":null,"session_id":"s2","total_cost_usd":0.05,"usage":{"input_tokens":4,"cache_creation_input_tokens":0,"cache_read_input_tokens":1,"output_tokens":2,"server_tool_use":{"web_search_requests":0,"web_fetch_requests":0},"service_tier":"standard"},"modelUsage":{},"uuid":"u2"}`
	pipelineResultLine := `{"type":"pipeline_result","version":"v1","status":"success","is_error":false,"stage_count":1,"completed_stages":1,"task_count":2,"failed_task_count":0,"tasks":[]}`
	lines := []string{
		`{"type":"pipeline_event","event":"plan_start"}`,
		resultLineA,
		resultLineB,
		pipelineResultLine,
	}

	restore := withRunCommandDeps(t, func(ctx context.Context, req runner.RunRequest, hooks runner.StreamHooks) (runner.RunOutput, error) {
		for _, line := range lines {
			if hooks.OnStdoutLine != nil {
				hooks.OnStdoutLine(line)
			}
		}
		return runner.RunOutput{
			Stdout:   strings.Join(lines, "\n") + "\n",
			Stderr:   "",
			ExitCode: 0,
		}, nil
	})
	defer restore()

	var out bytes.Buffer
	runOutputWriter = &out

	if err := RunCommand(context.Background(), cwd, []string{"--pipeline", planPath}); err != nil {
		t.Fatalf("run command: %v", err)
	}

	output := out.String()
	assertContains(t, output, "Pipeline completed")
	assertContains(t, output, "Pipeline Task Stats")
	assertContains(t, output, "STAGE/TASK")
	assertContains(t, output, "TOOL_USES")
	assertContains(t, output, "CACHE_READ")
	assertContains(t, output, "COST_USD")
	assertNotContains(t, output, "run_id:")
	assertNotContains(t, output, "stats_file:")
	assertNotContains(t, output, "docker_exit_code:")
}

func TestRunCommandPipelineTaskNormalizedIncludesCostWebSearchAndOutOfOrder(t *testing.T) {
	cwd := t.TempDir()
	writeTestConfig(t, cwd)

	planPath := filepath.Join(cwd, "pipeline.yaml")
	planContent := "version: v1\nstages:\n  - id: dev\n    mode: sequential\n    tasks:\n      - id: task_a\n        prompt: a\n      - id: task_b\n        prompt: b\n"
	if err := os.WriteFile(planPath, []byte(planContent), 0o644); err != nil {
		t.Fatalf("write plan file: %v", err)
	}

	resultLineA := `{"type":"result","subtype":"success","is_error":false,"duration_ms":10,"duration_api_ms":12,"num_turns":1,"result":"ok","stop_reason":null,"session_id":"s1","total_cost_usd":0.25,"usage":{"input_tokens":10,"cache_creation_input_tokens":1,"cache_read_input_tokens":2,"output_tokens":3,"server_tool_use":{"web_search_requests":2,"web_fetch_requests":0},"service_tier":"standard"},"modelUsage":{"claude-opus":{"inputTokens":10,"outputTokens":3,"cacheReadInputTokens":2,"cacheCreationInputTokens":1,"webSearchRequests":2,"costUSD":0.25}},"uuid":"u1"}`
	resultLineB := `{"type":"result","subtype":"success","is_error":false,"duration_ms":5,"duration_api_ms":6,"num_turns":1,"result":"ok","stop_reason":null,"session_id":"s2","total_cost_usd":0.05,"usage":{"input_tokens":4,"cache_creation_input_tokens":0,"cache_read_input_tokens":1,"output_tokens":2,"server_tool_use":{"web_search_requests":1,"web_fetch_requests":0},"service_tier":"standard"},"modelUsage":{"claude-sonnet":{"inputTokens":4,"outputTokens":2,"cacheReadInputTokens":1,"cacheCreationInputTokens":0,"webSearchRequests":1,"costUSD":0.05}},"uuid":"u2"}`
	pipelineResultLine := `{"type":"pipeline_result","version":"v1","status":"success","is_error":false,"stage_count":1,"completed_stages":1,"task_count":2,"failed_task_count":0,"tasks":[{"stage_id":"dev","task_id":"task_a","status":"success","on_error":"fail_fast","workspace":"shared","model":"opus","verbosity":"vv","prompt_source":"prompt","exit_code":0,"started_at":"2026-02-16T00:00:00Z","finished_at":"2026-02-16T00:00:01Z","duration_ms":1000},{"stage_id":"dev","task_id":"task_b","status":"success","on_error":"fail_fast","workspace":"shared","model":"opus","verbosity":"vv","prompt_source":"prompt","exit_code":0,"started_at":"2026-02-16T00:00:01Z","finished_at":"2026-02-16T00:00:02Z","duration_ms":1000}]}`
	lines := []string{
		resultLineB,
		`{"type":"pipeline_event","event":"task_session_bind","stage_id":"dev","task_id":"task_a","session_id":"s1"}`,
		resultLineA,
		`{"type":"pipeline_event","event":"task_session_bind","stage_id":"dev","task_id":"task_b","session_id":"s2"}`,
		pipelineResultLine,
	}

	restore := withRunCommandDeps(t, func(ctx context.Context, req runner.RunRequest, hooks runner.StreamHooks) (runner.RunOutput, error) {
		for _, line := range lines {
			if hooks.OnStdoutLine != nil {
				hooks.OnStdoutLine(line)
			}
		}
		return runner.RunOutput{
			Stdout:   strings.Join(lines, "\n") + "\n",
			Stderr:   "",
			ExitCode: 0,
		}, nil
	})
	defer restore()

	var out bytes.Buffer
	runOutputWriter = &out

	if err := RunCommand(context.Background(), cwd, []string{"--pipeline", planPath}); err != nil {
		t.Fatalf("run command: %v", err)
	}

	saved := loadSingleRunRecord(t, cwd)
	if strings.Contains(string(saved.RawStatsJSON), "\"stream\":") {
		t.Fatal("stats.json should not contain stream")
	}

	pipeline := saved.Record.Pipeline
	if pipeline == nil {
		t.Fatal("expected pipeline record")
	}

	taskA := mustFindPipelineTask(t, pipeline, "dev", "task_a")
	if taskA.Normalized == nil {
		t.Fatal("expected normalized metrics for task_a")
	}
	if taskA.Normalized.InputTokens != 10 {
		t.Fatalf("unexpected task_a input tokens: %d", taskA.Normalized.InputTokens)
	}
	if taskA.Normalized.CacheCreationInputTokens != 1 {
		t.Fatalf("unexpected task_a cache_creation_input_tokens: %d", taskA.Normalized.CacheCreationInputTokens)
	}
	if taskA.Normalized.CacheReadInputTokens != 2 {
		t.Fatalf("unexpected task_a cache_read_input_tokens: %d", taskA.Normalized.CacheReadInputTokens)
	}
	if taskA.Normalized.OutputTokens != 3 {
		t.Fatalf("unexpected task_a output_tokens: %d", taskA.Normalized.OutputTokens)
	}
	if taskA.Normalized.WebSearchRequests != 2 {
		t.Fatalf("unexpected task_a web_search_requests: %d", taskA.Normalized.WebSearchRequests)
	}
	assertFloatNear(t, taskA.Normalized.CostUSD, 0.25, "task_a cost_usd")
	modelA, ok := taskA.Normalized.ByModel["claude-opus"]
	if !ok {
		t.Fatal("expected claude-opus model metrics for task_a")
	}
	if modelA.WebSearchRequests != 2 {
		t.Fatalf("unexpected task_a by_model web_search_requests: %d", modelA.WebSearchRequests)
	}
	assertFloatNear(t, modelA.CostUSD, 0.25, "task_a by_model cost_usd")

	taskB := mustFindPipelineTask(t, pipeline, "dev", "task_b")
	if taskB.Normalized == nil {
		t.Fatal("expected normalized metrics for task_b")
	}
	if taskB.Normalized.InputTokens != 4 {
		t.Fatalf("unexpected task_b input tokens: %d", taskB.Normalized.InputTokens)
	}
	if taskB.Normalized.WebSearchRequests != 1 {
		t.Fatalf("unexpected task_b web_search_requests: %d", taskB.Normalized.WebSearchRequests)
	}
	assertFloatNear(t, taskB.Normalized.CostUSD, 0.05, "task_b cost_usd")
	modelB, ok := taskB.Normalized.ByModel["claude-sonnet"]
	if !ok {
		t.Fatal("expected claude-sonnet model metrics for task_b")
	}
	if modelB.WebSearchRequests != 1 {
		t.Fatalf("unexpected task_b by_model web_search_requests: %d", modelB.WebSearchRequests)
	}
	assertFloatNear(t, modelB.CostUSD, 0.05, "task_b by_model cost_usd")
}

func TestRunCommandPipelineTaskNormalizedOmittedWithoutTaskResult(t *testing.T) {
	cwd := t.TempDir()
	writeTestConfig(t, cwd)

	planPath := filepath.Join(cwd, "pipeline.yaml")
	planContent := "version: v1\nstages:\n  - id: dev\n    mode: sequential\n    tasks:\n      - id: task_a\n        prompt: a\n      - id: task_b\n        prompt: b\n"
	if err := os.WriteFile(planPath, []byte(planContent), 0o644); err != nil {
		t.Fatalf("write plan file: %v", err)
	}

	resultLineA := `{"type":"result","subtype":"success","is_error":false,"duration_ms":10,"duration_api_ms":12,"num_turns":1,"result":"ok","stop_reason":null,"session_id":"s1","total_cost_usd":0.25,"usage":{"input_tokens":10,"cache_creation_input_tokens":1,"cache_read_input_tokens":2,"output_tokens":3,"server_tool_use":{"web_search_requests":2,"web_fetch_requests":0},"service_tier":"standard"},"modelUsage":{"claude-opus":{"inputTokens":10,"outputTokens":3,"cacheReadInputTokens":2,"cacheCreationInputTokens":1,"webSearchRequests":2,"costUSD":0.25}},"uuid":"u1"}`
	pipelineResultLine := `{"type":"pipeline_result","version":"v1","status":"success","is_error":false,"stage_count":1,"completed_stages":1,"task_count":2,"failed_task_count":0,"tasks":[{"stage_id":"dev","task_id":"task_a","status":"success","on_error":"fail_fast","workspace":"shared","model":"opus","verbosity":"vv","prompt_source":"prompt","exit_code":0,"started_at":"2026-02-16T00:00:00Z","finished_at":"2026-02-16T00:00:01Z","duration_ms":1000},{"stage_id":"dev","task_id":"task_b","status":"success","on_error":"fail_fast","workspace":"shared","model":"opus","verbosity":"vv","prompt_source":"prompt","exit_code":0,"started_at":"2026-02-16T00:00:01Z","finished_at":"2026-02-16T00:00:02Z","duration_ms":1000}]}`
	lines := []string{
		`{"type":"pipeline_event","event":"task_session_bind","stage_id":"dev","task_id":"task_a","session_id":"s1"}`,
		resultLineA,
		`{"type":"pipeline_event","event":"task_session_bind","stage_id":"dev","task_id":"task_b","session_id":"s2"}`,
		pipelineResultLine,
	}

	restore := withRunCommandDeps(t, func(ctx context.Context, req runner.RunRequest, hooks runner.StreamHooks) (runner.RunOutput, error) {
		for _, line := range lines {
			if hooks.OnStdoutLine != nil {
				hooks.OnStdoutLine(line)
			}
		}
		return runner.RunOutput{
			Stdout:   strings.Join(lines, "\n") + "\n",
			Stderr:   "",
			ExitCode: 0,
		}, nil
	})
	defer restore()

	var out bytes.Buffer
	runOutputWriter = &out

	if err := RunCommand(context.Background(), cwd, []string{"--pipeline", planPath}); err != nil {
		t.Fatalf("run command: %v", err)
	}

	pipeline := loadSingleRunRecord(t, cwd).Record.Pipeline
	if pipeline == nil {
		t.Fatal("expected pipeline record")
	}

	taskA := mustFindPipelineTask(t, pipeline, "dev", "task_a")
	if taskA.Normalized == nil {
		t.Fatal("expected normalized metrics for task_a")
	}
	taskB := mustFindPipelineTask(t, pipeline, "dev", "task_b")
	if taskB.Normalized != nil {
		t.Fatal("expected normalized metrics to be omitted for task_b")
	}
}

func TestRunCommandPipelineParseErrorUsesEntrypointMessage(t *testing.T) {
	cwd := t.TempDir()
	writeTestConfig(t, cwd)

	planPath := filepath.Join(cwd, "pipeline.yaml")
	planContent := "version: v1\nstages:\n  - id: main\n    mode: parallel\n    tasks:\n      - id: build\n        prompt: build\n"
	if err := os.WriteFile(planPath, []byte(planContent), 0o644); err != nil {
		t.Fatalf("write plan file: %v", err)
	}

	entrypointErr := "Entrypoint failed: Parallel task main/build uses shared workspace with writes. Set read_only=true or allow_shared_writes=true."

	restore := withRunCommandDeps(t, func(ctx context.Context, req runner.RunRequest, hooks runner.StreamHooks) (runner.RunOutput, error) {
		if hooks.OnStderrLine != nil {
			hooks.OnStderrLine(entrypointErr)
		}
		return runner.RunOutput{
			Stdout:   "",
			Stderr:   entrypointErr + "\n",
			ExitCode: 1,
		}, errors.New("container exited with code 1")
	})
	defer restore()

	var out bytes.Buffer
	runOutputWriter = &out

	err := RunCommand(context.Background(), cwd, []string{"--pipeline", planPath})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), entrypointErr) {
		t.Fatalf("unexpected error: %v", err)
	}

	record := loadSingleRunRecord(t, cwd).Record
	if record.Status != stats.RunStatusParseError {
		t.Fatalf("unexpected status: %s", record.Status)
	}
	if record.ErrorType != "pipeline_parse_error" {
		t.Fatalf("unexpected error type: %s", record.ErrorType)
	}
	if record.ErrorMessage != entrypointErr {
		t.Fatalf("unexpected error message: %q", record.ErrorMessage)
	}
}

func TestRunCommandModelOverrideTakesPriority(t *testing.T) {
	cwd := t.TempDir()
	writeTestConfigWithModel(t, cwd, "sonnet")

	resultLine := `{"type":"result","subtype":"success","is_error":false,"duration_ms":1,"duration_api_ms":2,"num_turns":1,"result":"ok","stop_reason":null,"session_id":"s1","total_cost_usd":0.1,"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1,"server_tool_use":{"web_search_requests":0,"web_fetch_requests":0},"service_tier":"standard"},"modelUsage":{},"uuid":"u1"}`
	lines := []string{resultLine}

	var capturedReq runner.RunRequest
	restore := withRunCommandDeps(t, func(ctx context.Context, req runner.RunRequest, hooks runner.StreamHooks) (runner.RunOutput, error) {
		capturedReq = req
		for _, line := range lines {
			if hooks.OnStdoutLine != nil {
				hooks.OnStdoutLine(line)
			}
		}
		return runner.RunOutput{
			Stdout:   strings.Join(lines, "\n") + "\n",
			Stderr:   "",
			ExitCode: 0,
		}, nil
	})
	defer restore()

	var out bytes.Buffer
	runOutputWriter = &out

	if err := RunCommand(context.Background(), cwd, []string{"--model", "opus", "build"}); err != nil {
		t.Fatalf("run command: %v", err)
	}
	if capturedReq.Model != "opus" {
		t.Fatalf("expected model override to win, got %q", capturedReq.Model)
	}
}

func TestRunCommandUsesConfigModelWithoutOverride(t *testing.T) {
	cwd := t.TempDir()
	writeTestConfigWithModel(t, cwd, "sonnet")

	resultLine := `{"type":"result","subtype":"success","is_error":false,"duration_ms":1,"duration_api_ms":2,"num_turns":1,"result":"ok","stop_reason":null,"session_id":"s1","total_cost_usd":0.1,"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1,"server_tool_use":{"web_search_requests":0,"web_fetch_requests":0},"service_tier":"standard"},"modelUsage":{},"uuid":"u1"}`
	lines := []string{resultLine}

	var capturedReq runner.RunRequest
	restore := withRunCommandDeps(t, func(ctx context.Context, req runner.RunRequest, hooks runner.StreamHooks) (runner.RunOutput, error) {
		capturedReq = req
		for _, line := range lines {
			if hooks.OnStdoutLine != nil {
				hooks.OnStdoutLine(line)
			}
		}
		return runner.RunOutput{
			Stdout:   strings.Join(lines, "\n") + "\n",
			Stderr:   "",
			ExitCode: 0,
		}, nil
	})
	defer restore()

	var out bytes.Buffer
	runOutputWriter = &out

	if err := RunCommand(context.Background(), cwd, []string{"build"}); err != nil {
		t.Fatalf("run command: %v", err)
	}
	if capturedReq.Model != "sonnet" {
		t.Fatalf("expected model from config, got %q", capturedReq.Model)
	}
}

func TestRunCommandUsesConfigEnableDinD(t *testing.T) {
	cwd := t.TempDir()
	writeTestConfigWithModelAndDinD(t, cwd, "", true)

	resultLine := `{"type":"result","subtype":"success","is_error":false,"duration_ms":1,"duration_api_ms":2,"num_turns":1,"result":"ok","stop_reason":null,"session_id":"s1","total_cost_usd":0.1,"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1,"server_tool_use":{"web_search_requests":0,"web_fetch_requests":0},"service_tier":"standard"},"modelUsage":{},"uuid":"u1"}`
	lines := []string{resultLine}

	var capturedReq runner.RunRequest
	restore := withRunCommandDeps(t, func(ctx context.Context, req runner.RunRequest, hooks runner.StreamHooks) (runner.RunOutput, error) {
		capturedReq = req
		for _, line := range lines {
			if hooks.OnStdoutLine != nil {
				hooks.OnStdoutLine(line)
			}
		}
		return runner.RunOutput{
			Stdout:   strings.Join(lines, "\n") + "\n",
			Stderr:   "",
			ExitCode: 0,
		}, nil
	})
	defer restore()

	var out bytes.Buffer
	runOutputWriter = &out

	if err := RunCommand(context.Background(), cwd, []string{"build"}); err != nil {
		t.Fatalf("run command: %v", err)
	}
	if !capturedReq.EnableDinD {
		t.Fatal("expected EnableDinD to be true from config")
	}
	if capturedReq.RunIdleTimeoutSec != config.DefaultRunIdleTimeoutSec {
		t.Fatalf("unexpected run idle timeout sec: %d", capturedReq.RunIdleTimeoutSec)
	}
	if capturedReq.PipelineTaskIdleTimeoutSec != config.DefaultPipelineTaskIdleTimeoutSec {
		t.Fatalf("unexpected pipeline task idle timeout sec: %d", capturedReq.PipelineTaskIdleTimeoutSec)
	}
}

func mustFindPipelineTask(
	t *testing.T,
	pipeline *stats.PipelineRunRecord,
	stageID string,
	taskID string,
) *stats.PipelineTaskRecord {
	t.Helper()

	if pipeline == nil {
		t.Fatal("pipeline record is nil")
	}
	for i := range pipeline.Tasks {
		task := &pipeline.Tasks[i]
		if task.StageID == stageID && task.TaskID == taskID {
			return task
		}
	}
	t.Fatalf("task %s/%s not found", stageID, taskID)
	return nil
}

func assertFloatNear(t *testing.T, got float64, want float64, field string) {
	t.Helper()

	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("unexpected %s: got %.12f want %.12f", field, got, want)
	}
}

func withRunCommandDeps(
	t *testing.T,
	fn func(ctx context.Context, req runner.RunRequest, hooks runner.StreamHooks) (runner.RunOutput, error),
) func() {
	t.Helper()

	prevRunner := runDockerStreamingFn
	prevWriter := runOutputWriter
	runDockerStreamingFn = fn
	runOutputWriter = os.Stdout

	return func() {
		runDockerStreamingFn = prevRunner
		runOutputWriter = prevWriter
	}
}

func writeTestConfig(t *testing.T, cwd string) {
	writeTestConfigWithModel(t, cwd, "")
}

func writeTestConfigWithModel(t *testing.T, cwd, model string) {
	writeTestConfigWithModelAndDinD(t, cwd, model, false)
}

func writeTestConfigWithModelAndDinD(t *testing.T, cwd, model string, enableDinD bool) {
	t.Helper()

	configPath := config.ConfigPath(cwd)
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	modelLine := ""
	if strings.TrimSpace(model) != "" {
		modelLine = "\nmodel = \"" + model + "\""
	}
	enableDinDLine := ""
	if enableDinD {
		enableDinDLine = "\nenable_dind = true"
	}

	content := `[docker]
image = "claude:go"` + modelLine + enableDinDLine + `

[auth]
github_token = "gh-token"
claude_token = "claude-token"

[workspace]
source_workspace_dir = "/workspace-source"

[git]
user_name = "User"
user_email = "user@example.com"
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

type savedRunRecord struct {
	Record       *stats.RunRecord
	RunDir       string
	StatsPath    string
	RawStatsJSON []byte
}

func loadSingleRunRecord(t *testing.T, cwd string) *savedRunRecord {
	t.Helper()

	runsDir := config.RunsDir(cwd)
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		t.Fatalf("read runs dir: %v", err)
	}

	runDirs := make([]os.DirEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			runDirs = append(runDirs, entry)
		}
	}
	if len(runDirs) != 1 {
		t.Fatalf("expected one run directory, got %d", len(runDirs))
	}

	runDir := filepath.Join(runsDir, runDirs[0].Name())
	path := filepath.Join(runDir, "stats.json")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read stats file: %v", err)
	}

	var record stats.RunRecord
	if err := json.Unmarshal(content, &record); err != nil {
		t.Fatalf("decode stats file: %v", err)
	}
	return &savedRunRecord{
		Record:       &record,
		RunDir:       runDir,
		StatsPath:    path,
		RawStatsJSON: content,
	}
}
