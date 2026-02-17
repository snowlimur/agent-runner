package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadValidConfig(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	path := filepath.Join(cwd, ".agent-cli", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	content := `[docker]
image = "claude:go"
model = "sonnet"
enable_dind = true
run_idle_timeout_sec = 123
pipeline_task_idle_timeout_sec = 45

[auth]
github_token = "gh-token"
claude_token = "claude-token"

[workspace]
source_workspace_dir = "/workspace-source"

[git]
user_name = "Test User"
user_email = "test@example.com"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(cwd)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Docker.Image != "claude:go" {
		t.Fatalf("unexpected image: %q", cfg.Docker.Image)
	}
	if cfg.Git.UserEmail != "test@example.com" {
		t.Fatalf("unexpected git email: %q", cfg.Git.UserEmail)
	}
	if cfg.Docker.Model != "sonnet" {
		t.Fatalf("unexpected model: %q", cfg.Docker.Model)
	}
	if !cfg.Docker.EnableDinD {
		t.Fatalf("expected enable_dind=true")
	}
	if cfg.Docker.RunIdleTimeoutSec != 123 {
		t.Fatalf("unexpected run idle timeout: %d", cfg.Docker.RunIdleTimeoutSec)
	}
	if cfg.Docker.PipelineTaskIdleTimeoutSec != 45 {
		t.Fatalf("unexpected pipeline task idle timeout: %d", cfg.Docker.PipelineTaskIdleTimeoutSec)
	}
}

func TestLoadMissingFile(t *testing.T) {
	t.Parallel()

	_, err := Load(t.TempDir())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "config file not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadMissingRequiredField(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	path := filepath.Join(cwd, ".agent-cli", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	content := `[docker]
image = ""

[auth]
github_token = ""
claude_token = ""

[workspace]
source_workspace_dir = "workspace-source"

[git]
user_name = ""
user_email = ""
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(cwd)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "missing required config fields") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadDefaultModelWhenMissing(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	path := filepath.Join(cwd, ".agent-cli", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	content := `[docker]
image = "claude:go"

[auth]
github_token = "gh-token"
claude_token = "claude-token"

[workspace]
source_workspace_dir = "/workspace-source"

[git]
user_name = "Test User"
user_email = "test@example.com"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(cwd)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Docker.Model != DefaultDockerModel {
		t.Fatalf("expected default model %q, got %q", DefaultDockerModel, cfg.Docker.Model)
	}
	if cfg.Docker.EnableDinD {
		t.Fatalf("expected enable_dind default to false")
	}
	if cfg.Docker.RunIdleTimeoutSec != DefaultRunIdleTimeoutSec {
		t.Fatalf("expected default run idle timeout %d, got %d", DefaultRunIdleTimeoutSec, cfg.Docker.RunIdleTimeoutSec)
	}
	if cfg.Docker.PipelineTaskIdleTimeoutSec != DefaultPipelineTaskIdleTimeoutSec {
		t.Fatalf(
			"expected default pipeline task idle timeout %d, got %d",
			DefaultPipelineTaskIdleTimeoutSec,
			cfg.Docker.PipelineTaskIdleTimeoutSec,
		)
	}
}

func TestLoadInvalidModel(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	path := filepath.Join(cwd, ".agent-cli", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	content := `[docker]
image = "claude:go"
model = "bad-model"

[auth]
github_token = "gh-token"
claude_token = "claude-token"

[workspace]
source_workspace_dir = "/workspace-source"

[git]
user_name = "Test User"
user_email = "test@example.com"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(cwd)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "docker.model must be one of") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadInvalidEnableDinD(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	path := filepath.Join(cwd, ".agent-cli", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	content := `[docker]
image = "claude:go"
enable_dind = maybe

[auth]
github_token = "gh-token"
claude_token = "claude-token"

[workspace]
source_workspace_dir = "/workspace-source"

[git]
user_name = "Test User"
user_email = "test@example.com"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(cwd)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid docker.enable_dind") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadInvalidRunIdleTimeout(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	path := filepath.Join(cwd, ".agent-cli", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	content := `[docker]
image = "claude:go"
run_idle_timeout_sec = 0

[auth]
github_token = "gh-token"
claude_token = "claude-token"

[workspace]
source_workspace_dir = "/workspace-source"

[git]
user_name = "Test User"
user_email = "test@example.com"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(cwd)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid docker.run_idle_timeout_sec") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadInvalidPipelineTaskIdleTimeout(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	path := filepath.Join(cwd, ".agent-cli", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	content := `[docker]
image = "claude:go"
pipeline_task_idle_timeout_sec = nope

[auth]
github_token = "gh-token"
claude_token = "claude-token"

[workspace]
source_workspace_dir = "/workspace-source"

[git]
user_name = "Test User"
user_email = "test@example.com"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(cwd)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid docker.pipeline_task_idle_timeout_sec") {
		t.Fatalf("unexpected error: %v", err)
	}
}
