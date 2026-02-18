package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func TestParseRunArgsDebugFlag(t *testing.T) {
	t.Parallel()

	opts, err := parseRunArgs(t.TempDir(), []string{"--debug", "build", "and", "test"})
	if err != nil {
		t.Fatalf("parse args: %v", err)
	}
	if !opts.Debug {
		t.Fatal("expected debug flag to be enabled")
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

func TestParseRunArgsPipelineWithTemplateVars(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	planFile := filepath.Join(cwd, "pipeline.yaml")
	if err := os.WriteFile(planFile, []byte("version: v1\nstages:\n  - id: s\n    mode: sequential\n    tasks:\n      - id: t\n        prompt: hi\n"), 0o644); err != nil {
		t.Fatalf("write plan file: %v", err)
	}

	opts, err := parseRunArgs(cwd, []string{"--pipeline", planFile, "--var", "A_VAR=1", "--var", "B_VAR=2"})
	if err != nil {
		t.Fatalf("parse args: %v", err)
	}

	assertTemplateVars(t, opts.TemplateVars, map[string]string{
		"A_VAR": "1",
		"B_VAR": "2",
	})
}

func TestParseRunArgsTemplateVarRequiresPipeline(t *testing.T) {
	t.Parallel()

	_, err := parseRunArgs(t.TempDir(), []string{"--var", "A_VAR=1", "build"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "--var is only supported with --pipeline") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseRunArgsTemplateVarRejectsInvalidName(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	planFile := filepath.Join(cwd, "pipeline.yaml")
	if err := os.WriteFile(planFile, []byte("version: v1\nstages:\n  - id: s\n    mode: sequential\n    tasks:\n      - id: t\n        prompt: hi\n"), 0o644); err != nil {
		t.Fatalf("write plan file: %v", err)
	}

	_, err := parseRunArgs(cwd, []string{"--pipeline", planFile, "--var", "a_var=1"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid --var name") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseRunArgsTemplateVarRejectsDuplicateKey(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	planFile := filepath.Join(cwd, "pipeline.yaml")
	if err := os.WriteFile(planFile, []byte("version: v1\nstages:\n  - id: s\n    mode: sequential\n    tasks:\n      - id: t\n        prompt: hi\n"), 0o644); err != nil {
		t.Fatalf("write plan file: %v", err)
	}

	_, err := parseRunArgs(cwd, []string{"--pipeline", planFile, "--var", "A_VAR=1", "--var", "A_VAR=2"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "duplicate --var key") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseRunArgsTemplateVarRejectsInvalidFormat(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	planFile := filepath.Join(cwd, "pipeline.yaml")
	if err := os.WriteFile(planFile, []byte("version: v1\nstages:\n  - id: s\n    mode: sequential\n    tasks:\n      - id: t\n        prompt: hi\n"), 0o644); err != nil {
		t.Fatalf("write plan file: %v", err)
	}

	_, err := parseRunArgs(cwd, []string{"--pipeline", planFile, "--var", "A_VAR"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid --var") {
		t.Fatalf("unexpected error: %v", err)
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

func assertTemplateVars(t *testing.T, got map[string]string, want map[string]string) {
	t.Helper()

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected template vars: got=%v want=%v", got, want)
	}
}
