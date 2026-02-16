package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agent-cli/internal/config"
	"agent-cli/internal/result"
	"agent-cli/internal/runner"
	"agent-cli/internal/stats"
)

type runOptions struct {
	Prompt      string
	PromptPath  string
	PromptFrom  stats.PromptSource
	Pipeline    string
	PlanContent string
	JSONOutput  bool
	Model       string
}

var (
	runDockerStreamingFn           = runner.RunDockerStreaming
	runOutputWriter      io.Writer = os.Stdout
)

func RunCommand(ctx context.Context, cwd string, args []string) error {
	opts, err := parseRunArgs(cwd, args)
	if err != nil {
		return err
	}

	cfg, err := config.Load(cwd)
	if err != nil {
		return err
	}

	model := cfg.Docker.Model
	if strings.TrimSpace(opts.Model) != "" {
		model = opts.Model
	}

	isPlanRun := strings.TrimSpace(opts.Pipeline) != ""
	promptArtifactContent := opts.Prompt
	if isPlanRun {
		promptArtifactContent = opts.PlanContent
	}

	promptHash := sha256.Sum256([]byte(promptArtifactContent))
	record := &stats.RunRecord{
		Timestamp: time.Now().UTC(),
		Status:    stats.RunStatusExecError,
		CWD:       cwd,
		Stream:    stats.NewStreamMetrics(),
		Prompt: stats.PromptMetadata{
			Source:     opts.PromptFrom,
			FilePath:   opts.PromptPath,
			PromptSHA:  hex.EncodeToString(promptHash[:]),
			PromptSize: len(promptArtifactContent),
		},
	}
	record.Stream.EnsureMaps()

	stdoutLines := make([]string, 0, 32)
	startedToolUses := map[string]struct{}{}
	resolvedToolResults := map[string]struct{}{}
	lastTodoStatuses := map[string]string{}
	streamMetrics := result.NormalizedMetrics{
		ByModel: map[string]result.ModelMetric{},
	}

	var progressPrinter *ProgressPrinter
	if !opts.JSONOutput {
		progressPrinter = NewProgressPrinter(runOutputWriter)
	}

	runOutput, runErr := runDockerStreamingFn(ctx, runner.RunRequest{
		Image:              cfg.Docker.Image,
		CWD:                cwd,
		SourceWorkspaceDir: cfg.Workspace.SourceWorkspaceDir,
		GitHubToken:        cfg.Auth.GitHubToken,
		ClaudeToken:        cfg.Auth.ClaudeToken,
		GitUserName:        cfg.Git.UserName,
		GitUserEmail:       cfg.Git.UserEmail,
		Prompt:             opts.Prompt,
		Pipeline:           opts.Pipeline,
		Model:              model,
		EnableDinD:         cfg.Docker.EnableDinD,
	}, runner.StreamHooks{
		OnStdoutLine: func(line string) {
			stdoutLines = append(stdoutLines, line)
			event, kind, parseErr := result.ParseStreamLine(line)
			if parseErr != nil {
				record.Stream.InvalidJSONLines++
				return
			}

			switch kind {
			case result.StreamLineJSONEvent:
				record.Stream.TotalJSONEvents++
				if event != nil {
					if event.Result != nil {
						mergeNormalizedMetrics(&streamMetrics, result.ExtractMetrics(*event.Result))
					}
					eventKey := event.Type
					if event.System != nil && strings.TrimSpace(event.System.Subtype) != "" {
						eventKey = event.Type + "/" + event.System.Subtype
					} else if event.Pipeline != nil && strings.TrimSpace(event.Pipeline.Event) != "" {
						eventKey = event.Type + "/" + event.Pipeline.Event
					}
					if strings.TrimSpace(eventKey) != "" {
						record.Stream.EventCounts[eventKey]++
					}
					collectStreamMetrics(event, &record.Stream, startedToolUses, resolvedToolResults, lastTodoStatuses)
					if progressPrinter != nil {
						progressPrinter.HandleEvent(event)
					}
				}
			case result.StreamLineNonJSON:
				record.Stream.NonJSONLines++
			case result.StreamLineInvalidJSON:
				record.Stream.InvalidJSONLines++
			}
		},
	})
	record.Normalized = streamMetrics
	record.DockerExitCode = runOutput.ExitCode
	record.Stream.UnmatchedToolUseTotal = countUnmatchedToolUses(startedToolUses, resolvedToolResults)

	var (
		parsed      *result.ParsedResult
		parseErr    error
		pipelineRaw string
	)
	if isPlanRun {
		var pipelineRecord *stats.PipelineRunRecord
		pipelineRecord, pipelineRaw, parseErr = extractPipelineResultFromStream(stdoutLines)
		if parseErr == nil {
			record.Pipeline = pipelineRecord
		}
	} else {
		parsed, parseErr = result.ExtractFinalResultFromStream(stdoutLines)
		if parseErr == nil {
			agent := parsed.Agent
			record.AgentResult = &agent
			record.Normalized = parsed.Metrics
		}
	}

	switch {
	case runErr != nil && errors.Is(runErr, runner.ErrInterrupted):
		record.Status = stats.RunStatusError
		record.ErrorType = "interrupted"
		record.ErrorMessage = runErr.Error()
	case runErr != nil && runOutput.ExitCode == -1:
		record.Status = stats.RunStatusExecError
		record.ErrorType = "docker_exec_error"
		record.ErrorMessage = runErr.Error()
	case parseErr != nil:
		record.Status = stats.RunStatusParseError
		if isPlanRun {
			record.ErrorType = "pipeline_parse_error"
		} else {
			record.ErrorType = "parse_error"
		}
		record.ErrorMessage = parseErr.Error()
	case !isPlanRun && parsed != nil && parsed.Agent.IsError:
		record.Status = stats.RunStatusError
		record.ErrorType = "agent_error"
		record.ErrorMessage = "agent returned is_error=true"
	case isPlanRun && record.Pipeline != nil && record.Pipeline.IsError:
		record.Status = stats.RunStatusError
		record.ErrorType = "pipeline_error"
		record.ErrorMessage = "pipeline returned is_error=true"
	case runErr != nil:
		record.Status = stats.RunStatusError
		record.ErrorType = "docker_exit_error"
		record.ErrorMessage = runErr.Error()
	default:
		record.Status = stats.RunStatusSuccess
	}

	runsDir := config.RunsDir(cwd)
	savedPath, saveErr := stats.SaveRunRecord(runsDir, record)
	if saveErr != nil {
		return fmt.Errorf("save run statistics: %w", saveErr)
	}
	if err := stats.SaveRunArtifacts(filepath.Dir(savedPath), promptArtifactContent, runOutput.Stdout, runOutput.Stderr); err != nil {
		return fmt.Errorf("save run artifacts: %w", err)
	}

	if opts.JSONOutput && parseErr == nil {
		if isPlanRun {
			if strings.TrimSpace(pipelineRaw) != "" {
				fmt.Fprintln(runOutputWriter, pipelineRaw)
			}
		} else {
			fmt.Fprintln(runOutputWriter, string(parsed.Raw))
		}
	} else if !opts.JSONOutput {
		printRunSummary(record)
	}

	if runErr != nil {
		if errors.Is(runErr, runner.ErrInterrupted) {
			return runErr
		}
		if runOutput.ExitCode != -1 {
			return fmt.Errorf("docker exited with code %d", runOutput.ExitCode)
		}
		return runErr
	}
	if parseErr != nil {
		return parseErr
	}
	if !isPlanRun && parsed != nil && parsed.Agent.IsError {
		return errors.New("agent returned is_error=true")
	}
	if isPlanRun && record.Pipeline != nil && record.Pipeline.IsError {
		return errors.New("pipeline returned is_error=true")
	}

	return nil
}

func parseRunArgs(cwd string, args []string) (*runOptions, error) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var filePath string
	var pipelinePath string
	var jsonOutput bool
	var modelOverride string
	fs.StringVar(&filePath, "file", "", "path to file with prompt")
	fs.StringVar(&pipelinePath, "pipeline", "", "path to YAML pipeline plan file")
	fs.BoolVar(&jsonOutput, "json", false, "print raw JSON agent result")
	fs.StringVar(&modelOverride, "model", "", "model override (sonnet|opus)")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	modelOverride = strings.ToLower(strings.TrimSpace(modelOverride))
	if modelOverride != "" && !config.IsValidDockerModel(modelOverride) {
		return nil, fmt.Errorf(
			"invalid --model %q: expected %s or %s",
			modelOverride,
			config.DockerModelSonnet,
			config.DockerModelOpus,
		)
	}

	rest := fs.Args()
	pipelinePath = strings.TrimSpace(pipelinePath)
	if pipelinePath != "" {
		if strings.TrimSpace(filePath) != "" {
			return nil, errors.New("use either --pipeline or --file, not both")
		}
		if len(rest) > 0 {
			return nil, errors.New("pipeline mode does not accept positional prompt text")
		}

		pathForRecord := pipelinePath
		if !filepath.IsAbs(pathForRecord) {
			pathForRecord = filepath.Join(cwd, pathForRecord)
		}

		planBytes, err := os.ReadFile(pathForRecord)
		if err != nil {
			return nil, fmt.Errorf("read pipeline file %s: %w", pipelinePath, err)
		}
		planContent := strings.TrimSpace(string(planBytes))
		if planContent == "" {
			return nil, errors.New("pipeline file is empty")
		}

		return &runOptions{
			PromptPath:  pathForRecord,
			PromptFrom:  stats.PromptSourcePlanFile,
			Pipeline:    pathForRecord,
			PlanContent: planContent,
			JSONOutput:  jsonOutput,
			Model:       modelOverride,
		}, nil
	}

	if strings.TrimSpace(filePath) != "" && len(rest) > 0 {
		return nil, errors.New("use either positional prompt text or --file, not both")
	}

	if strings.TrimSpace(filePath) != "" {
		promptBytes, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("read prompt file %s: %w", filePath, err)
		}

		pathForRecord := filePath
		if !filepath.IsAbs(pathForRecord) {
			pathForRecord = filepath.Join(cwd, pathForRecord)
		}

		prompt := strings.TrimSpace(string(promptBytes))
		if prompt == "" {
			return nil, errors.New("prompt file is empty")
		}

		return &runOptions{
			Prompt:     prompt,
			PromptPath: pathForRecord,
			PromptFrom: stats.PromptSourceFile,
			JSONOutput: jsonOutput,
			Model:      modelOverride,
		}, nil
	}

	prompt := strings.TrimSpace(strings.Join(rest, " "))
	if prompt == "" {
		return nil, errors.New("prompt is empty: pass text or use --file")
	}

	return &runOptions{
		Prompt:     prompt,
		PromptFrom: stats.PromptSourceInline,
		JSONOutput: jsonOutput,
		Model:      modelOverride,
	}, nil
}

type pipelineResultEvent struct {
	Type            string                     `json:"type"`
	Version         string                     `json:"version"`
	Status          string                     `json:"status"`
	IsError         bool                       `json:"is_error"`
	StageCount      int                        `json:"stage_count"`
	CompletedStages int                        `json:"completed_stages"`
	TaskCount       int                        `json:"task_count"`
	FailedTaskCount int                        `json:"failed_task_count"`
	Tasks           []stats.PipelineTaskRecord `json:"tasks"`
}

func extractPipelineResultFromStream(lines []string) (*stats.PipelineRunRecord, string, error) {
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		var event pipelineResultEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		if event.Type != "pipeline_result" {
			continue
		}

		return &stats.PipelineRunRecord{
			Version:         event.Version,
			Status:          event.Status,
			IsError:         event.IsError,
			StageCount:      event.StageCount,
			CompletedStages: event.CompletedStages,
			TaskCount:       event.TaskCount,
			FailedTaskCount: event.FailedTaskCount,
			Tasks:           event.Tasks,
		}, line, nil
	}

	return nil, "", errors.New("pipeline result event not found in stream output")
}

func printRunSummary(record *stats.RunRecord) {
	fmt.Fprintf(runOutputWriter, "status: %s\n", record.Status)
	fmt.Fprintf(runOutputWriter, "input_tokens: %d\n", record.Normalized.InputTokens)
	fmt.Fprintf(runOutputWriter, "cache_creation_input_tokens: %d\n", record.Normalized.CacheCreationInputTokens)
	fmt.Fprintf(runOutputWriter, "cache_read_input_tokens: %d\n", record.Normalized.CacheReadInputTokens)
	fmt.Fprintf(runOutputWriter, "output_tokens: %d\n", record.Normalized.OutputTokens)
	totalTokens := record.Normalized.InputTokens +
		record.Normalized.CacheCreationInputTokens +
		record.Normalized.CacheReadInputTokens +
		record.Normalized.OutputTokens
	fmt.Fprintf(runOutputWriter, "total_tokens: %d\n", totalTokens)
}

func collectStreamMetrics(
	event *result.StreamEvent,
	metrics *stats.StreamMetrics,
	startedToolUses map[string]struct{},
	resolvedToolResults map[string]struct{},
	lastTodoStatuses map[string]string,
) {
	if event == nil {
		return
	}
	metrics.EnsureMaps()

	if event.Assistant != nil {
		for _, content := range event.Assistant.Content {
			if content.ToolUse == nil {
				continue
			}
			metrics.ToolUseTotal++
			toolName := strings.TrimSpace(content.ToolUse.Name)
			if toolName == "" {
				toolName = "unknown"
			}
			metrics.ToolUseByName[toolName]++
			startedToolUses[content.ToolUse.ID] = struct{}{}
		}
	}

	if event.User != nil {
		for _, item := range event.User.ToolResults {
			metrics.ToolResultTotal++
			if item.IsError {
				metrics.ToolResultErrorTotal++
			}
			if _, ok := startedToolUses[item.ToolUseID]; !ok {
				metrics.UnmatchedToolResultTotal++
				continue
			}
			resolvedToolResults[item.ToolUseID] = struct{}{}
		}

		transitions, completed := countTodoTransitions(
			event.User.ToolUseResult.OldTodos,
			event.User.ToolUseResult.NewTodos,
			lastTodoStatuses,
		)
		metrics.TodoTransitionTotal += transitions
		metrics.TodoCompletedTotal += completed
	}
}

func mergeNormalizedMetrics(target *result.NormalizedMetrics, delta result.NormalizedMetrics) {
	if target == nil {
		return
	}
	target.DurationMS += delta.DurationMS
	target.DurationAPIMS += delta.DurationAPIMS
	target.NumTurns += delta.NumTurns
	target.TotalCostUSD += delta.TotalCostUSD
	target.InputTokens += delta.InputTokens
	target.CacheCreationInputTokens += delta.CacheCreationInputTokens
	target.CacheReadInputTokens += delta.CacheReadInputTokens
	target.OutputTokens += delta.OutputTokens

	if target.ByModel == nil {
		target.ByModel = map[string]result.ModelMetric{}
	}
	for modelName, modelMetrics := range delta.ByModel {
		current := target.ByModel[modelName]
		current.InputTokens += modelMetrics.InputTokens
		current.OutputTokens += modelMetrics.OutputTokens
		current.CacheReadInputTokens += modelMetrics.CacheReadInputTokens
		current.CacheCreationInputTokens += modelMetrics.CacheCreationInputTokens
		current.WebSearchRequests += modelMetrics.WebSearchRequests
		current.CostUSD += modelMetrics.CostUSD
		target.ByModel[modelName] = current
	}
}

func countUnmatchedToolUses(started map[string]struct{}, resolved map[string]struct{}) int64 {
	var unmatched int64
	for id := range started {
		if _, ok := resolved[id]; !ok {
			unmatched++
		}
	}
	return unmatched
}

func countTodoTransitions(oldTodos, newTodos []result.TodoItem, last map[string]string) (int64, int64) {
	oldMap := map[string]string{}
	for _, item := range oldTodos {
		oldMap[item.Content] = item.Status
	}
	if len(oldMap) == 0 {
		for key, status := range last {
			oldMap[key] = status
		}
	}

	var transitions int64
	var completed int64

	for _, item := range newTodos {
		previous := oldMap[item.Content]
		if previous == "" {
			previous = "none"
		}
		if previous != item.Status {
			transitions++
			if item.Status == "completed" {
				completed++
			}
		}
		last[item.Content] = item.Status
	}

	return transitions, completed
}
