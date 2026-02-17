package stats

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"agent-cli/internal/result"
)

func TestAggregateStatsMixedRuns(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "runs")

	t1 := time.Date(2026, 2, 12, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 2, 12, 11, 0, 0, 0, time.UTC)

	_, err := SaveRunRecord(dir, &RunRecord{
		Timestamp: t1,
		Status:    RunStatusSuccess,
		Stream: StreamMetrics{
			TotalJSONEvents:          10,
			EventCounts:              map[string]int64{"assistant": 4, "result": 1},
			NonJSONLines:             1,
			InvalidJSONLines:         0,
			ToolUseTotal:             2,
			ToolUseByName:            map[string]int64{"Bash": 1, "TodoWrite": 1},
			ToolResultTotal:          2,
			ToolResultErrorTotal:     0,
			UnmatchedToolUseTotal:    0,
			UnmatchedToolResultTotal: 0,
			TodoTransitionTotal:      3,
			TodoCompletedTotal:       1,
		},
		Normalized: result.NormalizedMetrics{
			DurationMS:               100,
			DurationAPIMS:            200,
			NumTurns:                 2,
			TotalCostUSD:             1.5,
			InputTokens:              10,
			CacheCreationInputTokens: 3,
			CacheReadInputTokens:     4,
			OutputTokens:             5,
			ByModel: map[string]result.ModelMetric{
				"model-a": {
					InputTokens:              10,
					OutputTokens:             5,
					CacheReadInputTokens:     4,
					CacheCreationInputTokens: 3,
					WebSearchRequests:        1,
					CostUSD:                  1.5,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("save first record: %v", err)
	}

	_, err = SaveRunRecord(dir, &RunRecord{
		Timestamp: t2,
		Status:    RunStatusParseError,
		Stream: StreamMetrics{
			TotalJSONEvents:          3,
			EventCounts:              map[string]int64{"assistant": 1, "user": 1},
			NonJSONLines:             2,
			InvalidJSONLines:         1,
			ToolUseTotal:             1,
			ToolUseByName:            map[string]int64{"Bash": 1},
			ToolResultTotal:          1,
			ToolResultErrorTotal:     1,
			UnmatchedToolUseTotal:    1,
			UnmatchedToolResultTotal: 0,
			TodoTransitionTotal:      1,
			TodoCompletedTotal:       0,
		},
		Normalized: result.NormalizedMetrics{
			DurationMS:    50,
			DurationAPIMS: 60,
			NumTurns:      1,
			TotalCostUSD:  0.5,
			InputTokens:   2,
			OutputTokens:  1,
			ByModel: map[string]result.ModelMetric{
				"model-a": {
					InputTokens:  2,
					OutputTokens: 1,
					CostUSD:      0.5,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("save second record: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(dir, "broken-run"), 0o755); err != nil {
		t.Fatalf("mkdir broken run dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken-run", statsFileName), []byte("{"), 0o644); err != nil {
		t.Fatalf("write broken file: %v", err)
	}
	// Legacy layout file should be ignored by aggregation in runs mode.
	if err := os.WriteFile(filepath.Join(dir, "legacy.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write legacy file: %v", err)
	}

	agg, err := AggregateStats(dir)
	if err != nil {
		t.Fatalf("aggregate stats: %v", err)
	}

	if agg.TotalRuns != 2 {
		t.Fatalf("unexpected total runs: %d", agg.TotalRuns)
	}
	if agg.SuccessRuns != 1 {
		t.Fatalf("unexpected success runs: %d", agg.SuccessRuns)
	}
	if agg.ErrorRuns != 1 {
		t.Fatalf("unexpected error runs: %d", agg.ErrorRuns)
	}
	if agg.ParseErrorRuns != 1 {
		t.Fatalf("unexpected parse error runs: %d", agg.ParseErrorRuns)
	}
	if agg.Sums.DurationMS != 150 {
		t.Fatalf("unexpected duration sum: %d", agg.Sums.DurationMS)
	}
	if agg.ByModel["model-a"].InputTokens != 12 {
		t.Fatalf("unexpected by_model input tokens: %d", agg.ByModel["model-a"].InputTokens)
	}
	if agg.StreamSums.TotalJSONEvents != 13 {
		t.Fatalf("unexpected stream json events: %d", agg.StreamSums.TotalJSONEvents)
	}
	if agg.StreamSums.NonJSONLines != 3 {
		t.Fatalf("unexpected stream non-json lines: %d", agg.StreamSums.NonJSONLines)
	}
	if agg.StreamSums.ToolUseByName["Bash"] != 2 {
		t.Fatalf("unexpected stream tool count for Bash: %d", agg.StreamSums.ToolUseByName["Bash"])
	}
	if agg.StreamSums.TodoCompletedTotal != 1 {
		t.Fatalf("unexpected todo completed transitions: %d", agg.StreamSums.TodoCompletedTotal)
	}
	if len(agg.SkippedFiles) != 1 || agg.SkippedFiles[0] != "broken-run/stats.json" {
		t.Fatalf("unexpected skipped files: %#v", agg.SkippedFiles)
	}
	if agg.FirstRunAt == nil || !agg.FirstRunAt.Equal(t1) {
		t.Fatalf("unexpected first run at: %v", agg.FirstRunAt)
	}
	if agg.LastRunAt == nil || !agg.LastRunAt.Equal(t2) {
		t.Fatalf("unexpected last run at: %v", agg.LastRunAt)
	}
}

func TestAggregateStatsMissingDirectory(t *testing.T) {
	t.Parallel()

	agg, err := AggregateStats(filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatalf("aggregate stats: %v", err)
	}
	if agg.TotalRuns != 0 {
		t.Fatalf("expected 0 runs, got %d", agg.TotalRuns)
	}
}

func TestAggregateStatsReadsTimestampPrefixedRunDirectory(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "runs")
	runDir := filepath.Join(dir, "20260213T090807.123456789Z-manual-id")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	record := RunRecord{
		RunID:     "manual-id",
		Timestamp: time.Date(2026, 2, 13, 9, 8, 7, 123456789, time.UTC),
		Status:    RunStatusSuccess,
		Normalized: result.NormalizedMetrics{
			DurationMS: 10,
			NumTurns:   1,
		},
	}
	content, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatalf("marshal record: %v", err)
	}
	content = append(content, '\n')
	if err := os.WriteFile(filepath.Join(runDir, statsFileName), content, 0o644); err != nil {
		t.Fatalf("write stats file: %v", err)
	}

	agg, err := AggregateStats(dir)
	if err != nil {
		t.Fatalf("aggregate stats: %v", err)
	}
	if agg.TotalRuns != 1 {
		t.Fatalf("expected 1 run, got %d", agg.TotalRuns)
	}
	if agg.SuccessRuns != 1 {
		t.Fatalf("expected 1 success run, got %d", agg.SuccessRuns)
	}
	if len(agg.SkippedFiles) != 0 {
		t.Fatalf("expected no skipped files, got %#v", agg.SkippedFiles)
	}
}

func TestAggregateStatsAcceptsLegacyPromptField(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "runs")
	runDir := filepath.Join(dir, "20260213T090807-legacy")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	content := []byte(`{
  "run_id": "legacy",
  "timestamp": "2026-02-13T09:08:07Z",
  "status": "success",
  "docker_exit_code": 0,
  "cwd": "/tmp/work",
  "prompt": {
    "source": "inline",
    "prompt_sha256": "abc",
    "prompt_bytes": 3
  },
  "normalized": {
    "duration_ms": 10,
    "duration_api_ms": 0,
    "num_turns": 1,
    "total_cost_usd": 0
  }
}
`)
	if err := os.WriteFile(filepath.Join(runDir, statsFileName), content, 0o644); err != nil {
		t.Fatalf("write stats file: %v", err)
	}

	agg, err := AggregateStats(dir)
	if err != nil {
		t.Fatalf("aggregate stats: %v", err)
	}
	if agg.TotalRuns != 1 {
		t.Fatalf("expected 1 run, got %d", agg.TotalRuns)
	}
	if agg.SuccessRuns != 1 {
		t.Fatalf("expected 1 success run, got %d", agg.SuccessRuns)
	}
	if len(agg.SkippedFiles) != 0 {
		t.Fatalf("expected no skipped files, got %#v", agg.SkippedFiles)
	}
}
