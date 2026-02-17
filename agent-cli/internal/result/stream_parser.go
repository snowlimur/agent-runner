package result

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type StreamLineKind string

const (
	StreamLineJSONEvent   StreamLineKind = "json_event"
	StreamLineNonJSON     StreamLineKind = "non_json"
	StreamLineInvalidJSON StreamLineKind = "invalid_json"
)

type StreamEvent struct {
	Raw       string
	Type      string
	System    *SystemEvent
	Assistant *AssistantEvent
	User      *UserEvent
	Pipeline  *PipelineEvent
	Result    *AgentResult
}

type TodoItem struct {
	Content    string `json:"content"`
	ActiveForm string `json:"activeForm"`
	Status     string `json:"status"`
}

type ToolUseInput struct {
	Command     string     `json:"command,omitempty"`
	Description string     `json:"description,omitempty"`
	Todos       []TodoItem `json:"todos,omitempty"`
}

type SystemEvent struct {
	Subtype   string `json:"subtype"`
	SessionID string `json:"session_id"`
	Model     string `json:"model"`
}

type AssistantEvent struct {
	MessageID string             `json:"message_id"`
	SessionID string             `json:"session_id"`
	Content   []AssistantContent `json:"content"`
}

type AssistantContent struct {
	Type    string            `json:"type"`
	Text    string            `json:"text,omitempty"`
	ToolUse *AssistantToolUse `json:"tool_use,omitempty"`
}

type AssistantToolUse struct {
	ID    string       `json:"id"`
	Name  string       `json:"name"`
	Input ToolUseInput `json:"input"`
}

type UserEvent struct {
	SessionID     string            `json:"session_id"`
	ToolResults   []UserToolResult  `json:"tool_results"`
	ToolUseResult UserToolUseResult `json:"tool_use_result"`
}

type UserToolResult struct {
	ToolUseID string `json:"tool_use_id"`
	Type      string `json:"type"`
	Content   string `json:"content"`
	IsError   bool   `json:"is_error"`
}

type UserToolUseResult struct {
	Stdout           string     `json:"stdout"`
	Stderr           string     `json:"stderr"`
	Interrupted      bool       `json:"interrupted"`
	IsImage          bool       `json:"isImage"`
	NoOutputExpected bool       `json:"noOutputExpected"`
	OldTodos         []TodoItem `json:"oldTodos"`
	NewTodos         []TodoItem `json:"newTodos"`
}

type PipelineEvent struct {
	Event              string `json:"event"`
	Version            string `json:"version"`
	StageID            string `json:"stage_id"`
	TaskID             string `json:"task_id"`
	SessionID          string `json:"session_id"`
	Status             string `json:"status"`
	Mode               string `json:"mode"`
	Model              string `json:"model"`
	Verbosity          string `json:"verbosity"`
	Workspace          string `json:"workspace"`
	OnError            string `json:"on_error"`
	PromptSource       string `json:"prompt_source"`
	PromptFile         string `json:"prompt_file"`
	Signal             string `json:"signal"`
	StartedAt          string `json:"started_at"`
	FinishedAt         string `json:"finished_at"`
	ErrorMessage       string `json:"error_message"`
	Reason             string `json:"reason"`
	StageCount         int    `json:"stage_count"`
	CompletedStages    int    `json:"completed_stages"`
	TaskCount          int    `json:"task_count"`
	FailedTaskCount    int    `json:"failed_task_count"`
	CompletedTasks     int    `json:"completed_tasks"`
	FailedTasks        int    `json:"failed_tasks"`
	MaxParallel        int    `json:"max_parallel"`
	IdleTimeoutSec     int    `json:"idle_timeout_sec"`
	TaskIdleTimeoutSec int    `json:"task_idle_timeout_sec"`
	ExitCode           int    `json:"exit_code"`
	DurationMS         int64  `json:"duration_ms"`
}

func ParseStreamLine(line string) (*StreamEvent, StreamLineKind, error) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return nil, StreamLineNonJSON, nil
	}
	if !strings.HasPrefix(trimmed, "{") {
		return nil, StreamLineNonJSON, nil
	}

	var header struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(trimmed), &header); err != nil {
		return nil, StreamLineInvalidJSON, fmt.Errorf("decode stream header: %w", err)
	}

	event := &StreamEvent{
		Raw:  trimmed,
		Type: header.Type,
	}

	switch header.Type {
	case "system":
		var payload struct {
			Type      string `json:"type"`
			Subtype   string `json:"subtype"`
			SessionID string `json:"session_id"`
			Model     string `json:"model"`
		}
		if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
			return nil, StreamLineInvalidJSON, fmt.Errorf("decode system event: %w", err)
		}
		event.System = &SystemEvent{
			Subtype:   payload.Subtype,
			SessionID: payload.SessionID,
			Model:     payload.Model,
		}
	case "assistant":
		var payload struct {
			Type      string `json:"type"`
			SessionID string `json:"session_id"`
			Message   struct {
				ID      string `json:"id"`
				Content []struct {
					Type  string `json:"type"`
					Text  string `json:"text"`
					ID    string `json:"id"`
					Name  string `json:"name"`
					Input struct {
						Command     string     `json:"command"`
						Description string     `json:"description"`
						Todos       []TodoItem `json:"todos"`
					} `json:"input"`
				} `json:"content"`
			} `json:"message"`
		}
		if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
			return nil, StreamLineInvalidJSON, fmt.Errorf("decode assistant event: %w", err)
		}

		content := make([]AssistantContent, 0, len(payload.Message.Content))
		for _, item := range payload.Message.Content {
			mapped := AssistantContent{
				Type: item.Type,
				Text: item.Text,
			}
			if item.Type == "tool_use" {
				mapped.ToolUse = &AssistantToolUse{
					ID:   item.ID,
					Name: item.Name,
					Input: ToolUseInput{
						Command:     item.Input.Command,
						Description: item.Input.Description,
						Todos:       item.Input.Todos,
					},
				}
			}
			content = append(content, mapped)
		}
		event.Assistant = &AssistantEvent{
			MessageID: payload.Message.ID,
			SessionID: payload.SessionID,
			Content:   content,
		}
	case "user":
		var payload struct {
			Type      string `json:"type"`
			SessionID string `json:"session_id"`
			Message   struct {
				Content []struct {
					ToolUseID string `json:"tool_use_id"`
					Type      string `json:"type"`
					Content   string `json:"content"`
					IsError   bool   `json:"is_error"`
				} `json:"content"`
			} `json:"message"`
			ToolUseResult UserToolUseResult `json:"tool_use_result"`
		}
		if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
			return nil, StreamLineInvalidJSON, fmt.Errorf("decode user event: %w", err)
		}

		results := make([]UserToolResult, 0, len(payload.Message.Content))
		for _, item := range payload.Message.Content {
			if item.Type != "tool_result" {
				continue
			}
			results = append(results, UserToolResult{
				ToolUseID: item.ToolUseID,
				Type:      item.Type,
				Content:   item.Content,
				IsError:   item.IsError,
			})
		}
		event.User = &UserEvent{
			SessionID:     payload.SessionID,
			ToolResults:   results,
			ToolUseResult: payload.ToolUseResult,
		}
	case "result":
		var agentResult AgentResult
		if err := json.Unmarshal([]byte(trimmed), &agentResult); err != nil {
			return nil, StreamLineInvalidJSON, fmt.Errorf("decode final result event: %w", err)
		}
		event.Result = &agentResult
	case "pipeline_event":
		var payload struct {
			Type               string `json:"type"`
			Event              string `json:"event"`
			Version            string `json:"version"`
			StageID            string `json:"stage_id"`
			TaskID             string `json:"task_id"`
			SessionID          string `json:"session_id"`
			Status             string `json:"status"`
			Mode               string `json:"mode"`
			Model              string `json:"model"`
			Verbosity          string `json:"verbosity"`
			Workspace          string `json:"workspace"`
			OnError            string `json:"on_error"`
			PromptSource       string `json:"prompt_source"`
			PromptFile         string `json:"prompt_file"`
			Signal             string `json:"signal"`
			StartedAt          string `json:"started_at"`
			FinishedAt         string `json:"finished_at"`
			ErrorMessage       string `json:"error_message"`
			Reason             string `json:"reason"`
			StageCount         int    `json:"stage_count"`
			CompletedStages    int    `json:"completed_stages"`
			TaskCount          int    `json:"task_count"`
			FailedTaskCount    int    `json:"failed_task_count"`
			CompletedTasks     int    `json:"completed_tasks"`
			FailedTasks        int    `json:"failed_tasks"`
			MaxParallel        int    `json:"max_parallel"`
			IdleTimeoutSec     int    `json:"idle_timeout_sec"`
			TaskIdleTimeoutSec int    `json:"task_idle_timeout_sec"`
			ExitCode           int    `json:"exit_code"`
			DurationMS         int64  `json:"duration_ms"`
		}
		if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
			return nil, StreamLineInvalidJSON, fmt.Errorf("decode pipeline event: %w", err)
		}
		event.Pipeline = &PipelineEvent{
			Event:              payload.Event,
			Version:            payload.Version,
			StageID:            payload.StageID,
			TaskID:             payload.TaskID,
			SessionID:          payload.SessionID,
			Status:             payload.Status,
			Mode:               payload.Mode,
			Model:              payload.Model,
			Verbosity:          payload.Verbosity,
			Workspace:          payload.Workspace,
			OnError:            payload.OnError,
			PromptSource:       payload.PromptSource,
			PromptFile:         payload.PromptFile,
			Signal:             payload.Signal,
			StartedAt:          payload.StartedAt,
			FinishedAt:         payload.FinishedAt,
			ErrorMessage:       payload.ErrorMessage,
			Reason:             payload.Reason,
			StageCount:         payload.StageCount,
			CompletedStages:    payload.CompletedStages,
			TaskCount:          payload.TaskCount,
			FailedTaskCount:    payload.FailedTaskCount,
			CompletedTasks:     payload.CompletedTasks,
			FailedTasks:        payload.FailedTasks,
			MaxParallel:        payload.MaxParallel,
			IdleTimeoutSec:     payload.IdleTimeoutSec,
			TaskIdleTimeoutSec: payload.TaskIdleTimeoutSec,
			ExitCode:           payload.ExitCode,
			DurationMS:         payload.DurationMS,
		}
	}

	return event, StreamLineJSONEvent, nil
}

func ExtractFinalResultFromStream(lines []string) (*ParsedResult, error) {
	finalResultRaw := ""
	for _, line := range lines {
		event, kind, err := ParseStreamLine(line)
		if err != nil || kind != StreamLineJSONEvent || event == nil {
			continue
		}
		if event.Result != nil {
			finalResultRaw = event.Raw
		}
	}
	if finalResultRaw != "" {
		return ParseAgentResult(finalResultRaw)
	}

	fullOutput := strings.TrimSpace(strings.Join(lines, "\n"))
	if fullOutput != "" {
		parsed, err := ParseAgentResult(fullOutput)
		if err == nil {
			return parsed, nil
		}
	}

	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		parsed, err := ParseAgentResult(line)
		if err == nil {
			return parsed, nil
		}
	}

	return nil, errors.New("final result event not found in stream output")
}
