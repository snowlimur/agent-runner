package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"

	"agent-cli/internal/config"
	"agent-cli/internal/stats"
)

func StatsCommand(cwd string, args []string) error {
	fs := flag.NewFlagSet("stats", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var jsonOutput bool
	fs.BoolVar(&jsonOutput, "json", false, "print statistics as JSON")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return errors.New("stats command does not accept positional arguments")
	}

	agg, err := stats.AggregateStats(config.RunsDir(cwd))
	if err != nil {
		return err
	}

	if jsonOutput {
		encoded, err := json.MarshalIndent(agg, "", "  ")
		if err != nil {
			return fmt.Errorf("encode stats JSON: %w", err)
		}
		fmt.Println(string(encoded))
		return nil
	}

	printStatsTable(agg)
	return nil
}

func printStatsTable(agg *stats.Aggregate) {
	fmt.Println("Run Summary")
	fmt.Printf("  Total: %d\n", agg.TotalRuns)
	fmt.Printf("  Success: %d\n", agg.SuccessRuns)
	fmt.Printf("  Errors: %d\n", agg.ErrorRuns)
	fmt.Printf("  Parse Errors: %d\n", agg.ParseErrorRuns)

	if agg.FirstRunAt != nil {
		fmt.Printf("  First Run: %s\n", agg.FirstRunAt.UTC().Format("2006-01-02T15:04:05Z"))
	}
	if agg.LastRunAt != nil {
		fmt.Printf("  Last Run: %s\n", agg.LastRunAt.UTC().Format("2006-01-02T15:04:05Z"))
	}

	fmt.Println()
	fmt.Println("Sums")
	fmt.Printf("  Duration(ms): %d\n", agg.Sums.DurationMS)
	fmt.Printf("  API Duration(ms): %d\n", agg.Sums.DurationAPIMS)
	fmt.Printf("  Turns: %d\n", agg.Sums.NumTurns)
	fmt.Printf("  Cost(USD): %.6f\n", agg.Sums.TotalCostUSD)
	fmt.Printf("  Input Tokens: %d\n", agg.Sums.InputTokens)
	fmt.Printf("  Cache Create Tokens: %d\n", agg.Sums.CacheCreationInputTokens)
	fmt.Printf("  Cache Read Tokens: %d\n", agg.Sums.CacheReadInputTokens)
	fmt.Printf("  Output Tokens: %d\n", agg.Sums.OutputTokens)

	fmt.Println()
	fmt.Println("Stream")
	fmt.Printf("  JSON Events: %d\n", agg.StreamSums.TotalJSONEvents)
	fmt.Printf("  Non-JSON Lines: %d\n", agg.StreamSums.NonJSONLines)
	fmt.Printf("  Invalid JSON Lines: %d\n", agg.StreamSums.InvalidJSONLines)
	fmt.Printf("  Tool Use: %d\n", agg.StreamSums.ToolUseTotal)
	fmt.Printf("  Tool Result: %d\n", agg.StreamSums.ToolResultTotal)
	fmt.Printf("  Tool Result Errors: %d\n", agg.StreamSums.ToolResultErrorTotal)
	fmt.Printf("  Unmatched Tool Uses: %d\n", agg.StreamSums.UnmatchedToolUseTotal)
	fmt.Printf("  Unmatched Tool Results: %d\n", agg.StreamSums.UnmatchedToolResultTotal)
	fmt.Printf("  Todo Transitions: %d\n", agg.StreamSums.TodoTransitionTotal)
	fmt.Printf("  Todo Completed Transitions: %d\n", agg.StreamSums.TodoCompletedTotal)

	if len(agg.StreamSums.EventCounts) > 0 {
		fmt.Println("  Event Counts")
		eventTypes := make([]string, 0, len(agg.StreamSums.EventCounts))
		for key := range agg.StreamSums.EventCounts {
			eventTypes = append(eventTypes, key)
		}
		sort.Strings(eventTypes)
		for _, key := range eventTypes {
			fmt.Printf("    %s: %d\n", key, agg.StreamSums.EventCounts[key])
		}
	}

	if len(agg.StreamSums.ToolUseByName) > 0 {
		fmt.Println("  Tool Use By Name")
		toolNames := make([]string, 0, len(agg.StreamSums.ToolUseByName))
		for key := range agg.StreamSums.ToolUseByName {
			toolNames = append(toolNames, key)
		}
		sort.Strings(toolNames)
		for _, key := range toolNames {
			fmt.Printf("    %s: %d\n", key, agg.StreamSums.ToolUseByName[key])
		}
	}

	if len(agg.ByModel) > 0 {
		fmt.Println()
		fmt.Println("By Model")
		models := make([]string, 0, len(agg.ByModel))
		for model := range agg.ByModel {
			models = append(models, model)
		}
		sort.Strings(models)

		for _, model := range models {
			metric := agg.ByModel[model]
			fmt.Printf("  %s\n", model)
			fmt.Printf("    Input Tokens: %d\n", metric.InputTokens)
			fmt.Printf("    Output Tokens: %d\n", metric.OutputTokens)
			fmt.Printf("    Cache Read Tokens: %d\n", metric.CacheReadInputTokens)
			fmt.Printf("    Cache Create Tokens: %d\n", metric.CacheCreationInputTokens)
			fmt.Printf("    Web Search Requests: %d\n", metric.WebSearchRequests)
			fmt.Printf("    Cost(USD): %.6f\n", metric.CostUSD)
		}
	}

	if len(agg.SkippedFiles) > 0 {
		fmt.Println()
		fmt.Println("Skipped Files")
		for _, name := range agg.SkippedFiles {
			fmt.Printf("  - %s\n", name)
		}
	}
}
