package stats

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	statsFileName         = "stats.json"
	outputFileName        = "output.log"
	outputNDJSONFileName  = "output.ndjson"
	runDirTimestampFormat = "20060102T150405"
)

func SaveRunRecord(runsDir string, record *RunRecord) (string, error) {
	if record == nil {
		return "", errors.New("run record is nil")
	}

	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now().UTC()
	}

	if strings.TrimSpace(record.RunID) == "" {
		runID, err := NewRunID()
		if err != nil {
			return "", err
		}
		record.RunID = runID
	}

	runDir, err := runArtifactsDir(runsDir, record.Timestamp, record.RunID)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return "", fmt.Errorf("create run directory: %w", err)
	}
	path := filepath.Join(runDir, statsFileName)

	content, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal run record: %w", err)
	}
	content = append(content, '\n')

	if err := os.WriteFile(path, content, 0o644); err != nil {
		return "", fmt.Errorf("write run record: %w", err)
	}

	return path, nil
}

func SaveRunArtifacts(runDir string, stdout string, stderr string) error {
	if strings.TrimSpace(runDir) == "" {
		return errors.New("run directory is empty")
	}
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return fmt.Errorf("create run directory: %w", err)
	}

	var ndjsonLog strings.Builder
	var outputLog strings.Builder
	appendArtifactLines(stdout, &ndjsonLog, &outputLog)
	appendArtifactLines(stderr, &ndjsonLog, &outputLog)

	if err := os.WriteFile(filepath.Join(runDir, outputNDJSONFileName), []byte(ndjsonLog.String()), 0o644); err != nil {
		return fmt.Errorf("write ndjson log file: %w", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, outputFileName), []byte(outputLog.String()), 0o644); err != nil {
		return fmt.Errorf("write output log file: %w", err)
	}

	return nil
}

func appendArtifactLines(raw string, ndjsonLog *strings.Builder, outputLog *strings.Builder) {
	for _, line := range strings.SplitAfter(raw, "\n") {
		if line == "" {
			continue
		}
		if isJSONObjectLine(line) {
			ndjsonLog.WriteString(line)
			continue
		}
		outputLog.WriteString(line)
	}
}

func isJSONObjectLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}

	var payload map[string]json.RawMessage
	return json.Unmarshal([]byte(trimmed), &payload) == nil
}

func NewRunID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate run id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func sanitizeID(id string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", " ", "_", ":", "_")
	return replacer.Replace(id)
}

func runArtifactsDir(runsDir string, timestamp time.Time, runID string) (string, error) {
	name, err := runDirName(timestamp, runID)
	if err != nil {
		return "", err
	}
	return filepath.Join(runsDir, name), nil
}

func runDirName(timestamp time.Time, runID string) (string, error) {
	if timestamp.IsZero() {
		return "", errors.New("run timestamp is zero")
	}
	if strings.TrimSpace(runID) == "" {
		return "", errors.New("run id is empty")
	}
	return fmt.Sprintf("%s-%s", timestamp.UTC().Format(runDirTimestampFormat), sanitizeID(runID)), nil
}
