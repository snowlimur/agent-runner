package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"time"

	"agent-cli/internal/result"
	"agent-cli/internal/stats"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	ansiGreen            = "\x1b[32m"
	ansiReset            = "\x1b[0m"
	maxContainerLogLines = 80
)

type ProgressTUI struct {
	program  *tea.Program
	doneCh   chan error
	finished atomic.Bool
}

type streamEventMsg struct {
	Event *result.StreamEvent
}

type rawLogLineMsg struct {
	Source string
	Line   string
}

type runFinishedMsg struct {
	Record *stats.RunRecord
}

func NewProgressTUI(
	ctx context.Context,
	output io.Writer,
	input io.Reader,
	pipelineHint bool,
	cancelRun context.CancelFunc,
) *ProgressTUI {
	model := newProgressTUIModel(pipelineHint, cancelRun)
	program := tea.NewProgram(
		model,
		tea.WithContext(ctx),
		tea.WithInput(input),
		tea.WithOutput(output),
		tea.WithoutSignals(),
	)

	return &ProgressTUI{
		program: program,
		doneCh:  make(chan error, 1),
	}
}

func (p *ProgressTUI) Start() {
	go func() {
		_, err := p.program.Run()
		p.doneCh <- err
	}()
}

func (p *ProgressTUI) SendEvent(event *result.StreamEvent) {
	if event == nil {
		return
	}
	p.program.Send(streamEventMsg{Event: event})
}

func (p *ProgressTUI) SendRawLine(source string, line string) {
	if strings.TrimSpace(line) == "" {
		return
	}
	p.program.Send(rawLogLineMsg{
		Source: source,
		Line:   line,
	})
}

func (p *ProgressTUI) Finish(record *stats.RunRecord) {
	if !p.finished.CompareAndSwap(false, true) {
		return
	}
	p.program.Send(runFinishedMsg{Record: record})
}

func (p *ProgressTUI) Wait() error {
	return <-p.doneCh
}

type progressTUIModel struct {
	isPipeline      bool
	pipelineStarted bool
	expanded        bool
	interrupting    bool
	done            bool

	stageCount int
	planStatus string

	stageOrder []string
	stages     map[string]*pipelineStageState

	sessionTask            map[string]taskRef
	toolTaskByKey          map[string]taskRef
	toolUseIDByToolKey     map[string]string
	pendingOutcomeBySessID map[string]taskOutcome
	containerLogLines      []string

	finalRecord *stats.RunRecord
	cancelRun   context.CancelFunc
}

type pipelineStageState struct {
	ID             string
	Mode           string
	Status         string
	TaskCount      int
	CompletedTasks int
	FailedTasks    int
	DurationMS     int64
	TaskOrder      []string
	Tasks          map[string]*pipelineTaskState
}

type pipelineTaskState struct {
	StageID string
	TaskID  string

	Status     string
	Model      string
	Verbosity  string
	Workspace  string
	ErrorText  string
	ResultText string
	DurationMS int64

	ToolUses        int
	Tokens          int64
	CacheReadTokens int64
	CostUSD         float64

	Started bool
	Done    bool

	ActiveSteps map[string]activeStep
	StepOrder   []string
}

type activeStep struct {
	ToolUseID string
	Name      string
	Summary   string
}

type taskOutcome struct {
	Tokens    int64
	CacheRead int64
	Cost      float64
	Result    string
}

func newProgressTUIModel(pipelineHint bool, cancelRun context.CancelFunc) progressTUIModel {
	model := progressTUIModel{
		isPipeline:             pipelineHint,
		stageOrder:             make([]string, 0, 8),
		stages:                 map[string]*pipelineStageState{},
		sessionTask:            map[string]taskRef{},
		toolTaskByKey:          map[string]taskRef{},
		toolUseIDByToolKey:     map[string]string{},
		pendingOutcomeBySessID: map[string]taskOutcome{},
		containerLogLines:      make([]string, 0, 8),
		cancelRun:              cancelRun,
	}

	if !pipelineHint {
		model.ensureSimpleRunTask("")
	}

	return model
}

func (m progressTUIModel) Init() tea.Cmd {
	return nil
}

func (m progressTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.KeyMsg:
		switch typed.String() {
		case "ctrl+o":
			m.expanded = !m.expanded
		case "ctrl+c":
			return m, m.interruptRun()
		}
	case tea.InterruptMsg:
		return m, m.interruptRun()
	case streamEventMsg:
		m.handleStreamEvent(typed.Event)
	case rawLogLineMsg:
		m.appendContainerLogLine(typed.Source, typed.Line)
	case runFinishedMsg:
		m.finalRecord = typed.Record
		if typed.Record != nil {
			if typed.Record.Pipeline == nil {
				stage := m.ensureStage("run")
				task := m.ensureSimpleRunTask("")
				task.Done = true
				task.Status = normalizeStatus(string(typed.Record.Status), "success")
				if typed.Record.AgentResult != nil {
					task.ResultText = mergeResultText(task.ResultText, strings.TrimSpace(typed.Record.AgentResult.Result))
				}
				if strings.TrimSpace(task.ResultText) == "" && strings.EqualFold(task.Status, "success") {
					task.ResultText = "Done"
				}
				if strings.TrimSpace(task.ErrorText) == "" {
					task.ErrorText = strings.TrimSpace(typed.Record.ErrorMessage)
				}
				task.ActiveSteps = map[string]activeStep{}
				task.StepOrder = task.StepOrder[:0]
				stage.Status = task.Status
				stage.CompletedTasks = stageTotalCount(stage)
				if strings.EqualFold(task.Status, "error") {
					stage.FailedTasks = 1
				}
			} else if strings.TrimSpace(m.planStatus) == "" {
				m.planStatus = normalizeStatus(typed.Record.Pipeline.Status, "success")
				if m.stageCount == 0 {
					m.stageCount = typed.Record.Pipeline.StageCount
				}
			}
		}
		m.done = true
		return m, tea.Quit
	}

	return m, nil
}

func (m *progressTUIModel) interruptRun() tea.Cmd {
	if !m.interrupting && !m.done {
		m.interrupting = true
		if m.cancelRun != nil {
			m.cancelRun()
		}
	}
	return tea.Quit
}

func (m progressTUIModel) View() string {
	lines := make([]string, 0, 128)

	if m.isPipeline || m.pipelineStarted {
		lines = append(lines, m.renderPipelineHeader())
	} else {
		lines = append(lines, m.renderRunHeader())
	}

	lines = append(lines, m.renderTree()...)

	if len(m.containerLogLines) > 0 {
		lines = append(lines, "")
		lines = append(lines, "Container logs (non-JSON)")
		for _, line := range m.containerLogLines {
			lines = append(lines, "  "+line)
		}
	}

	if m.done && m.finalRecord != nil && m.finalRecord.Pipeline != nil {
		lines = append(lines, "")
		lines = append(lines, "Pipeline Task Stats")
		lines = append(lines, m.renderPipelineStatsTable()...)
	}

	if m.done && m.finalRecord != nil && m.finalRecord.Pipeline == nil {
		lines = append(lines, "")
		lines = append(lines, runSummaryLines(m.finalRecord)...)
	}

	return strings.Join(lines, "\n") + "\n"
}

func (m *progressTUIModel) handleStreamEvent(event *result.StreamEvent) {
	if event == nil {
		return
	}

	if event.Pipeline != nil {
		m.handlePipelineEvent(event.Pipeline)
	}
	if event.System != nil {
		m.handleSystemEvent(event.System)
	}
	if event.Assistant != nil {
		m.handleAssistantEvent(event.Assistant)
	}
	if event.User != nil {
		m.handleUserEvent(event.User)
	}
	if event.Result != nil {
		m.handleResultEvent(event.Result)
	}
}

func (m *progressTUIModel) appendContainerLogLine(source string, line string) {
	trimmedLine := strings.TrimSpace(strings.TrimRight(line, "\r"))
	if trimmedLine == "" {
		return
	}

	normalizedSource := strings.ToLower(strings.TrimSpace(source))
	if normalizedSource == "" {
		normalizedSource = "stdout"
	}

	formatted := fmt.Sprintf("[%s] %s", normalizedSource, trimmedLine)
	m.containerLogLines = append(m.containerLogLines, formatted)
	if len(m.containerLogLines) > maxContainerLogLines {
		m.containerLogLines = append(
			[]string(nil),
			m.containerLogLines[len(m.containerLogLines)-maxContainerLogLines:]...,
		)
	}
}

func (m *progressTUIModel) handlePipelineEvent(event *result.PipelineEvent) {
	if event == nil {
		return
	}
	m.isPipeline = true
	m.pipelineStarted = true

	eventName := strings.ToLower(strings.TrimSpace(event.Event))
	switch eventName {
	case "plan_start":
		if event.StageCount > 0 {
			m.stageCount = event.StageCount
		}
	case "stage_start":
		stage := m.ensureStage(event.StageID)
		stage.Mode = strings.TrimSpace(event.Mode)
		stage.Status = "running"
		if event.TaskCount > 0 {
			stage.TaskCount = event.TaskCount
		}
	case "task_start":
		task := m.ensureTask(event.StageID, event.TaskID)
		task.Started = true
		task.Status = "running"
		if strings.TrimSpace(event.Model) != "" {
			task.Model = strings.TrimSpace(event.Model)
		}
		if strings.TrimSpace(event.Verbosity) != "" {
			task.Verbosity = strings.TrimSpace(event.Verbosity)
		}
		if strings.TrimSpace(event.Workspace) != "" {
			task.Workspace = strings.TrimSpace(event.Workspace)
		}
		stage := m.ensureStage(event.StageID)
		if event.TaskCount > stage.TaskCount {
			stage.TaskCount = event.TaskCount
		}
	case "task_session_bind":
		sessionID := strings.TrimSpace(event.SessionID)
		stageID := strings.TrimSpace(event.StageID)
		taskID := strings.TrimSpace(event.TaskID)
		if sessionID == "" || stageID == "" || taskID == "" {
			return
		}
		task := m.ensureTask(stageID, taskID)
		task.Started = true
		m.sessionTask[sessionID] = taskRef{
			StageID: stageID,
			TaskID:  taskID,
		}
		m.applyPendingOutcome(sessionID, task)
	case "task_timeout":
		task := m.ensureTask(event.StageID, event.TaskID)
		task.Started = true
		reason := strings.TrimSpace(event.Reason)
		if reason == "" && event.IdleTimeoutSec > 0 {
			reason = fmt.Sprintf("idle timeout after %ds without task output", event.IdleTimeoutSec)
		}
		if reason != "" {
			task.ErrorText = reason
		}
	case "task_finish":
		task := m.ensureTask(event.StageID, event.TaskID)
		task.Started = true
		task.Done = true
		task.Status = normalizeStatus(event.Status, "success")
		if strings.TrimSpace(event.Model) != "" {
			task.Model = strings.TrimSpace(event.Model)
		}
		if strings.TrimSpace(event.Verbosity) != "" {
			task.Verbosity = strings.TrimSpace(event.Verbosity)
		}
		if strings.TrimSpace(event.Workspace) != "" {
			task.Workspace = strings.TrimSpace(event.Workspace)
		}
		if event.DurationMS > 0 {
			task.DurationMS = event.DurationMS
		}
		if strings.TrimSpace(event.ErrorMessage) != "" {
			task.ErrorText = strings.TrimSpace(event.ErrorMessage)
		}
		task.ActiveSteps = map[string]activeStep{}
		task.StepOrder = task.StepOrder[:0]
	case "stage_finish":
		stage := m.ensureStage(event.StageID)
		stage.Status = normalizeStatus(event.Status, "success")
		stage.CompletedTasks = event.CompletedTasks
		stage.FailedTasks = event.FailedTasks
		stage.DurationMS = event.DurationMS
		if event.TaskCount > 0 {
			stage.TaskCount = event.TaskCount
		}
	case "plan_finish":
		m.planStatus = normalizeStatus(event.Status, "success")
		if event.StageCount > 0 {
			m.stageCount = event.StageCount
		}
	}
}

func (m *progressTUIModel) handleSystemEvent(event *result.SystemEvent) {
	if event == nil {
		return
	}
	if !strings.EqualFold(strings.TrimSpace(event.Subtype), "init") {
		return
	}

	if m.isPipeline || m.pipelineStarted {
		return
	}

	task := m.ensureSimpleRunTask(event.SessionID)
	if strings.TrimSpace(event.Model) != "" {
		task.Model = strings.TrimSpace(event.Model)
	}
}

func (m *progressTUIModel) handleAssistantEvent(event *result.AssistantEvent) {
	if event == nil {
		return
	}

	sessionID := strings.TrimSpace(event.SessionID)
	task := m.resolveTaskForSession(sessionID, true)
	if task == nil {
		return
	}

	for _, item := range event.Content {
		if item.ToolUse == nil {
			continue
		}

		toolUseID := strings.TrimSpace(item.ToolUse.ID)
		toolName := strings.TrimSpace(item.ToolUse.Name)
		if toolName == "" {
			toolName = "tool"
		}

		summary := strings.TrimSpace(item.ToolUse.Input.Command)
		if summary == "" {
			summary = strings.TrimSpace(item.ToolUse.Input.Description)
		}
		if summary == "" {
			summary = "running"
		}

		task.ToolUses++
		task.Started = true
		task.addActiveStep(toolUseID, toolName, summary)

		toolKey := buildToolKey(sessionID, toolUseID)
		m.toolTaskByKey[toolKey] = taskRef{
			StageID: task.StageID,
			TaskID:  task.TaskID,
		}
		m.toolUseIDByToolKey[toolKey] = toolUseID
	}
}

func (m *progressTUIModel) handleUserEvent(event *result.UserEvent) {
	if event == nil {
		return
	}

	sessionID := strings.TrimSpace(event.SessionID)
	for _, item := range event.ToolResults {
		toolUseID := strings.TrimSpace(item.ToolUseID)
		toolKey := buildToolKey(sessionID, toolUseID)
		ref, ok := m.toolTaskByKey[toolKey]
		if !ok && sessionID == "" {
			foundKey, foundRef, found := m.findToolByUseID(toolUseID)
			if found {
				toolKey = foundKey
				ref = foundRef
				ok = true
			}
		}
		if !ok {
			task := m.resolveTaskForSession(sessionID, false)
			if task != nil {
				task.removeActiveStep(toolUseID)
			}
			continue
		}

		task := m.ensureTask(ref.StageID, ref.TaskID)
		task.removeActiveStep(m.toolUseIDByToolKey[toolKey])
		delete(m.toolTaskByKey, toolKey)
		delete(m.toolUseIDByToolKey, toolKey)
	}
}

func (m *progressTUIModel) handleResultEvent(event *result.AgentResult) {
	if event == nil {
		return
	}

	deltaTokens := event.Usage.InputTokens +
		event.Usage.CacheCreationInputTokens +
		event.Usage.CacheReadInputTokens +
		event.Usage.OutputTokens
	deltaCacheRead := event.Usage.CacheReadInputTokens
	deltaCost := event.TotalCostUSD
	resultText := strings.TrimSpace(event.Result)
	sessionID := strings.TrimSpace(event.SessionID)

	if task := m.resolveTaskForSession(sessionID, false); task != nil {
		applyTaskOutcome(task, deltaTokens, deltaCacheRead, deltaCost, resultText)
		return
	}

	if m.isPipeline || m.pipelineStarted {
		pending := m.pendingOutcomeBySessID[sessionID]
		pending.Tokens += deltaTokens
		pending.CacheRead += deltaCacheRead
		pending.Cost += deltaCost
		pending.Result = mergeResultText(pending.Result, resultText)
		m.pendingOutcomeBySessID[sessionID] = pending
		return
	}

	task := m.resolveTaskForSession(sessionID, true)
	if task == nil {
		return
	}
	applyTaskOutcome(task, deltaTokens, deltaCacheRead, deltaCost, resultText)
}

func (m *progressTUIModel) ensureSimpleRunTask(sessionID string) *pipelineTaskState {
	stage := m.ensureStage("run")
	stage.Status = normalizeStatus(stage.Status, "running")
	if stage.TaskCount == 0 {
		stage.TaskCount = 1
	}
	task := m.ensureTask(stage.ID, "prompt")
	task.Started = true

	normalizedSession := strings.TrimSpace(sessionID)
	if normalizedSession != "" {
		m.sessionTask[normalizedSession] = taskRef{
			StageID: stage.ID,
			TaskID:  task.TaskID,
		}
	}

	return task
}

func (m *progressTUIModel) ensureStage(stageID string) *pipelineStageState {
	normalizedStageID := strings.TrimSpace(stageID)
	if normalizedStageID == "" {
		normalizedStageID = "stage"
	}

	stage := m.stages[normalizedStageID]
	if stage == nil {
		stage = &pipelineStageState{
			ID:        normalizedStageID,
			TaskOrder: make([]string, 0, 8),
			Tasks:     map[string]*pipelineTaskState{},
		}
		m.stages[normalizedStageID] = stage
		m.stageOrder = append(m.stageOrder, normalizedStageID)
	}
	return stage
}

func (m *progressTUIModel) ensureTask(stageID string, taskID string) *pipelineTaskState {
	stage := m.ensureStage(stageID)
	normalizedTaskID := strings.TrimSpace(taskID)
	if normalizedTaskID == "" {
		normalizedTaskID = "task"
	}

	task := stage.Tasks[normalizedTaskID]
	if task == nil {
		task = &pipelineTaskState{
			StageID:     stage.ID,
			TaskID:      normalizedTaskID,
			Status:      "pending",
			ActiveSteps: map[string]activeStep{},
			StepOrder:   make([]string, 0, 8),
		}
		stage.Tasks[normalizedTaskID] = task
		stage.TaskOrder = append(stage.TaskOrder, normalizedTaskID)
		if len(stage.TaskOrder) > stage.TaskCount {
			stage.TaskCount = len(stage.TaskOrder)
		}
	}
	return task
}

func (m *progressTUIModel) resolveTaskForSession(sessionID string, create bool) *pipelineTaskState {
	normalizedSessionID := strings.TrimSpace(sessionID)
	if normalizedSessionID != "" {
		if ref, ok := m.sessionTask[normalizedSessionID]; ok && ref.isBound() {
			return m.ensureTask(ref.StageID, ref.TaskID)
		}
	}

	if m.isPipeline || m.pipelineStarted {
		return nil
	}
	if !create {
		return nil
	}
	return m.ensureSimpleRunTask(normalizedSessionID)
}

func (m *progressTUIModel) applyPendingOutcome(sessionID string, task *pipelineTaskState) {
	normalizedSessionID := strings.TrimSpace(sessionID)
	if normalizedSessionID == "" || task == nil {
		return
	}
	pending, ok := m.pendingOutcomeBySessID[normalizedSessionID]
	if !ok {
		return
	}
	applyTaskOutcome(task, pending.Tokens, pending.CacheRead, pending.Cost, pending.Result)
	delete(m.pendingOutcomeBySessID, normalizedSessionID)
}

func (m *progressTUIModel) findToolByUseID(toolUseID string) (string, taskRef, bool) {
	normalizedToolUseID := strings.TrimSpace(toolUseID)
	if normalizedToolUseID == "" {
		return "", taskRef{}, false
	}

	for toolKey, candidateToolUseID := range m.toolUseIDByToolKey {
		if strings.TrimSpace(candidateToolUseID) != normalizedToolUseID {
			continue
		}
		ref := m.toolTaskByKey[toolKey]
		return toolKey, ref, true
	}
	return "", taskRef{}, false
}

func (m *progressTUIModel) renderPipelineHeader() string {
	action := "collapse"
	if !m.expanded {
		action = "expand"
	}
	if m.interrupting && !m.done {
		return fmt.Sprintf("Interrupting pipeline... (ctrl+o to %s)", action)
	}

	if m.done {
		status := "completed"
		if m.finalRecord != nil && m.finalRecord.Status != stats.RunStatusSuccess {
			status = "finished with errors"
		}
		return fmt.Sprintf("Pipeline %s (%d stages) (ctrl+o to %s)", status, m.effectiveStageCount(), action)
	}

	return fmt.Sprintf("Running pipeline (%d stages) (ctrl+o to %s)", m.effectiveStageCount(), action)
}

func (m *progressTUIModel) renderRunHeader() string {
	action := "collapse"
	if !m.expanded {
		action = "expand"
	}
	if m.interrupting && !m.done {
		return fmt.Sprintf("Interrupting run... (ctrl+o to %s)", action)
	}
	if m.done {
		return fmt.Sprintf("Run completed (ctrl+o to %s)", action)
	}
	return fmt.Sprintf("Running agent... (ctrl+o to %s)", action)
}

func (m *progressTUIModel) renderTree() []string {
	if len(m.stageOrder) == 0 {
		return []string{"└─ Waiting for events..."}
	}

	lines := make([]string, 0, 128)
	for stageIndex, stageID := range m.stageOrder {
		stage := m.stages[stageID]
		if stage == nil {
			continue
		}

		isLastStage := stageIndex == len(m.stageOrder)-1
		stageBranch := "├─"
		stageIndent := "│  "
		if isLastStage {
			stageBranch = "└─"
			stageIndent = "   "
		}

		stageLine := fmt.Sprintf(
			"%s %s · %s · %d/%d tasks",
			stageBranch,
			stage.ID,
			stageDisplayStatus(stage),
			stageCompletedCount(stage),
			stageTotalCount(stage),
		)
		lines = append(lines, stageLine)

		taskOrder := stage.TaskOrder
		for taskIndex, taskID := range taskOrder {
			task := stage.Tasks[taskID]
			if task == nil {
				continue
			}

			isLastTask := taskIndex == len(taskOrder)-1
			taskBranch := "├─"
			taskIndent := stageIndent + "│  "
			if isLastTask {
				taskBranch = "└─"
				taskIndent = stageIndent + "   "
			}

			taskLine := fmt.Sprintf(
				"%s%s %s · %s · %d tool uses · %s tokens",
				stageIndent,
				taskBranch,
				task.TaskID,
				taskDisplayStatus(task),
				task.ToolUses,
				formatCompactTokens(task.Tokens),
			)
			if task.Done && strings.EqualFold(strings.TrimSpace(task.Status), "success") {
				taskLine = colorSuccess(taskLine)
			}
			lines = append(lines, taskLine)

			if task.Done {
				resultText := strings.TrimSpace(task.ResultText)
				if resultText == "" {
					if strings.EqualFold(strings.TrimSpace(task.Status), "error") && strings.TrimSpace(task.ErrorText) != "" {
						resultText = strings.TrimSpace(task.ErrorText)
					} else {
						resultText = "Done"
					}
				}

				resultLines := strings.Split(resultText, "\n")
				for resultIndex, resultLine := range resultLines {
					prefix := taskIndent + "   "
					if resultIndex == 0 {
						prefix = taskIndent + "└─ "
					}
					renderedResult := prefix + strings.TrimRight(resultLine, "\r")
					if strings.EqualFold(strings.TrimSpace(task.Status), "success") {
						renderedResult = colorSuccess(renderedResult)
					}
					lines = append(lines, renderedResult)
				}
				continue
			}

			stepLines := task.activeStepLines(m.expanded)
			for stepIndex, stepLine := range stepLines {
				stepBranch := "├─"
				if stepIndex == len(stepLines)-1 {
					stepBranch = "└─"
				}
				lines = append(lines, taskIndent+stepBranch+" "+stepLine)
			}
		}
	}

	return lines
}

func (m *progressTUIModel) renderPipelineStatsTable() []string {
	headers := []string{"STAGE/TASK", "STATUS", "DURATION", "TOOL_USES", "TOKENS", "CACHE_READ", "COST_USD"}
	if m.finalRecord == nil || m.finalRecord.Pipeline == nil || len(m.finalRecord.Pipeline.Tasks) == 0 {
		return renderTextTable(headers, nil)
	}

	rows := make([][]string, 0, len(m.finalRecord.Pipeline.Tasks))
	for _, task := range m.finalRecord.Pipeline.Tasks {
		key := buildPipelineTaskKey(task.StageID, task.TaskID)
		liveTask := m.lookupTask(task.StageID, task.TaskID)
		toolUses := 0
		if liveTask != nil {
			toolUses = liveTask.ToolUses
		}

		tokens := int64(0)
		cacheRead := int64(0)
		cost := 0.0
		if task.Normalized != nil {
			tokens = task.Normalized.InputTokens +
				task.Normalized.CacheCreationInputTokens +
				task.Normalized.CacheReadInputTokens +
				task.Normalized.OutputTokens
			cacheRead = task.Normalized.CacheReadInputTokens
			cost = task.Normalized.CostUSD
		}
		if liveTask != nil && strings.TrimSpace(key) != "" {
			if tokens == 0 {
				tokens = liveTask.Tokens
			}
			if cacheRead == 0 {
				cacheRead = liveTask.CacheReadTokens
			}
			if cost == 0 {
				cost = liveTask.CostUSD
			}
		}

		rows = append(rows, []string{
			task.StageID + "/" + task.TaskID,
			normalizeStatus(task.Status, "unknown"),
			formatDurationMS(task.DurationMS),
			fmt.Sprintf("%d", toolUses),
			fmt.Sprintf("%d", tokens),
			fmt.Sprintf("%d", cacheRead),
			fmt.Sprintf("%.6f", cost),
		})
	}

	return renderTextTable(headers, rows)
}

func (m *progressTUIModel) lookupTask(stageID, taskID string) *pipelineTaskState {
	stage := m.stages[strings.TrimSpace(stageID)]
	if stage == nil {
		return nil
	}
	return stage.Tasks[strings.TrimSpace(taskID)]
}

func (m *progressTUIModel) effectiveStageCount() int {
	if m.stageCount > 0 {
		return m.stageCount
	}
	return len(m.stageOrder)
}

func (task *pipelineTaskState) addActiveStep(toolUseID, toolName, summary string) {
	normalizedToolUseID := strings.TrimSpace(toolUseID)
	if normalizedToolUseID == "" {
		normalizedToolUseID = fmt.Sprintf("step-%d", len(task.StepOrder)+1)
	}
	if _, exists := task.ActiveSteps[normalizedToolUseID]; !exists {
		task.StepOrder = append(task.StepOrder, normalizedToolUseID)
	}
	task.ActiveSteps[normalizedToolUseID] = activeStep{
		ToolUseID: normalizedToolUseID,
		Name:      strings.TrimSpace(toolName),
		Summary:   strings.TrimSpace(summary),
	}
}

func (task *pipelineTaskState) removeActiveStep(toolUseID string) {
	normalizedToolUseID := strings.TrimSpace(toolUseID)
	if normalizedToolUseID == "" {
		return
	}

	delete(task.ActiveSteps, normalizedToolUseID)
	for index, candidate := range task.StepOrder {
		if candidate != normalizedToolUseID {
			continue
		}
		task.StepOrder = append(task.StepOrder[:index], task.StepOrder[index+1:]...)
		break
	}
}

func (task *pipelineTaskState) activeStepLines(expanded bool) []string {
	lines := make([]string, 0, len(task.StepOrder))
	for _, stepID := range task.StepOrder {
		step, ok := task.ActiveSteps[stepID]
		if !ok {
			continue
		}
		line := strings.TrimSpace(step.Name)
		if line == "" {
			line = "step"
		}
		if strings.TrimSpace(step.Summary) != "" {
			line += ": " + strings.TrimSpace(step.Summary)
		}
		lines = append(lines, line)
	}

	if expanded || len(lines) <= 1 {
		return lines
	}
	return lines[:1]
}

func applyTaskOutcome(task *pipelineTaskState, tokens int64, cacheRead int64, cost float64, resultText string) {
	if task == nil {
		return
	}
	task.Tokens += tokens
	task.CacheReadTokens += cacheRead
	task.CostUSD += cost
	task.ResultText = mergeResultText(task.ResultText, resultText)
}

func mergeResultText(current string, incoming string) string {
	trimmedIncoming := strings.TrimSpace(incoming)
	if trimmedIncoming == "" {
		return current
	}
	if strings.TrimSpace(current) == "" {
		return trimmedIncoming
	}
	if strings.TrimSpace(current) == trimmedIncoming {
		return current
	}
	return current + "\n" + trimmedIncoming
}

func normalizeStatus(status string, fallback string) string {
	normalized := strings.ToLower(strings.TrimSpace(status))
	if normalized == "" {
		return fallback
	}
	return normalized
}

func stageTotalCount(stage *pipelineStageState) int {
	if stage == nil {
		return 0
	}
	if stage.TaskCount > 0 {
		return stage.TaskCount
	}
	return len(stage.TaskOrder)
}

func stageCompletedCount(stage *pipelineStageState) int {
	if stage == nil {
		return 0
	}
	if stage.CompletedTasks > 0 {
		return stage.CompletedTasks
	}

	completed := 0
	for _, taskID := range stage.TaskOrder {
		task := stage.Tasks[taskID]
		if task != nil && task.Done {
			completed++
		}
	}
	return completed
}

func stageDisplayStatus(stage *pipelineStageState) string {
	if stage == nil {
		return "pending"
	}
	status := strings.ToLower(strings.TrimSpace(stage.Status))
	if status != "" {
		return status
	}

	if stageCompletedCount(stage) == 0 {
		return "pending"
	}

	failed := 0
	for _, taskID := range stage.TaskOrder {
		task := stage.Tasks[taskID]
		if task == nil {
			continue
		}
		if task.Done && strings.EqualFold(strings.TrimSpace(task.Status), "error") {
			failed++
		}
	}
	if failed > 0 {
		return "error"
	}
	if stageCompletedCount(stage) >= stageTotalCount(stage) {
		return "success"
	}
	return "running"
}

func taskDisplayStatus(task *pipelineTaskState) string {
	if task == nil {
		return "pending"
	}
	if task.Done {
		status := strings.ToLower(strings.TrimSpace(task.Status))
		if status == "" {
			return "success"
		}
		if task.DurationMS > 0 {
			return status + " (" + formatDurationMS(task.DurationMS) + ")"
		}
		return status
	}
	if task.Started {
		return "running"
	}
	return "pending"
}

func formatCompactTokens(tokens int64) string {
	if tokens < 1000 {
		return fmt.Sprintf("%d", tokens)
	}
	if tokens < 1_000_000 {
		value := float64(tokens) / 1000
		formatted := fmt.Sprintf("%.1fk", value)
		if strings.HasSuffix(formatted, ".0k") {
			return strings.TrimSuffix(formatted, ".0k") + "k"
		}
		return formatted
	}
	value := float64(tokens) / 1_000_000
	formatted := fmt.Sprintf("%.1fm", value)
	if strings.HasSuffix(formatted, ".0m") {
		return strings.TrimSuffix(formatted, ".0m") + "m"
	}
	return formatted
}

func formatDurationMS(durationMS int64) string {
	if durationMS <= 0 {
		return "0s"
	}
	return formatDuration(time.Duration(durationMS) * time.Millisecond)
}

func renderTextTable(headers []string, rows [][]string) []string {
	if len(headers) == 0 {
		return nil
	}

	widths := make([]int, len(headers))
	for index, header := range headers {
		widths[index] = len(header)
	}
	for _, row := range rows {
		for index, cell := range row {
			if index >= len(widths) {
				continue
			}
			if len(cell) > widths[index] {
				widths[index] = len(cell)
			}
		}
	}

	lines := make([]string, 0, len(rows)+2)
	lines = append(lines, padTableRow(headers, widths))
	separator := make([]string, 0, len(widths))
	for _, width := range widths {
		separator = append(separator, strings.Repeat("-", width))
	}
	lines = append(lines, strings.Join(separator, "-+-"))

	for _, row := range rows {
		cells := make([]string, len(headers))
		copy(cells, row)
		lines = append(lines, padTableRow(cells, widths))
	}

	return lines
}

func padTableRow(cells []string, widths []int) string {
	padded := make([]string, 0, len(widths))
	for index, width := range widths {
		cell := ""
		if index < len(cells) {
			cell = cells[index]
		}
		padded = append(padded, fmt.Sprintf("%-*s", width, cell))
	}
	return strings.Join(padded, " | ")
}

func colorSuccess(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return value
	}
	return ansiGreen + value + ansiReset
}
