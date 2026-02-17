package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseRunArgsInline(t *testing.T) {
	t.Parallel()

	opts, err := parseRunArgs(t.TempDir(), []string{"build", "and", "test"})
	if err != nil {
		t.Fatalf("parse args: %v", err)
	}
	if opts.Prompt != "build and test" {
		t.Fatalf("unexpected prompt: %q", opts.Prompt)
	}
	if opts.Model != "" {
		t.Fatalf("unexpected model override: %q", opts.Model)
	}
}

func TestParseRunArgsFile(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	file := filepath.Join(cwd, "prompt.txt")
	if err := os.WriteFile(file, []byte(" run tests "), 0o644); err != nil {
		t.Fatalf("write prompt file: %v", err)
	}

	opts, err := parseRunArgs(cwd, []string{"--file", file})
	if err != nil {
		t.Fatalf("parse args: %v", err)
	}
	if opts.Prompt != "run tests" {
		t.Fatalf("unexpected prompt: %q", opts.Prompt)
	}
	if opts.Model != "" {
		t.Fatalf("unexpected model override: %q", opts.Model)
	}
}

func TestParseRunArgsMutualExclusion(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	file := filepath.Join(cwd, "prompt.txt")
	if err := os.WriteFile(file, []byte("prompt"), 0o644); err != nil {
		t.Fatalf("write prompt file: %v", err)
	}

	_, err := parseRunArgs(cwd, []string{"--file", file, "extra"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseRunArgsEmptyPrompt(t *testing.T) {
	t.Parallel()

	_, err := parseRunArgs(t.TempDir(), []string{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseRunArgsModelOverride(t *testing.T) {
	t.Parallel()

	opts, err := parseRunArgs(t.TempDir(), []string{"--model", "sonnet", "build", "and", "test"})
	if err != nil {
		t.Fatalf("parse args: %v", err)
	}
	if opts.Model != "sonnet" {
		t.Fatalf("unexpected model override: %q", opts.Model)
	}
}

func TestParseRunArgsModelOverrideWithFile(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	file := filepath.Join(cwd, "prompt.txt")
	if err := os.WriteFile(file, []byte(" run tests "), 0o644); err != nil {
		t.Fatalf("write prompt file: %v", err)
	}

	opts, err := parseRunArgs(cwd, []string{"--model", "opus", "--file", file})
	if err != nil {
		t.Fatalf("parse args: %v", err)
	}
	if opts.Model != "opus" {
		t.Fatalf("unexpected model override: %q", opts.Model)
	}
}

func TestParseRunArgsInvalidModelOverride(t *testing.T) {
	t.Parallel()

	_, err := parseRunArgs(t.TempDir(), []string{"--model", "bad", "build"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseRunArgsPipeline(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	planFile := filepath.Join(cwd, "pipeline.yaml")
	if err := os.WriteFile(planFile, []byte("version: v1\nstages:\n  - id: s\n    mode: sequential\n    tasks:\n      - id: t\n        prompt: hi\n"), 0o644); err != nil {
		t.Fatalf("write plan file: %v", err)
	}

	opts, err := parseRunArgs(cwd, []string{"--pipeline", planFile})
	if err != nil {
		t.Fatalf("parse args: %v", err)
	}
	if opts.Pipeline != planFile {
		t.Fatalf("unexpected pipeline path: %q", opts.Pipeline)
	}
}

func TestParseRunArgsPipelineMutualExclusionWithPrompt(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	planFile := filepath.Join(cwd, "pipeline.yaml")
	if err := os.WriteFile(planFile, []byte("version: v1\nstages: []\n"), 0o644); err != nil {
		t.Fatalf("write plan file: %v", err)
	}

	_, err := parseRunArgs(cwd, []string{"--pipeline", planFile, "build"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseRunArgsPipelineMutualExclusionWithPromptFile(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	planFile := filepath.Join(cwd, "pipeline.yaml")
	promptFile := filepath.Join(cwd, "prompt.txt")
	if err := os.WriteFile(planFile, []byte("version: v1\nstages: []\n"), 0o644); err != nil {
		t.Fatalf("write plan file: %v", err)
	}
	if err := os.WriteFile(promptFile, []byte("build"), 0o644); err != nil {
		t.Fatalf("write prompt file: %v", err)
	}

	_, err := parseRunArgs(cwd, []string{"--pipeline", planFile, "--file", promptFile})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseRunArgsRejectsLegacyPlanFileFlag(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	planFile := filepath.Join(cwd, "pipeline.yaml")
	if err := os.WriteFile(planFile, []byte("version: v1\nstages: []\n"), 0o644); err != nil {
		t.Fatalf("write pipeline file: %v", err)
	}

	_, err := parseRunArgs(cwd, []string{"--plan-file", planFile})
	if err == nil {
		t.Fatal("expected error")
	}
}
