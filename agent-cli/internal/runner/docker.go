package runner

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/stdcopy"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	defaultModel                    = "opus"
	defaultDockerMode               = dockerModeNone
	defaultRunIdleTimeoutSeconds    = 7200
	defaultPipelineTaskIdleTimeout  = 1800
	containerWorkspaceDir           = "/workspace"
	hostDockerSocketPath            = "/var/run/docker.sock"
	containerStopTimeoutSeconds     = 10
	containerCleanupTimeout         = 15 * time.Second
	interruptedLogDrainTimeout      = 5 * time.Second
	postExitLogDrainTimeout         = 15 * time.Second
	runIdleCheckInterval            = 250 * time.Millisecond
	interruptedExitCode             = 130
	managedContainerLabelKey        = "agent-cli.managed"
	managedContainerLabelValue      = "true"
	managedContainerCWDHashLabelKey = "agent-cli.cwd_hash"

	dockerModeNone = "none"
	dockerModeDinD = "dind"
	dockerModeDooD = "dood"

	dindStorageDriverOverlay2 = "overlay2"
	dindStorageDriverVFS      = "vfs"
)

var (
	// ErrInterrupted marks an interrupted run (for example Ctrl+C/SIGTERM).
	ErrInterrupted = errors.New("run interrupted")
	// ErrIdleTimeout marks a timeout when there is no stdout/stderr activity for too long.
	ErrIdleTimeout = errors.New("run idle timeout")

	newDockerAPIFn = func() (dockerAPI, error) {
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			return nil, err
		}
		return cli, nil
	}
)

type dockerAPI interface {
	Close() error
	ImagePull(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error)
	ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
	ContainerCreate(
		ctx context.Context,
		config *container.Config,
		hostConfig *container.HostConfig,
		networkingConfig *network.NetworkingConfig,
		platform *ocispec.Platform,
		containerName string,
	) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
	ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error)
	ContainerWait(
		ctx context.Context,
		containerID string,
		condition container.WaitCondition,
	) (<-chan container.WaitResponse, <-chan error)
	ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error
	ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error
}

type runSpec struct {
	HostDir           string
	Model             string
	CommandArgs       []string
	Env               []string
	Labels            map[string]string
	CWDHash           string
	DockerMode        string
	DinDStorageDriver string
}

type RunRequest struct {
	Image                      string
	CWD                        string
	SourceWorkspaceDir         string
	GitHubToken                string
	ClaudeToken                string
	GitUserName                string
	GitUserEmail               string
	Prompt                     string
	Pipeline                   string
	TemplateVars               map[string]string
	Model                      string
	Debug                      bool
	DockerMode                 string
	DinDStorageDriver          string
	RunIdleTimeoutSec          int
	PipelineTaskIdleTimeoutSec int
}

type RunOutput struct {
	Args     []string
	Stdout   string
	Stderr   string
	ExitCode int
}

type StreamHooks struct {
	OnStdoutLine func(line string)
	OnStderrLine func(line string)
}

func RunDockerStreaming(ctx context.Context, req RunRequest, hooks StreamHooks) (RunOutput, error) {
	spec, err := buildRunSpec(req)
	if err != nil {
		return RunOutput{}, err
	}

	output := RunOutput{
		Args: append([]string(nil), spec.CommandArgs...),
	}

	runIdleTimeout := resolveRunIdleTimeout(req.RunIdleTimeoutSec)
	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()

	var lastActivityUnixNano atomic.Int64
	lastActivityUnixNano.Store(time.Now().UnixNano())
	var idleTimedOut atomic.Bool

	touchActivity := func() {
		lastActivityUnixNano.Store(time.Now().UnixNano())
	}

	go monitorRunIdleTimeout(runCtx, runIdleTimeout, &lastActivityUnixNano, &idleTimedOut, cancelRun)

	wrappedHooks := StreamHooks{
		OnStdoutLine: func(line string) {
			touchActivity()
			if hooks.OnStdoutLine != nil {
				hooks.OnStdoutLine(line)
			}
		},
		OnStderrLine: func(line string) {
			touchActivity()
			if hooks.OnStderrLine != nil {
				hooks.OnStderrLine(line)
			}
		},
	}

	if runCtx.Err() != nil {
		exitCode, cancelErr := runCancellationError(runIdleTimeout, &idleTimedOut)
		output.ExitCode = exitCode
		return output, cancelErr
	}

	dockerClient, err := newDockerAPIFn()
	if err != nil {
		output.ExitCode = -1
		return output, fmt.Errorf("create docker client: %w", err)
	}
	defer dockerClient.Close()

	if err := cleanupStaleContainers(runCtx, dockerClient, spec.CWDHash); err != nil {
		if runCtx.Err() != nil || isContextCanceledError(err) {
			exitCode, cancelErr := runCancellationError(runIdleTimeout, &idleTimedOut)
			output.ExitCode = exitCode
			return output, cancelErr
		}
		output.ExitCode = -1
		return output, fmt.Errorf("cleanup stale containers: %w", err)
	}

	if err := pullImageBestEffort(runCtx, dockerClient, req.Image); err != nil && runCtx.Err() != nil {
		exitCode, cancelErr := runCancellationError(runIdleTimeout, &idleTimedOut)
		output.ExitCode = exitCode
		return output, cancelErr
	}

	containerConfig := &container.Config{
		Image:        req.Image,
		Env:          spec.Env,
		Cmd:          spec.CommandArgs,
		AttachStdout: true,
		AttachStderr: true,
		Labels:       spec.Labels,
	}

	networkMode := container.NetworkMode("host")
	privileged := false
	binds := []string{
		fmt.Sprintf("%s:%s:ro", spec.HostDir, req.SourceWorkspaceDir),
	}

	if spec.DockerMode == dockerModeDinD {
		// DinD daemon should not share host network namespace; otherwise it can mutate
		// host iptables rules and break host-side docker compose.
		networkMode = container.NetworkMode("bridge")
		privileged = true
	}

	if spec.DockerMode == dockerModeDooD {
		binds = append(binds, fmt.Sprintf("%s:%s", hostDockerSocketPath, hostDockerSocketPath))
	}

	hostConfig := &container.HostConfig{
		NetworkMode: networkMode,
		AutoRemove:  true,
		Privileged:  privileged,
		Binds:       binds,
	}

	createResp, err := dockerClient.ContainerCreate(runCtx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		if runCtx.Err() != nil || isContextCanceledError(err) {
			exitCode, cancelErr := runCancellationError(runIdleTimeout, &idleTimedOut)
			output.ExitCode = exitCode
			return output, cancelErr
		}
		output.ExitCode = -1
		return output, fmt.Errorf("create container: %w", err)
	}

	containerID := createResp.ID
	cleanup := makeCleanupOnce(dockerClient, containerID)

	if err := dockerClient.ContainerStart(runCtx, containerID, container.StartOptions{}); err != nil {
		cleanupErr := cleanup()
		if runCtx.Err() != nil || isContextCanceledError(err) {
			exitCode, cancelErr := runCancellationError(runIdleTimeout, &idleTimedOut, cleanupErr)
			output.ExitCode = exitCode
			return output, cancelErr
		}
		output.ExitCode = -1
		if cleanupErr != nil {
			return output, fmt.Errorf("start container: %w; cleanup failed: %v", err, cleanupErr)
		}
		return output, fmt.Errorf("start container: %w", err)
	}

	logsReader, err := dockerClient.ContainerLogs(runCtx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	})
	if err != nil {
		cleanupErr := cleanup()
		if runCtx.Err() != nil || isContextCanceledError(err) {
			exitCode, cancelErr := runCancellationError(runIdleTimeout, &idleTimedOut, cleanupErr)
			output.ExitCode = exitCode
			return output, cancelErr
		}
		output.ExitCode = -1
		if cleanupErr != nil {
			return output, fmt.Errorf("open container logs: %w; cleanup failed: %v", err, cleanupErr)
		}
		return output, fmt.Errorf("open container logs: %w", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	streamErrCh := make(chan error, 1)
	go func() {
		streamErrCh <- streamContainerLogs(logsReader, &stdout, &stderr, wrappedHooks)
	}()

	statusCh, waitErrCh := dockerClient.ContainerWait(runCtx, containerID, container.WaitConditionNotRunning)
	waitResp, waitErr := waitForContainer(runCtx, statusCh, waitErrCh)

	if waitErr != nil {
		if runCtx.Err() != nil || isContextCanceledError(waitErr) {
			cleanupErr := cleanup()
			streamErr := waitForStreamErrorWithTimeout(streamErrCh, interruptedLogDrainTimeout)
			output.Stdout = stdout.String()
			output.Stderr = stderr.String()
			exitCode, cancelErr := runCancellationError(runIdleTimeout, &idleTimedOut, cleanupErr, streamErr)
			output.ExitCode = exitCode
			return output, cancelErr
		}

		cleanupErr := cleanup()
		streamErr := waitForStreamErrorWithTimeout(streamErrCh, interruptedLogDrainTimeout)
		output.Stdout = stdout.String()
		output.Stderr = stderr.String()
		output.ExitCode = -1

		if streamErr != nil {
			return output, fmt.Errorf("wait for container: %w; log stream error: %v", waitErr, streamErr)
		}
		if cleanupErr != nil {
			return output, fmt.Errorf("wait for container: %w; cleanup failed: %v", waitErr, cleanupErr)
		}
		return output, fmt.Errorf("wait for container: %w", waitErr)
	}

	streamErr := waitForStreamErrorWithTimeout(streamErrCh, postExitLogDrainTimeout)
	output.Stdout = stdout.String()
	output.Stderr = stderr.String()

	if streamErr != nil {
		output.ExitCode = -1
		return output, streamErr
	}

	output.ExitCode = int(waitResp.StatusCode)
	if output.ExitCode != 0 {
		return output, fmt.Errorf("container exited with code %d", output.ExitCode)
	}

	return output, nil
}

func buildRunSpec(req RunRequest) (runSpec, error) {
	if strings.TrimSpace(req.Image) == "" {
		return runSpec{}, errors.New("docker image is required")
	}
	if strings.TrimSpace(req.CWD) == "" {
		return runSpec{}, errors.New("cwd is required")
	}
	if strings.TrimSpace(req.SourceWorkspaceDir) == "" {
		return runSpec{}, errors.New("source workspace dir is required")
	}
	prompt := strings.TrimSpace(req.Prompt)
	pipeline := strings.TrimSpace(req.Pipeline)
	modeCount := 0
	if prompt != "" {
		modeCount++
	}
	if pipeline != "" {
		modeCount++
	}
	if modeCount == 0 {
		return runSpec{}, errors.New("prompt or pipeline is required")
	}
	if modeCount > 1 {
		return runSpec{}, errors.New("use exactly one input source: prompt or pipeline")
	}

	model := strings.ToLower(strings.TrimSpace(req.Model))
	if model == "" {
		model = defaultModel
	}
	if model != "sonnet" && model != "opus" {
		return runSpec{}, errors.New("model must be one of: sonnet, opus")
	}

	dockerMode := normalizeDockerMode(req.DockerMode)
	if dockerMode == "" {
		dockerMode = defaultDockerMode
	}
	if !isValidDockerMode(dockerMode) {
		return runSpec{}, fmt.Errorf(
			"docker mode must be one of: %s, %s, %s",
			dockerModeNone,
			dockerModeDinD,
			dockerModeDooD,
		)
	}

	dindStorageDriver := normalizeDinDStorageDriver(req.DinDStorageDriver)
	if dindStorageDriver == "" {
		dindStorageDriver = defaultDinDStorageDriverForGOOS(runtime.GOOS)
	}
	if !isValidDinDStorageDriver(dindStorageDriver) {
		return runSpec{}, fmt.Errorf(
			"dind storage driver must be one of: %s, %s",
			dindStorageDriverOverlay2,
			dindStorageDriverVFS,
		)
	}

	hostDir, err := filepath.Abs(req.CWD)
	if err != nil {
		return runSpec{}, fmt.Errorf("resolve cwd: %w", err)
	}

	cwdHash := hashString(hostDir)
	labels := map[string]string{
		managedContainerLabelKey:        managedContainerLabelValue,
		managedContainerCWDHashLabelKey: cwdHash,
	}
	pipelineNodeTimeoutSec := resolvePipelineTaskIdleTimeoutSec(req.PipelineTaskIdleTimeoutSec)
	env := []string{
		"GH_TOKEN=" + req.GitHubToken,
		"CLAUDE_CODE_OAUTH_TOKEN=" + req.ClaudeToken,
		"SOURCE_WORKSPACE_DIR=" + req.SourceWorkspaceDir,
		"GIT_USER_NAME=" + req.GitUserName,
		"GIT_USER_EMAIL=" + req.GitUserEmail,
		fmt.Sprintf("PIPELINE_AGENT_IDLE_TIMEOUT_SEC=%d", pipelineNodeTimeoutSec),
		fmt.Sprintf("PIPELINE_COMMAND_TIMEOUT_SEC=%d", pipelineNodeTimeoutSec),
		"FORCE_COLOR=1",
	}

	var commandArgs []string
	baseArgs := []string{"--model", model}
	if req.Debug {
		baseArgs = append(baseArgs, "--debug")
	}

	switch {
	case prompt != "":
		commandArgs = append(baseArgs, "-vv", "-v", prompt)
	case pipeline != "":
		containerPipelinePath, err := resolveContainerPipelineFilePath(hostDir, pipeline)
		if err != nil {
			return runSpec{}, err
		}
		commandArgs = append(baseArgs, "--pipeline", containerPipelinePath)

		if len(req.TemplateVars) > 0 {
			keys := make([]string, 0, len(req.TemplateVars))
			for key := range req.TemplateVars {
				keys = append(keys, key)
			}
			sort.Strings(keys)

			for _, key := range keys {
				commandArgs = append(commandArgs, "--var", key+"="+req.TemplateVars[key])
			}
		}
	}

	if dockerMode == dockerModeDinD {
		env = append(env, "ENABLE_DIND=1", "DIND_STORAGE_DRIVER="+dindStorageDriver)
	}

	return runSpec{
		HostDir:           hostDir,
		Model:             model,
		CommandArgs:       commandArgs,
		Env:               env,
		Labels:            labels,
		CWDHash:           cwdHash,
		DockerMode:        dockerMode,
		DinDStorageDriver: dindStorageDriver,
	}, nil
}

func normalizeDockerMode(mode string) string {
	return strings.ToLower(strings.TrimSpace(mode))
}

func isValidDockerMode(mode string) bool {
	switch normalizeDockerMode(mode) {
	case dockerModeNone, dockerModeDinD, dockerModeDooD:
		return true
	default:
		return false
	}
}

func normalizeDinDStorageDriver(driver string) string {
	return strings.ToLower(strings.TrimSpace(driver))
}

func isValidDinDStorageDriver(driver string) bool {
	switch normalizeDinDStorageDriver(driver) {
	case dindStorageDriverOverlay2, dindStorageDriverVFS:
		return true
	default:
		return false
	}
}

func defaultDinDStorageDriverForGOOS(goos string) string {
	if strings.EqualFold(strings.TrimSpace(goos), "linux") {
		return dindStorageDriverOverlay2
	}
	return dindStorageDriverVFS
}

func resolveContainerPipelineFilePath(hostDir string, pipelinePath string) (string, error) {
	trimmed := strings.TrimSpace(pipelinePath)
	if trimmed == "" {
		return "", errors.New("pipeline path is empty")
	}

	hostPipelinePath := trimmed
	if !filepath.IsAbs(hostPipelinePath) {
		hostPipelinePath = filepath.Join(hostDir, hostPipelinePath)
	}

	absPipelinePath, err := filepath.Abs(hostPipelinePath)
	if err != nil {
		return "", fmt.Errorf("resolve pipeline path: %w", err)
	}

	relativePipelinePath, err := filepath.Rel(hostDir, absPipelinePath)
	if err != nil {
		return "", fmt.Errorf("resolve pipeline path relative to cwd: %w", err)
	}

	relativePipelinePath = filepath.Clean(relativePipelinePath)
	if relativePipelinePath == "." {
		return "", errors.New("pipeline path must point to a file inside cwd")
	}
	if relativePipelinePath == ".." || strings.HasPrefix(relativePipelinePath, ".."+string(filepath.Separator)) {
		return "", errors.New("pipeline path must be inside cwd")
	}
	if filepath.IsAbs(relativePipelinePath) {
		return "", errors.New("pipeline path must be a relative path inside cwd")
	}

	return path.Join(containerWorkspaceDir, filepath.ToSlash(relativePipelinePath)), nil
}

func pullImageBestEffort(ctx context.Context, dockerClient dockerAPI, imageRef string) error {
	reader, err := dockerClient.ImagePull(ctx, imageRef, image.PullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()

	return drainReadCloserWithContext(ctx, reader)
}

func cleanupStaleContainers(ctx context.Context, dockerClient dockerAPI, cwdHash string) error {
	filterArgs := filters.NewArgs(
		filters.Arg("label", managedContainerLabelKey+"="+managedContainerLabelValue),
		filters.Arg("label", managedContainerCWDHashLabelKey+"="+cwdHash),
	)

	containers, err := dockerClient.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filterArgs,
	})
	if err != nil {
		return err
	}

	for _, item := range containers {
		if isContainerRunning(item) {
			continue
		}
		if err := dockerClient.ContainerRemove(
			ctx,
			item.ID,
			container.RemoveOptions{Force: true, RemoveVolumes: true},
		); err != nil &&
			!isNotFoundError(err) {
			return fmt.Errorf("remove stale container %s: %w", shortenContainerID(item.ID), err)
		}
	}

	return nil
}

func isContainerRunning(item container.Summary) bool {
	state := strings.ToLower(strings.TrimSpace(item.State))
	if state == "running" {
		return true
	}
	if state == "" {
		status := strings.ToLower(strings.TrimSpace(item.Status))
		if strings.HasPrefix(status, "up") {
			return true
		}
	}
	return false
}

func makeCleanupOnce(dockerClient dockerAPI, containerID string) func() error {
	var once sync.Once
	var cleanupErr error

	return func() error {
		once.Do(func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), containerCleanupTimeout)
			defer cancel()
			cleanupErr = cleanupContainer(cleanupCtx, dockerClient, containerID)
		})
		return cleanupErr
	}
}

func cleanupContainer(ctx context.Context, dockerClient dockerAPI, containerID string) error {
	stopTimeout := containerStopTimeoutSeconds
	stopErr := dockerClient.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &stopTimeout})
	if isIgnorableStopError(stopErr) {
		stopErr = nil
	}

	removeErr := dockerClient.ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})
	if isNotFoundError(removeErr) {
		removeErr = nil
	}

	if stopErr == nil && removeErr == nil {
		return nil
	}
	if stopErr == nil {
		return fmt.Errorf("remove container: %w", removeErr)
	}
	if removeErr == nil {
		return fmt.Errorf("stop container: %w", stopErr)
	}

	return fmt.Errorf("stop container: %w; remove container: %v", stopErr, removeErr)
}

func isIgnorableStopError(err error) bool {
	if err == nil {
		return true
	}
	if errdefs.IsNotFound(err) {
		return true
	}
	if errdefs.IsConflict(err) {
		return true
	}
	return false
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return errdefs.IsNotFound(err)
}

func waitForContainer(
	ctx context.Context,
	statusCh <-chan container.WaitResponse,
	errCh <-chan error,
) (container.WaitResponse, error) {
	for statusCh != nil || errCh != nil {
		select {
		case <-ctx.Done():
			return container.WaitResponse{}, ctx.Err()
		case status, ok := <-statusCh:
			if !ok {
				statusCh = nil
				continue
			}
			return status, nil
		case err, ok := <-errCh:
			if !ok {
				errCh = nil
				continue
			}
			if err != nil {
				return container.WaitResponse{}, err
			}
		}
	}

	return container.WaitResponse{}, errors.New("container wait finished without status")
}

func streamContainerLogs(
	logsReader io.ReadCloser,
	stdoutCollector *bytes.Buffer,
	stderrCollector *bytes.Buffer,
	hooks StreamHooks,
) error {
	defer logsReader.Close()

	stdoutReader, stdoutWriter := io.Pipe()
	stderrReader, stderrWriter := io.Pipe()

	errCh := make(chan error, 3)

	go func() {
		errCh <- streamLines(stdoutReader, stdoutCollector, hooks.OnStdoutLine)
	}()

	go func() {
		errCh <- streamLines(stderrReader, stderrCollector, hooks.OnStderrLine)
	}()

	go func() {
		_, err := stdcopy.StdCopy(stdoutWriter, stderrWriter, logsReader)
		if err != nil {
			_ = stdoutWriter.CloseWithError(err)
			_ = stderrWriter.CloseWithError(err)
			errCh <- fmt.Errorf("demux container logs: %w", err)
			return
		}

		_ = stdoutWriter.Close()
		_ = stderrWriter.Close()
		errCh <- nil
	}()

	var firstErr error
	for i := 0; i < 3; i++ {
		err := <-errCh
		if err == nil || errors.Is(err, io.EOF) {
			continue
		}
		if firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

func waitForStreamErrorWithTimeout(streamErrCh <-chan error, timeout time.Duration) error {
	select {
	case err := <-streamErrCh:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("timed out waiting for log stream to stop after %v", timeout)
	}
}

func drainReadCloserWithContext(ctx context.Context, reader io.ReadCloser) error {
	copyErrCh := make(chan error, 1)
	go func() {
		_, err := io.Copy(io.Discard, reader)
		copyErrCh <- err
	}()

	select {
	case err := <-copyErrCh:
		return err
	case <-ctx.Done():
		_ = reader.Close()
		select {
		case err := <-copyErrCh:
			if err == nil || errors.Is(err, io.EOF) {
				return ctx.Err()
			}
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return ctx.Err()
			}
			return err
		case <-time.After(2 * time.Second):
			return ctx.Err()
		}
	}
}

func monitorRunIdleTimeout(
	ctx context.Context,
	timeout time.Duration,
	lastActivityUnixNano *atomic.Int64,
	idleTimedOut *atomic.Bool,
	cancel context.CancelFunc,
) {
	if timeout <= 0 {
		return
	}

	ticker := time.NewTicker(runIdleCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			lastActivityAt := time.Unix(0, lastActivityUnixNano.Load())
			if time.Since(lastActivityAt) >= timeout {
				idleTimedOut.Store(true)
				cancel()
				return
			}
		}
	}
}

func runCancellationError(runIdleTimeout time.Duration, idleTimedOut *atomic.Bool, extraErrs ...error) (int, error) {
	if idleTimedOut != nil && idleTimedOut.Load() {
		return -1, wrapIdleTimeoutError(runIdleTimeout, extraErrs...)
	}
	return interruptedExitCode, wrapInterruptedError(extraErrs...)
}

func isContextCanceledError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func streamLines(reader io.Reader, collector *bytes.Buffer, onLine func(line string)) error {
	scanner := bufio.NewScanner(reader)
	buffer := make([]byte, 0, 1024*64)
	scanner.Buffer(buffer, 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		collector.WriteString(line)
		collector.WriteByte('\n')
		if onLine != nil {
			onLine(line)
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

func wrapInterruptedError(extraErrs ...error) error {
	details := make([]string, 0, len(extraErrs))
	for _, err := range extraErrs {
		if err == nil {
			continue
		}
		details = append(details, err.Error())
	}

	if len(details) == 0 {
		return fmt.Errorf("%w: run interrupted by signal", ErrInterrupted)
	}

	return fmt.Errorf("%w: run interrupted by signal; %s", ErrInterrupted, strings.Join(details, "; "))
}

func wrapIdleTimeoutError(timeout time.Duration, extraErrs ...error) error {
	details := make([]string, 0, len(extraErrs))
	for _, err := range extraErrs {
		if err == nil {
			continue
		}
		details = append(details, err.Error())
	}

	if len(details) == 0 {
		return fmt.Errorf("%w: no log activity for %v", ErrIdleTimeout, timeout)
	}

	return fmt.Errorf("%w: no log activity for %v; %s", ErrIdleTimeout, timeout, strings.Join(details, "; "))
}

func resolveRunIdleTimeout(timeoutSec int) time.Duration {
	if timeoutSec <= 0 {
		timeoutSec = defaultRunIdleTimeoutSeconds
	}
	return time.Duration(timeoutSec) * time.Second
}

func resolvePipelineTaskIdleTimeoutSec(timeoutSec int) int {
	if timeoutSec <= 0 {
		return defaultPipelineTaskIdleTimeout
	}
	return timeoutSec
}

func hashString(input string) string {
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:])
}

func shortenContainerID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:12]
}
