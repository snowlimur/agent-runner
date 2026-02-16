package config

import (
	"os"
	"path/filepath"
	"runtime"
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
mode = "dind"
dind_storage_driver = "vfs"
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
	if cfg.Docker.Mode != DockerModeDinD {
		t.Fatalf("unexpected docker mode: %q", cfg.Docker.Mode)
	}
	if cfg.Docker.DinDStorageDriver != DinDStorageDriverVFS {
		t.Fatalf("unexpected dind storage driver: %q", cfg.Docker.DinDStorageDriver)
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
	if cfg.Docker.Mode != DefaultDockerMode {
		t.Fatalf("expected default docker mode %q, got %q", DefaultDockerMode, cfg.Docker.Mode)
	}
	expectedDefaultDriver := DefaultDinDStorageDriverForGOOS(runtime.GOOS)
	if cfg.Docker.DinDStorageDriver != expectedDefaultDriver {
		t.Fatalf(
			"expected default dind storage driver %q, got %q",
			expectedDefaultDriver,
			cfg.Docker.DinDStorageDriver,
		)
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

func TestLoadInvalidDockerMode(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	path := filepath.Join(cwd, ".agent-cli", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	content := `[docker]
image = "claude:go"
mode = "bad-mode"

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
	if !strings.Contains(err.Error(), "docker.mode must be one of") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadInvalidDinDStorageDriver(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	path := filepath.Join(cwd, ".agent-cli", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	content := `[docker]
image = "claude:go"
dind_storage_driver = "bad-driver"

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
	if !strings.Contains(err.Error(), "docker.dind_storage_driver must be one of") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadUnknownEnableDinDKey(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	path := filepath.Join(cwd, ".agent-cli", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	content := `[docker]
image = "claude:go"
enable_dind = true

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
	if !strings.Contains(err.Error(), `unknown key "enable_dind" in section "docker"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDefaultDinDStorageDriverForGOOS(t *testing.T) {
	t.Parallel()

	if got := DefaultDinDStorageDriverForGOOS("linux"); got != DinDStorageDriverOverlay2 {
		t.Fatalf("expected overlay2 for linux, got %q", got)
	}
	if got := DefaultDinDStorageDriverForGOOS("darwin"); got != DinDStorageDriverVFS {
		t.Fatalf("expected vfs for darwin, got %q", got)
	}
	if got := DefaultDinDStorageDriverForGOOS("windows"); got != DinDStorageDriverVFS {
		t.Fatalf("expected vfs for windows, got %q", got)
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
