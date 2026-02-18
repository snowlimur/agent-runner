package cli

import (
	"strings"
	"time"
)

type taskRef struct {
	StageID string
	TaskID  string
}

func (t taskRef) isBound() bool {
	return strings.TrimSpace(t.StageID) != "" && strings.TrimSpace(t.TaskID) != ""
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

func buildToolKey(sessionID, toolUseID string) string {
	normalizedSessionID := strings.TrimSpace(sessionID)
	normalizedToolUseID := strings.TrimSpace(toolUseID)
	if normalizedSessionID == "" {
		return normalizedToolUseID
	}
	return normalizedSessionID + "|" + normalizedToolUseID
}
