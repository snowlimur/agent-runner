package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const (
	configDirName  = ".agent-cli"
	configFileName = "config.toml"

	DockerModelSonnet  = "sonnet"
	DockerModelOpus    = "opus"
	DefaultDockerModel = DockerModelOpus

	DockerModeNone    = "none"
	DockerModeDinD    = "dind"
	DockerModeDooD    = "dood"
	DefaultDockerMode = DockerModeNone

	DinDStorageDriverOverlay2 = "overlay2"
	DinDStorageDriverVFS      = "vfs"

	DefaultRunIdleTimeoutSec          = 7200
	DefaultPipelineTaskIdleTimeoutSec = 1800
)

// Config is the root configuration for agent-cli.
type Config struct {
	Docker    DockerConfig    `toml:"docker"`
	Auth      AuthConfig      `toml:"auth"`
	Workspace WorkspaceConfig `toml:"workspace"`
	Git       GitConfig       `toml:"git"`
}

type DockerConfig struct {
	Image                      string `toml:"image"`
	Model                      string `toml:"model"`
	Mode                       string `toml:"mode"`
	DinDStorageDriver          string `toml:"dind_storage_driver"`
	RunIdleTimeoutSec          int    `toml:"run_idle_timeout_sec"`
	PipelineTaskIdleTimeoutSec int    `toml:"pipeline_task_idle_timeout_sec"`
}

type AuthConfig struct {
	GitHubToken string `toml:"github_token"`
	ClaudeToken string `toml:"claude_token"`
}

type WorkspaceConfig struct {
	SourceWorkspaceDir string `toml:"source_workspace_dir"`
}

type GitConfig struct {
	UserName  string `toml:"user_name"`
	UserEmail string `toml:"user_email"`
}

func ConfigPath(cwd string) string {
	return filepath.Join(cwd, configDirName, configFileName)
}

func StatsDir(cwd string) string {
	return filepath.Join(cwd, configDirName, "stats")
}

func RunsDir(cwd string) string {
	return filepath.Join(cwd, configDirName, "runs")
}

func Load(cwd string) (*Config, error) {
	path := ConfigPath(cwd)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("config file not found: %s", path)
		}
		return nil, fmt.Errorf("stat config file: %w", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	cfg, err := parseConfigTOML(string(content))
	if err != nil {
		return nil, fmt.Errorf("decode config TOML: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	missing := make([]string, 0, 6)
	if strings.TrimSpace(c.Docker.Image) == "" {
		missing = append(missing, "docker.image")
	}

	c.Docker.Model = normalizeDockerModel(c.Docker.Model)
	if c.Docker.Model == "" {
		c.Docker.Model = DefaultDockerModel
	}
	if !IsValidDockerModel(c.Docker.Model) {
		return fmt.Errorf("docker.model must be one of: %s, %s", DockerModelSonnet, DockerModelOpus)
	}

	c.Docker.Mode = normalizeDockerMode(c.Docker.Mode)
	if c.Docker.Mode == "" {
		c.Docker.Mode = DefaultDockerMode
	}
	if !IsValidDockerMode(c.Docker.Mode) {
		return fmt.Errorf(
			"docker.mode must be one of: %s, %s, %s",
			DockerModeNone,
			DockerModeDinD,
			DockerModeDooD,
		)
	}

	c.Docker.DinDStorageDriver = normalizeDinDStorageDriver(c.Docker.DinDStorageDriver)
	if c.Docker.DinDStorageDriver == "" {
		c.Docker.DinDStorageDriver = DefaultDinDStorageDriverForGOOS(runtime.GOOS)
	}
	if !IsValidDinDStorageDriver(c.Docker.DinDStorageDriver) {
		return fmt.Errorf(
			"docker.dind_storage_driver must be one of: %s, %s",
			DinDStorageDriverOverlay2,
			DinDStorageDriverVFS,
		)
	}

	if c.Docker.RunIdleTimeoutSec <= 0 {
		c.Docker.RunIdleTimeoutSec = DefaultRunIdleTimeoutSec
	}
	if c.Docker.PipelineTaskIdleTimeoutSec <= 0 {
		c.Docker.PipelineTaskIdleTimeoutSec = DefaultPipelineTaskIdleTimeoutSec
	}

	if strings.TrimSpace(c.Auth.GitHubToken) == "" {
		missing = append(missing, "auth.github_token")
	}
	if strings.TrimSpace(c.Auth.ClaudeToken) == "" {
		missing = append(missing, "auth.claude_token")
	}
	if strings.TrimSpace(c.Workspace.SourceWorkspaceDir) == "" {
		missing = append(missing, "workspace.source_workspace_dir")
	}
	if strings.TrimSpace(c.Git.UserName) == "" {
		missing = append(missing, "git.user_name")
	}
	if strings.TrimSpace(c.Git.UserEmail) == "" {
		missing = append(missing, "git.user_email")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required config fields: %s", strings.Join(missing, ", "))
	}

	if !filepath.IsAbs(c.Workspace.SourceWorkspaceDir) {
		return fmt.Errorf("workspace.source_workspace_dir must be an absolute path: %q", c.Workspace.SourceWorkspaceDir)
	}

	return nil
}

func parseConfigTOML(content string) (*Config, error) {
	cfg := &Config{}
	section := ""

	lines := strings.Split(content, "\n")
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("line %d: expected key = value", i+1)
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(stripInlineComment(value))
		parsedValue, err := parseStringValue(value)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}

		if err := setConfigField(cfg, section, key, parsedValue); err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
	}

	return cfg, nil
}

func stripInlineComment(value string) string {
	inQuotes := false
	escaped := false

	for i, r := range value {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && inQuotes {
			escaped = true
			continue
		}
		if r == '"' {
			inQuotes = !inQuotes
			continue
		}
		if r == '#' && !inQuotes {
			return strings.TrimSpace(value[:i])
		}
	}

	return value
}

func parseStringValue(raw string) (string, error) {
	// Support quoted TOML string values used by agent-cli config.
	if strings.HasPrefix(raw, "\"") && strings.HasSuffix(raw, "\"") {
		value, err := strconv.Unquote(raw)
		if err != nil {
			return "", fmt.Errorf("invalid quoted string: %w", err)
		}
		return value, nil
	}
	return raw, nil
}

func setConfigField(cfg *Config, section, key, value string) error {
	switch section {
	case "docker":
		if key == "image" {
			cfg.Docker.Image = value
			return nil
		}
		if key == "model" {
			cfg.Docker.Model = value
			return nil
		}
		if key == "mode" {
			cfg.Docker.Mode = value
			return nil
		}
		if key == "dind_storage_driver" {
			cfg.Docker.DinDStorageDriver = value
			return nil
		}
		if key == "run_idle_timeout_sec" {
			timeoutSec, err := parsePositiveIntValue(value)
			if err != nil {
				return fmt.Errorf("invalid docker.run_idle_timeout_sec: %w", err)
			}
			cfg.Docker.RunIdleTimeoutSec = timeoutSec
			return nil
		}
		if key == "pipeline_task_idle_timeout_sec" {
			timeoutSec, err := parsePositiveIntValue(value)
			if err != nil {
				return fmt.Errorf("invalid docker.pipeline_task_idle_timeout_sec: %w", err)
			}
			cfg.Docker.PipelineTaskIdleTimeoutSec = timeoutSec
			return nil
		}
	case "auth":
		if key == "github_token" {
			cfg.Auth.GitHubToken = value
			return nil
		}
		if key == "claude_token" {
			cfg.Auth.ClaudeToken = value
			return nil
		}
	case "workspace":
		if key == "source_workspace_dir" {
			cfg.Workspace.SourceWorkspaceDir = value
			return nil
		}
	case "git":
		if key == "user_name" {
			cfg.Git.UserName = value
			return nil
		}
		if key == "user_email" {
			cfg.Git.UserEmail = value
			return nil
		}
	default:
		return fmt.Errorf("unknown section %q", section)
	}
	return fmt.Errorf("unknown key %q in section %q", key, section)
}

func normalizeDockerModel(model string) string {
	return strings.ToLower(strings.TrimSpace(model))
}

func parsePositiveIntValue(raw string) (int, error) {
	value := strings.TrimSpace(raw)
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("expected positive integer, got %q", raw)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("expected positive integer, got %q", raw)
	}
	return parsed, nil
}

func IsValidDockerModel(model string) bool {
	switch normalizeDockerModel(model) {
	case DockerModelSonnet, DockerModelOpus:
		return true
	default:
		return false
	}
}

func normalizeDockerMode(mode string) string {
	return strings.ToLower(strings.TrimSpace(mode))
}

func IsValidDockerMode(mode string) bool {
	switch normalizeDockerMode(mode) {
	case DockerModeNone, DockerModeDinD, DockerModeDooD:
		return true
	default:
		return false
	}
}

func normalizeDinDStorageDriver(driver string) string {
	return strings.ToLower(strings.TrimSpace(driver))
}

func IsValidDinDStorageDriver(driver string) bool {
	switch normalizeDinDStorageDriver(driver) {
	case DinDStorageDriverOverlay2, DinDStorageDriverVFS:
		return true
	default:
		return false
	}
}

func DefaultDinDStorageDriverForGOOS(goos string) string {
	if strings.EqualFold(strings.TrimSpace(goos), "linux") {
		return DinDStorageDriverOverlay2
	}
	return DinDStorageDriverVFS
}
