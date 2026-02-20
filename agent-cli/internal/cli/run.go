package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"agent-cli/internal/config"
	"agent-cli/internal/result"
	"agent-cli/internal/runner"
	"agent-cli/internal/stats"
)

type runOptions struct {
	Prompt       string
	Pipeline     string
	TemplateVars map[string]string
	JSONOutput   bool
	Model        string
	Debug        bool
}

var templateVarNamePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

type templateVarValues struct {
	values map[string]string
}

func (v *templateVarValues) String() string {
	if len(v.values) == 0 {
		return ""
	}

	keys := make([]string, 0, len(v.values))
	for key := range v.values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	pairs := make([]string, 0, len(keys))
	for _, key := range keys {
		pairs = append(pairs, key+"="+v.values[key])
	}
	return strings.Join(pairs, ",")
}

func (v *templateVarValues) Set(raw string) error {
	entry := strings.TrimSpace(raw)
	parts := strings.SplitN(entry, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid --var %q: expected KEY=VALUE", raw)
	}

	key := strings.TrimSpace(parts[0])
	if !templateVarNamePattern.MatchString(key) {
		return fmt.Errorf("invalid --var name %q: expected UPPER_SNAKE (^[A-Z][A-Z0-9_]*$)", key)
	}

	if v.values == nil {
		v.values = map[string]string{}
	}
	if _, exists := v.values[key]; exists {
		return fmt.Errorf("duplicate --var key %q", key)
	}

	v.values[key] = parts[1]
	return nil
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

	isPipelineRun := strings.TrimSpace(opts.Pipeline) != ""
	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	record := &stats.RunRecord{
		Timestamp: time.Now().UTC(),
		Status:    stats.RunStatusExecError,
		CWD:       cwd,
	}

	stdoutLines := make([]string, 0, 32)
	stderrLines := make([]string, 0, 16)
	streamMetrics := result.NormalizedMetrics{
		ByModel: map[string]result.ModelMetric{},
	}
	sessionTaskBindings := map[string]pipelineNodeRunRef{}
	taskUsageByKey := map[string]*stats.PipelineNodeRunNormalized{}
	taskUsagePendingBySession := map[string]*stats.PipelineNodeRunNormalized{}
	taskUsageSeen := map[string]bool{}

	var progressUI *ProgressTUI
	if !opts.JSONOutput {
		progressUI = NewProgressTUI(runCtx, runOutputWriter, os.Stdin, isPipelineRun, cancelRun)
		progressUI.Start()
	}
	progressClosed := false
	closeProgress := func() error {
		if progressUI == nil || progressClosed {
			return nil
		}
		progressClosed = true
		progressUI.Finish(record)
		return progressUI.Wait()
	}
	defer func() {
		if err := closeProgress(); err != nil {
			fmt.Fprintf(os.Stderr, "error: render progress ui: %v\n", err)
		}
	}()

	runOutput, runErr := runDockerStreamingFn(runCtx, runner.RunRequest{
		Image:                      cfg.Docker.Image,
		CWD:                        cwd,
		SourceWorkspaceDir:         cfg.Workspace.SourceWorkspaceDir,
		GitHubToken:                cfg.Auth.GitHubToken,
		ClaudeToken:                cfg.Auth.ClaudeToken,
		GitUserName:                cfg.Git.UserName,
		GitUserEmail:               cfg.Git.UserEmail,
		Prompt:                     opts.Prompt,
		Pipeline:                   opts.Pipeline,
		TemplateVars:               cloneTemplateVars(opts.TemplateVars),
		Model:                      model,
		Debug:                      opts.Debug,
		DockerMode:                 cfg.Docker.Mode,
		DinDStorageDriver:          cfg.Docker.DinDStorageDriver,
		RunIdleTimeoutSec:          cfg.Docker.RunIdleTimeoutSec,
		PipelineTaskIdleTimeoutSec: cfg.Docker.PipelineTaskIdleTimeoutSec,
	}, runner.StreamHooks{
		OnStdoutLine: func(line string) {
			stdoutLines = append(stdoutLines, line)
			event, kind, parseErr := result.ParseStreamLine(line)
			if parseErr != nil {
				if progressUI != nil {
					progressUI.SendRawLine("stdout", line)
				}
				return
			}

			if kind != result.StreamLineJSONEvent || event == nil {
				if progressUI != nil {
					progressUI.SendRawLine("stdout", line)
				}
				return
			}

			if event.Result != nil {
				mergeNormalizedMetrics(&streamMetrics, result.ExtractMetrics(*event.Result))
				if isPipelineRun {
					mergePipelineNodeRunUsageForResult(
						*event.Result,
						sessionTaskBindings,
						taskUsageByKey,
						taskUsagePendingBySession,
						taskUsageSeen,
					)
				}
			}

			if isPipelineRun {
				bindPipelineNodeRunSession(
					event,
					sessionTaskBindings,
					taskUsageByKey,
					taskUsagePendingBySession,
					taskUsageSeen,
				)
			}

			if progressUI != nil {
				progressUI.SendEvent(event)
			}
		},
		OnStderrLine: func(line string) {
			stderrLines = append(stderrLines, line)
			if progressUI != nil {
				progressUI.SendRawLine("stderr", line)
			}
		},
	})
	record.Normalized = streamMetrics
	record.DockerExitCode = runOutput.ExitCode

	var (
		parsed      *result.ParsedResult
		parseErr    error
		pipelineRaw string
	)
	if isPipelineRun {
		var pipelineRecord *stats.PipelineRunRecord
		pipelineRecord, pipelineRaw, parseErr = extractPipelineResultFromStream(stdoutLines, stderrLines)
		if parseErr == nil {
			applyPipelineNodeRunUsage(pipelineRecord, taskUsageByKey, taskUsageSeen)
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
	case runErr != nil && errors.Is(runErr, runner.ErrIdleTimeout):
		record.Status = stats.RunStatusError
		record.ErrorType = "timeout"
		record.ErrorMessage = runErr.Error()
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
		if isPipelineRun {
			record.ErrorType = "pipeline_parse_error"
		} else {
			record.ErrorType = "parse_error"
		}
		record.ErrorMessage = parseErr.Error()
	case !isPipelineRun && parsed != nil && parsed.Agent.IsError:
		record.Status = stats.RunStatusError
		record.ErrorType = "agent_error"
		record.ErrorMessage = "agent returned is_error=true"
	case isPipelineRun && record.Pipeline != nil && record.Pipeline.IsError:
		record.Status = stats.RunStatusError
		if pipelineFailureIsTimeout(record.Pipeline) {
			record.ErrorType = "pipeline_timeout"
		} else {
			record.ErrorType = "pipeline_error"
		}
		record.ErrorMessage = pipelineFailureMessage(record.Pipeline)
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
	if err := stats.SaveRunArtifacts(filepath.Dir(savedPath), runOutput.Stdout, runOutput.Stderr); err != nil {
		return fmt.Errorf("save run artifacts: %w", err)
	}

	if opts.JSONOutput && parseErr == nil {
		if isPipelineRun {
			if strings.TrimSpace(pipelineRaw) != "" {
				fmt.Fprintln(runOutputWriter, pipelineRaw)
			}
		} else {
			fmt.Fprintln(runOutputWriter, string(parsed.Raw))
		}
	}
	if !opts.JSONOutput {
		if err := closeProgress(); err != nil {
			return fmt.Errorf("render progress ui: %w", err)
		}
	}

	if runErr != nil && errors.Is(runErr, runner.ErrIdleTimeout) {
		return runErr
	}
	if runErr != nil && errors.Is(runErr, runner.ErrInterrupted) {
		return runErr
	}
	if runErr != nil && runOutput.ExitCode == -1 {
		return runErr
	}
	if parseErr != nil {
		return parseErr
	}
	if !isPipelineRun && parsed != nil && parsed.Agent.IsError {
		return errors.New("agent returned is_error=true")
	}
	if isPipelineRun && record.Pipeline != nil && record.Pipeline.IsError {
		message := strings.TrimSpace(record.ErrorMessage)
		if message == "" {
			message = pipelineFailureMessage(record.Pipeline)
		}
		return errors.New(message)
	}
	if runErr != nil {
		return fmt.Errorf("docker exited with code %d", runOutput.ExitCode)
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
	var debug bool
	var templateVars templateVarValues
	fs.StringVar(&filePath, "file", "", "path to file with prompt")
	fs.StringVar(&pipelinePath, "pipeline", "", "path to YAML pipeline plan file")
	fs.BoolVar(&jsonOutput, "json", false, "print raw JSON agent result")
	fs.StringVar(&modelOverride, "model", "", "model override (sonnet|opus)")
	fs.BoolVar(&debug, "debug", false, "enable debug logs in container entrypoint")
	fs.Var(&templateVars, "var", "template variable in KEY=VALUE format (repeatable, pipeline mode only)")

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
			Pipeline:     pathForRecord,
			TemplateVars: cloneTemplateVars(templateVars.values),
			JSONOutput:   jsonOutput,
			Model:        modelOverride,
			Debug:        debug,
		}, nil
	}

	if len(templateVars.values) > 0 {
		return nil, errors.New("--var is only supported with --pipeline")
	}

	if strings.TrimSpace(filePath) != "" && len(rest) > 0 {
		return nil, errors.New("use either positional prompt text or --file, not both")
	}

	if strings.TrimSpace(filePath) != "" {
		promptBytes, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("read prompt file %s: %w", filePath, err)
		}

		prompt := strings.TrimSpace(string(promptBytes))
		if prompt == "" {
			return nil, errors.New("prompt file is empty")
		}

		return &runOptions{
			Prompt:     prompt,
			JSONOutput: jsonOutput,
			Model:      modelOverride,
			Debug:      debug,
		}, nil
	}

	prompt := strings.TrimSpace(strings.Join(rest, " "))
	if prompt == "" {
		return nil, errors.New("prompt is empty: pass text or use --file")
	}

	return &runOptions{
		Prompt:     prompt,
		JSONOutput: jsonOutput,
		Model:      modelOverride,
		Debug:      debug,
	}, nil
}

func cloneTemplateVars(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}

	cloned := make(map[string]string, len(input))
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}

type pipelineResultEvent struct {
	Type            string                        `json:"type"`
	Version         string                        `json:"version"`
	Status          string                        `json:"status"`
	IsError         bool                          `json:"is_error"`
	EntryNode       string                        `json:"entry_node"`
	TerminalNode    string                        `json:"terminal_node"`
	TerminalStatus  string                        `json:"terminal_status"`
	ExitCode        int                           `json:"exit_code"`
	Iterations      int                           `json:"iterations"`
	NodeRunCount    int                           `json:"node_run_count"`
	FailedNodeCount int                           `json:"failed_node_count"`
	NodeRuns        []stats.PipelineNodeRunRecord `json:"node_runs"`
}

func extractPipelineResultFromStream(
	stdoutLines []string,
	stderrLines []string,
) (*stats.PipelineRunRecord, string, error) {
	for i := len(stdoutLines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(stdoutLines[i])
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
			EntryNode:       event.EntryNode,
			TerminalNode:    event.TerminalNode,
			TerminalStatus:  event.TerminalStatus,
			ExitCode:        event.ExitCode,
			Iterations:      event.Iterations,
			NodeRunCount:    event.NodeRunCount,
			FailedNodeCount: event.FailedNodeCount,
			NodeRuns:        event.NodeRuns,
		}, line, nil
	}

	if message := extractPipelineFailureMessage(stdoutLines, stderrLines); message != "" {
		return nil, "", errors.New(message)
	}

	return nil, "", errors.New("pipeline result event not found in stream output")
}

func extractPipelineFailureMessage(lineSets ...[]string) string {
	fallback := ""
	for _, lines := range lineSets {
		for i := len(lines) - 1; i >= 0; i-- {
			line := strings.TrimSpace(lines[i])
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, "{") {
				var payload map[string]any
				if err := json.Unmarshal([]byte(line), &payload); err == nil {
					// Skip machine-readable JSON lines and prefer explicit human-readable failures.
					continue
				}
			}
			if strings.HasPrefix(line, "Entrypoint failed:") {
				return line
			}
			if fallback == "" {
				fallback = line
			}
		}
	}

	return fallback
}

func pipelineFailureMessage(pipeline *stats.PipelineRunRecord) string {
	if pipeline == nil {
		return "pipeline returned is_error=true"
	}

	for _, nodeRun := range pipeline.NodeRuns {
		if !strings.EqualFold(strings.TrimSpace(nodeRun.Status), "error") {
			continue
		}

		detail := strings.TrimSpace(nodeRun.ErrorMessage)
		if detail == "" {
			detail = fmt.Sprintf("node exited with code %d", nodeRun.ExitCode)
		}
		nodeID := strings.TrimSpace(nodeRun.NodeID)
		nodeRunID := strings.TrimSpace(nodeRun.NodeRunID)
		if nodeID != "" && nodeRunID != "" {
			return fmt.Sprintf("pipeline failed at %s/%s: %s", nodeID, nodeRunID, detail)
		}
		return "pipeline failed: " + detail
	}

	return "pipeline returned is_error=true"
}

func pipelineFailureIsTimeout(pipeline *stats.PipelineRunRecord) bool {
	if pipeline == nil {
		return false
	}

	for _, nodeRun := range pipeline.NodeRuns {
		if !strings.EqualFold(strings.TrimSpace(nodeRun.Status), "error") {
			continue
		}
		message := strings.ToLower(strings.TrimSpace(nodeRun.ErrorMessage))
		if strings.Contains(message, "idle timeout") || strings.Contains(message, "timed out") {
			return true
		}
	}

	return false
}

func runSummaryLines(record *stats.RunRecord) []string {
	totalTokens := record.Normalized.InputTokens +
		record.Normalized.CacheCreationInputTokens +
		record.Normalized.CacheReadInputTokens +
		record.Normalized.OutputTokens

	headers := runStatsTableHeaders()
	rows := [][]string{
		{
			simpleRunStepName,
			string(record.Status),
			fmt.Sprintf("%d", record.Normalized.InputTokens),
			fmt.Sprintf("%d", record.Normalized.CacheCreationInputTokens),
			fmt.Sprintf("%d", record.Normalized.CacheReadInputTokens),
			fmt.Sprintf("%d", record.Normalized.OutputTokens),
			fmt.Sprintf("%d", totalTokens),
			formatCostUSD(record.Normalized.TotalCostUSD),
		},
	}
	return renderTextTable(headers, rows)
}

const simpleRunStepName = "run/prompt"

func runStatsTableHeaders() []string {
	return []string{
		"STEP",
		"STATUS",
		"INPUT_TOKENS",
		"CACHE_CREATE",
		"CACHE_READ",
		"OUTPUT_TOKENS",
		"TOTAL_TOKENS",
		"COST_USD",
	}
}

func formatCostUSD(value float64) string {
	return fmt.Sprintf("%.6f", value)
}

func formatStepName(stageID string, taskID string) string {
	normalizedStageID := strings.TrimSpace(stageID)
	normalizedTaskID := strings.TrimSpace(taskID)
	switch {
	case normalizedStageID != "" && normalizedTaskID != "":
		return normalizedStageID + "/" + normalizedTaskID
	case normalizedStageID != "":
		return normalizedStageID
	case normalizedTaskID != "":
		return normalizedTaskID
	default:
		return "step"
	}
}

type pipelineNodeRunRef struct {
	NodeID    string
	NodeRunID string
}

func bindPipelineNodeRunSession(
	event *result.StreamEvent,
	sessionTaskBindings map[string]pipelineNodeRunRef,
	taskUsageByKey map[string]*stats.PipelineNodeRunNormalized,
	taskUsagePendingBySession map[string]*stats.PipelineNodeRunNormalized,
	taskUsageSeen map[string]bool,
) {
	if event == nil || event.Pipeline == nil {
		return
	}
	if !strings.EqualFold(strings.TrimSpace(event.Pipeline.Event), "node_session_bind") {
		return
	}

	sessionID := strings.TrimSpace(event.Pipeline.SessionID)
	nodeID := strings.TrimSpace(event.Pipeline.NodeID)
	nodeRunID := strings.TrimSpace(event.Pipeline.NodeRunID)
	if sessionID == "" || nodeID == "" || nodeRunID == "" {
		return
	}

	ref := pipelineNodeRunRef{
		NodeID:    nodeID,
		NodeRunID: nodeRunID,
	}
	sessionTaskBindings[sessionID] = ref

	pending := taskUsagePendingBySession[sessionID]
	if pending == nil {
		return
	}
	key := buildPipelineNodeRunKey(nodeID, nodeRunID)
	usage := taskUsageByKey[key]
	if usage == nil {
		value := newPipelineNodeRunNormalized()
		usage = &value
		taskUsageByKey[key] = usage
	}
	mergePipelineNodeRunNormalized(usage, *pending)
	taskUsageSeen[key] = true
	delete(taskUsagePendingBySession, sessionID)
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

func mergePipelineNodeRunUsageForResult(
	agent result.AgentResult,
	sessionTaskBindings map[string]pipelineNodeRunRef,
	taskUsageByKey map[string]*stats.PipelineNodeRunNormalized,
	taskUsagePendingBySession map[string]*stats.PipelineNodeRunNormalized,
	taskUsageSeen map[string]bool,
) {
	sessionID := strings.TrimSpace(agent.SessionID)
	if sessionID == "" {
		return
	}

	delta := extractPipelineNodeRunNormalized(agent)

	ref, bound := sessionTaskBindings[sessionID]
	if !bound || strings.TrimSpace(ref.NodeID) == "" || strings.TrimSpace(ref.NodeRunID) == "" {
		pending := taskUsagePendingBySession[sessionID]
		if pending == nil {
			value := newPipelineNodeRunNormalized()
			pending = &value
			taskUsagePendingBySession[sessionID] = pending
		}
		mergePipelineNodeRunNormalized(pending, delta)
		return
	}

	key := buildPipelineNodeRunKey(ref.NodeID, ref.NodeRunID)
	usage := taskUsageByKey[key]
	if usage == nil {
		value := newPipelineNodeRunNormalized()
		usage = &value
		taskUsageByKey[key] = usage
	}
	mergePipelineNodeRunNormalized(usage, delta)
	taskUsageSeen[key] = true
}

func applyPipelineNodeRunUsage(
	pipeline *stats.PipelineRunRecord,
	taskUsageByKey map[string]*stats.PipelineNodeRunNormalized,
	taskUsageSeen map[string]bool,
) {
	if pipeline == nil {
		return
	}

	for i := range pipeline.NodeRuns {
		nodeRun := &pipeline.NodeRuns[i]
		key := buildPipelineNodeRunKey(nodeRun.NodeID, nodeRun.NodeRunID)
		if !taskUsageSeen[key] {
			nodeRun.Normalized = nil
			continue
		}

		usage := taskUsageByKey[key]
		if usage == nil {
			value := newPipelineNodeRunNormalized()
			nodeRun.Normalized = &value
			continue
		}
		value := clonePipelineNodeRunNormalized(*usage)
		nodeRun.Normalized = &value
	}
}

func extractPipelineNodeRunNormalized(agent result.AgentResult) stats.PipelineNodeRunNormalized {
	normalized := newPipelineNodeRunNormalized()
	normalized.InputTokens = agent.Usage.InputTokens
	normalized.CacheCreationInputTokens = agent.Usage.CacheCreationInputTokens
	normalized.CacheReadInputTokens = agent.Usage.CacheReadInputTokens
	normalized.OutputTokens = agent.Usage.OutputTokens
	normalized.CostUSD = agent.TotalCostUSD
	normalized.WebSearchRequests = agent.Usage.ServerToolUse.WebSearchRequests

	for modelName, usage := range agent.ModelUsage {
		normalized.ByModel[modelName] = stats.PipelineNodeRunModelMetric{
			InputTokens:              usage.InputTokens,
			OutputTokens:             usage.OutputTokens,
			CacheReadInputTokens:     usage.CacheReadInputTokens,
			CacheCreationInputTokens: usage.CacheCreationInputTokens,
			CostUSD:                  usage.CostUSD,
			WebSearchRequests:        usage.WebSearchRequests,
		}
	}
	return normalized
}

func mergePipelineNodeRunNormalized(target *stats.PipelineNodeRunNormalized, delta stats.PipelineNodeRunNormalized) {
	if target == nil {
		return
	}

	target.InputTokens += delta.InputTokens
	target.CacheCreationInputTokens += delta.CacheCreationInputTokens
	target.CacheReadInputTokens += delta.CacheReadInputTokens
	target.OutputTokens += delta.OutputTokens
	target.CostUSD += delta.CostUSD
	target.WebSearchRequests += delta.WebSearchRequests

	if target.ByModel == nil {
		target.ByModel = map[string]stats.PipelineNodeRunModelMetric{}
	}
	for modelName, metric := range delta.ByModel {
		current := target.ByModel[modelName]
		current.InputTokens += metric.InputTokens
		current.OutputTokens += metric.OutputTokens
		current.CacheReadInputTokens += metric.CacheReadInputTokens
		current.CacheCreationInputTokens += metric.CacheCreationInputTokens
		current.CostUSD += metric.CostUSD
		current.WebSearchRequests += metric.WebSearchRequests
		target.ByModel[modelName] = current
	}
}

func clonePipelineNodeRunNormalized(source stats.PipelineNodeRunNormalized) stats.PipelineNodeRunNormalized {
	cloned := stats.PipelineNodeRunNormalized{
		InputTokens:              source.InputTokens,
		CacheCreationInputTokens: source.CacheCreationInputTokens,
		CacheReadInputTokens:     source.CacheReadInputTokens,
		OutputTokens:             source.OutputTokens,
		CostUSD:                  source.CostUSD,
		WebSearchRequests:        source.WebSearchRequests,
		ByModel:                  map[string]stats.PipelineNodeRunModelMetric{},
	}
	for modelName, metric := range source.ByModel {
		cloned.ByModel[modelName] = metric
	}
	return cloned
}

func newPipelineNodeRunNormalized() stats.PipelineNodeRunNormalized {
	return stats.PipelineNodeRunNormalized{
		ByModel: map[string]stats.PipelineNodeRunModelMetric{},
	}
}

func buildPipelineNodeRunKey(nodeID string, nodeRunID string) string {
	return strings.TrimSpace(nodeID) + "\x00" + strings.TrimSpace(nodeRunID)
}
