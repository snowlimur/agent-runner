package cli

import (
	"fmt"
	"io"
	"strings"
	"time"

	"agent-cli/internal/result"
)

type ProgressPrinter struct {
	w                        io.Writer
	toolByKey                map[string]progressTool
	sessionTask              map[string]taskRef
	lastTodoStatusesBySessID map[string]map[string]string
	pipelineMode             bool
	now                      func() time.Time
}

type progressTool struct {
	Name      string
	Summary   string
	StartedAt time.Time
	SessionID string
	ToolUseID string
}

type taskRef struct {
	StageID string
	TaskID  string
}

func (t taskRef) isBound() bool {
	return strings.TrimSpace(t.StageID) != "" && strings.TrimSpace(t.TaskID) != ""
}

func NewProgressPrinter(w io.Writer) *ProgressPrinter {
	return &ProgressPrinter{
		w:                        w,
		toolByKey:                map[string]progressTool{},
		sessionTask:              map[string]taskRef{},
		lastTodoStatusesBySessID: map[string]map[string]string{},
		now:                      time.Now,
	}
}

func (p *ProgressPrinter) HandleEvent(event *result.StreamEvent) {
	if event == nil {
		return
	}

	if event.Pipeline != nil {
		p.handlePipeline(event.Pipeline)
	}

	if event.System != nil && event.System.Subtype == "init" {
		p.printLine(
			p.now(),
			event.System.SessionID,
			"init",
			"session=%s model=%s",
			event.System.SessionID,
			event.System.Model,
		)
	}

	if event.Assistant != nil {
		p.handleAssistant(event.Assistant)
	}

	if event.User != nil {
		p.handleUser(event.User)
	}

	if event.Result != nil {
		p.handleResult(event.Result)
	}
}

func (p *ProgressPrinter) handlePipeline(event *result.PipelineEvent) {
	if event == nil {
		return
	}
	p.pipelineMode = true

	switch strings.ToLower(strings.TrimSpace(event.Event)) {
	case "task_session_bind":
		sessionID := strings.TrimSpace(event.SessionID)
		if sessionID == "" {
			return
		}

		stageID := strings.TrimSpace(event.StageID)
		taskID := strings.TrimSpace(event.TaskID)
		if stageID == "" || taskID == "" {
			return
		}

		p.sessionTask[sessionID] = taskRef{
			StageID: stageID,
			TaskID:  taskID,
		}
	case "task_timeout":
		message := strings.TrimSpace(event.Reason)
		if message == "" && event.IdleTimeoutSec > 0 {
			message = fmt.Sprintf("idle timeout after %ds without task output", event.IdleTimeoutSec)
		}
		if message == "" {
			message = "task timed out"
		}
		p.printPipelineTaskLine(p.now(), event.StageID, event.TaskID, "timeout", "%s", message)
	case "task_finish":
		if !strings.EqualFold(strings.TrimSpace(event.Status), "error") {
			return
		}
		message := strings.TrimSpace(event.ErrorMessage)
		if message == "" {
			message = "task failed"
		}
		p.printPipelineTaskLine(p.now(), event.StageID, event.TaskID, "task:error", "%s", message)
	}
}

func (p *ProgressPrinter) handleAssistant(event *result.AssistantEvent) {
	sessionID := strings.TrimSpace(event.SessionID)

	for _, item := range event.Content {
		if item.ToolUse == nil {
			continue
		}

		summary := strings.TrimSpace(item.ToolUse.Input.Command)
		if summary == "" {
			summary = strings.TrimSpace(item.ToolUse.Input.Description)
		}

		startedAt := p.now()
		tool := progressTool{
			Name:      item.ToolUse.Name,
			Summary:   summary,
			StartedAt: startedAt,
			SessionID: sessionID,
			ToolUseID: item.ToolUse.ID,
		}
		p.toolByKey[buildToolKey(sessionID, item.ToolUse.ID)] = tool

		if isTodoWriteTool(item.ToolUse.Name) {
			continue
		}

		label := buildToolLabel(item.ToolUse.Name, item.ToolUse.ID, "start")
		if summary != "" {
			p.printLine(startedAt, sessionID, label, "%s", summary)
			continue
		}
		p.printLine(startedAt, sessionID, label, "")
	}
}

func (p *ProgressPrinter) handleUser(event *result.UserEvent) {
	suppressTodoTransitions := false
	sessionID := strings.TrimSpace(event.SessionID)

	for _, item := range event.ToolResults {
		tool, ok := p.toolByKey[buildToolKey(sessionID, item.ToolUseID)]
		if !ok && sessionID == "" {
			tool, _ = p.findToolByUseID(item.ToolUseID)
		}

		toolName := strings.TrimSpace(tool.Name)
		if toolName == "" {
			toolName = "unknown"
		}

		toolSessionID := sessionID
		if toolSessionID == "" {
			toolSessionID = strings.TrimSpace(tool.SessionID)
		}

		toolUseID := strings.TrimSpace(item.ToolUseID)
		if toolUseID == "" {
			toolUseID = strings.TrimSpace(tool.ToolUseID)
		}

		if isTodoWriteTool(toolName) {
			suppressTodoTransitions = true
			continue
		}

		status := "ok"
		if item.IsError {
			status = "error"
		}
		now := p.now()
		durationText := ""
		if !tool.StartedAt.IsZero() {
			durationText = formatDuration(now.Sub(tool.StartedAt))
		}

		snippet := buildToolSnippet(item, event.ToolUseResult)
		if isReadTool(toolName) && !item.IsError {
			snippet = ""
		}
		doneMsg := status
		if durationText != "" {
			doneMsg += " | " + durationText
		}

		doneLabel := buildToolLabel(toolName, toolUseID, "done")
		if snippet == "" {
			p.printLine(now, toolSessionID, doneLabel, "%s", doneMsg)
			continue
		}

		p.printLine(now, toolSessionID, doneLabel, "%s", doneMsg)
		outputLabel := buildToolLabel(toolName, toolUseID, "output")
		for _, line := range strings.Split(snippet, "\n") {
			p.printLine(now, toolSessionID, outputLabel, "%s", line)
		}
	}

	if !suppressTodoTransitions {
		p.printTodoTransitions(sessionID, event.ToolUseResult.OldTodos, event.ToolUseResult.NewTodos)
	}
}

func (p *ProgressPrinter) handleResult(event *result.AgentResult) {
	if event == nil {
		return
	}

	message := strings.TrimSpace(event.Result)
	if message == "" {
		return
	}

	sessionID := strings.TrimSpace(event.SessionID)
	p.printLine(p.now(), sessionID, "result", "%s", message)
}

func buildToolSnippet(item result.UserToolResult, payload result.UserToolUseResult) string {
	value := ""
	if item.IsError {
		value = payload.Stderr
		if strings.TrimSpace(value) == "" {
			value = item.Content
		}
	} else {
		value = payload.Stdout
		if strings.TrimSpace(value) == "" {
			value = item.Content
		}
	}

	return compactSnippet(value, 120)
}

func compactSnippet(value string, maxLen int) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	compact := strings.ReplaceAll(trimmed, "\r\n", "\n")
	if len(compact) <= maxLen {
		return compact
	}
	if maxLen <= 3 {
		return compact[:maxLen]
	}
	return compact[:maxLen-3] + "..."
}

func (p *ProgressPrinter) printTodoTransitions(sessionID string, oldTodos, newTodos []result.TodoItem) {
	sessionKey := todoSessionKey(sessionID)
	if _, ok := p.lastTodoStatusesBySessID[sessionKey]; !ok {
		p.lastTodoStatusesBySessID[sessionKey] = map[string]string{}
	}

	oldMap := map[string]string{}
	for _, item := range oldTodos {
		oldMap[item.Content] = item.Status
	}
	if len(oldMap) == 0 {
		for key, value := range p.lastTodoStatusesBySessID[sessionKey] {
			oldMap[key] = value
		}
	}

	for _, item := range newTodos {
		oldStatus, ok := oldMap[item.Content]
		if !ok {
			oldStatus = "none"
		}
		if oldStatus != item.Status {
			p.printLine(p.now(), sessionID, "todo", "%s: %s -> %s", item.Content, oldStatus, item.Status)
		}
		p.lastTodoStatusesBySessID[sessionKey][item.Content] = item.Status
	}
}

func isTodoWriteTool(name string) bool {
	return strings.EqualFold(strings.TrimSpace(name), "TodoWrite")
}

func isReadTool(name string) bool {
	return strings.EqualFold(strings.TrimSpace(name), "Read")
}

func formatDuration(value time.Duration) string {
	if value < 0 {
		value = 0
	}
	switch {
	case value < time.Second:
		rounded := value.Round(time.Millisecond)
		if rounded == 0 {
			return "0s"
		}
		return rounded.String()
	case value < time.Minute:
		return value.Round(100 * time.Millisecond).String()
	default:
		return value.Round(time.Second).String()
	}
}

func (p *ProgressPrinter) printLine(ts time.Time, sessionID, label, format string, args ...any) {
	timestamp := ts.Local().Format("15:04:05")
	taskPrefix := p.taskPrefixForSession(sessionID)

	if format == "" {
		if taskPrefix == "" {
			fmt.Fprintf(p.w, "%s [%s]\n", timestamp, label)
			return
		}
		fmt.Fprintf(p.w, "%s %s [%s]\n", timestamp, taskPrefix, label)
		return
	}

	message := fmt.Sprintf(format, args...)
	if taskPrefix == "" {
		fmt.Fprintf(p.w, "%s [%s] %s\n", timestamp, label, message)
		return
	}
	fmt.Fprintf(p.w, "%s %s [%s] %s\n", timestamp, taskPrefix, label, message)
}

func (p *ProgressPrinter) printPipelineTaskLine(ts time.Time, stageID, taskID, label, format string, args ...any) {
	timestamp := ts.Local().Format("15:04:05")
	stage := strings.TrimSpace(stageID)
	task := strings.TrimSpace(taskID)
	taskPrefix := "[unbound]"
	if stage != "" && task != "" {
		taskPrefix = fmt.Sprintf("[%s/%s]", stage, task)
	}

	if format == "" {
		fmt.Fprintf(p.w, "%s %s [%s]\n", timestamp, taskPrefix, label)
		return
	}
	message := fmt.Sprintf(format, args...)
	fmt.Fprintf(p.w, "%s %s [%s] %s\n", timestamp, taskPrefix, label, message)
}

func (p *ProgressPrinter) taskPrefixForSession(sessionID string) string {
	normalizedSessionID := strings.TrimSpace(sessionID)
	if normalizedSessionID != "" {
		if task, ok := p.sessionTask[normalizedSessionID]; ok && task.isBound() {
			return fmt.Sprintf("[%s/%s]", task.StageID, task.TaskID)
		}
	}
	if p.pipelineMode {
		return "[unbound]"
	}
	return ""
}

func (p *ProgressPrinter) findToolByUseID(toolUseID string) (progressTool, bool) {
	normalizedToolUseID := strings.TrimSpace(toolUseID)
	if normalizedToolUseID == "" {
		return progressTool{}, false
	}

	for _, tool := range p.toolByKey {
		if strings.TrimSpace(tool.ToolUseID) == normalizedToolUseID {
			return tool, true
		}
	}
	return progressTool{}, false
}

func buildToolKey(sessionID, toolUseID string) string {
	normalizedSessionID := strings.TrimSpace(sessionID)
	normalizedToolUseID := strings.TrimSpace(toolUseID)
	if normalizedSessionID == "" {
		return normalizedToolUseID
	}
	return normalizedSessionID + "|" + normalizedToolUseID
}

func buildToolLabel(toolName, toolUseID, phase string) string {
	normalizedToolName := strings.TrimSpace(toolName)
	if normalizedToolName == "" {
		normalizedToolName = "unknown"
	}

	normalizedToolUseID := strings.TrimSpace(toolUseID)
	if normalizedToolUseID == "" {
		return normalizedToolName + ":" + phase
	}
	return normalizedToolName + "#" + normalizedToolUseID + ":" + phase
}

func todoSessionKey(sessionID string) string {
	normalized := strings.TrimSpace(sessionID)
	if normalized == "" {
		return "__global__"
	}
	return normalized
}
