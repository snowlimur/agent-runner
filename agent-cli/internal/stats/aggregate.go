package stats

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

func AggregateStats(runsDir string) (*Aggregate, error) {
	agg := &Aggregate{
		ByModel:      map[string]ModelAggregate{},
		SkippedFiles: []string{},
	}

	entries, err := os.ReadDir(runsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return agg, nil
		}
		return nil, fmt.Errorf("read runs directory: %w", err)
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			files = append(files, filepath.Join(runsDir, entry.Name(), statsFileName))
		}
	}
	sort.Strings(files)

	for _, path := range files {
		record, err := LoadRunRecord(path)
		if err != nil {
			agg.SkippedFiles = append(
				agg.SkippedFiles,
				filepath.ToSlash(filepath.Join(filepath.Base(filepath.Dir(path)), filepath.Base(path))),
			)
			continue
		}

		agg.TotalRuns++
		if record.Status == RunStatusSuccess {
			agg.SuccessRuns++
		} else {
			agg.ErrorRuns++
		}
		if record.Status == RunStatusParseError {
			agg.ParseErrorRuns++
		}

		if agg.FirstRunAt == nil || record.Timestamp.Before(*agg.FirstRunAt) {
			ts := record.Timestamp
			agg.FirstRunAt = &ts
		}
		if agg.LastRunAt == nil || record.Timestamp.After(*agg.LastRunAt) {
			ts := record.Timestamp
			agg.LastRunAt = &ts
		}

		mergeMetrics(&agg.Sums, record)
		mergeByModel(agg.ByModel, record)
	}

	return agg, nil
}

func LoadRunRecord(path string) (*RunRecord, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read run record %s: %w", path, err)
	}

	var record RunRecord
	if err := json.Unmarshal(content, &record); err != nil {
		return nil, fmt.Errorf("decode run record %s: %w", path, err)
	}
	return &record, nil
}

func mergeMetrics(target *AggregateMetrics, record *RunRecord) {
	target.DurationMS += record.Normalized.DurationMS
	target.DurationAPIMS += record.Normalized.DurationAPIMS
	target.NumTurns += record.Normalized.NumTurns
	target.TotalCostUSD += record.Normalized.TotalCostUSD
	target.InputTokens += record.Normalized.InputTokens
	target.CacheCreationInputTokens += record.Normalized.CacheCreationInputTokens
	target.CacheReadInputTokens += record.Normalized.CacheReadInputTokens
	target.OutputTokens += record.Normalized.OutputTokens
}

func mergeByModel(target map[string]ModelAggregate, record *RunRecord) {
	for model, metric := range record.Normalized.ByModel {
		current := target[model]
		current.InputTokens += metric.InputTokens
		current.OutputTokens += metric.OutputTokens
		current.CacheReadInputTokens += metric.CacheReadInputTokens
		current.CacheCreationInputTokens += metric.CacheCreationInputTokens
		current.WebSearchRequests += metric.WebSearchRequests
		current.CostUSD += metric.CostUSD
		target[model] = current
	}
}
