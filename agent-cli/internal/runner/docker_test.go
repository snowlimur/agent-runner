package runner

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/stdcopy"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type removeCall struct {
	containerID string
	options     container.RemoveOptions
}

type fakeDockerAPI struct {
	mu sync.Mutex

	imagePullErr    error
	imagePullReader io.ReadCloser

	listResp    []container.Summary
	listErr     error
	listOptions container.ListOptions

	createResp      container.CreateResponse
	createErr       error
	createdConfig   *container.Config
	createdHost     *container.HostConfig
	createdName     string
	containerStarts []string
	startErr        error
	onStart         func()
	logsReader      io.ReadCloser
	logsErr         error
	waitResp        container.WaitResponse
	waitErr         error
	waitBlocksOnCtx bool
	stopErr         error
	stopCalls       []string
	removeErr       error
	removeErrByID   map[string]error
	removeCalls     []removeCall
	closed          bool
}

func (f *fakeDockerAPI) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

func (f *fakeDockerAPI) ImagePull(_ context.Context, _ string, _ image.PullOptions) (io.ReadCloser, error) {
	if f.imagePullErr != nil {
		return nil, f.imagePullErr
	}
	if f.imagePullReader != nil {
		return f.imagePullReader, nil
	}
	return io.NopCloser(strings.NewReader("{}")), nil
}

func (f *fakeDockerAPI) ContainerList(_ context.Context, options container.ListOptions) ([]container.Summary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listOptions = options
	if f.listErr != nil {
		return nil, f.listErr
	}
	resp := make([]container.Summary, len(f.listResp))
	copy(resp, f.listResp)
	return resp, nil
}

func (f *fakeDockerAPI) ContainerCreate(_ context.Context, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig, _ *ocispec.Platform, containerName string) (container.CreateResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createdConfig = config
	f.createdHost = hostConfig
	f.createdName = containerName
	if f.createErr != nil {
		return container.CreateResponse{}, f.createErr
	}
	if f.createResp.ID == "" {
		f.createResp.ID = "container-created"
	}
	return f.createResp, nil
}

func (f *fakeDockerAPI) ContainerStart(_ context.Context, containerID string, _ container.StartOptions) error {
	f.mu.Lock()
	f.containerStarts = append(f.containerStarts, containerID)
	onStart := f.onStart
	startErr := f.startErr
	f.mu.Unlock()

	if onStart != nil {
		onStart()
	}
	return startErr
}

func (f *fakeDockerAPI) ContainerLogs(_ context.Context, _ string, _ container.LogsOptions) (io.ReadCloser, error) {
	if f.logsErr != nil {
		return nil, f.logsErr
	}
	if f.logsReader == nil {
		return io.NopCloser(bytes.NewReader(nil)), nil
	}
	return f.logsReader, nil
}

func (f *fakeDockerAPI) ContainerWait(ctx context.Context, _ string, _ container.WaitCondition) (<-chan container.WaitResponse, <-chan error) {
	statusCh := make(chan container.WaitResponse, 1)
	errCh := make(chan error, 1)

	if f.waitBlocksOnCtx {
		go func() {
			<-ctx.Done()
			errCh <- ctx.Err()
		}()
		return statusCh, errCh
	}

	if f.waitErr != nil {
		errCh <- f.waitErr
		return statusCh, errCh
	}

	statusCh <- f.waitResp
	return statusCh, errCh
}

func (f *fakeDockerAPI) ContainerStop(_ context.Context, containerID string, _ container.StopOptions) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopCalls = append(f.stopCalls, containerID)
	return f.stopErr
}

func (f *fakeDockerAPI) ContainerRemove(_ context.Context, containerID string, options container.RemoveOptions) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removeCalls = append(f.removeCalls, removeCall{containerID: containerID, options: options})
	if f.removeErrByID != nil {
		if err, ok := f.removeErrByID[containerID]; ok {
			return err
		}
	}
	return f.removeErr
}

func withFakeDockerAPI(t *testing.T, fake *fakeDockerAPI) {
	t.Helper()
	prev := newDockerAPIFn
	newDockerAPIFn = func() (dockerAPI, error) {
		return fake, nil
	}
	t.Cleanup(func() {
		newDockerAPIFn = prev
	})
}

func TestBuildDockerArgsReturnsContainerCommand(t *testing.T) {
	args, err := BuildDockerArgs(RunRequest{
		Image:              "claude:go",
		CWD:                "/tmp/work",
		SourceWorkspaceDir: "/workspace-source",
		GitHubToken:        "gh-token",
		ClaudeToken:        "claude-token",
		GitUserName:        "User",
		GitUserEmail:       "user@example.com",
		Prompt:             "build project",
		Model:              "sonnet",
	})
	if err != nil {
		t.Fatalf("build args: %v", err)
	}

	joined := strings.Join(args, " ")
	if joined != "--model sonnet -vv -v build project" {
		t.Fatalf("unexpected args: %q", joined)
	}
}

func TestBuildDockerArgsDefaultsModelToOpus(t *testing.T) {
	args, err := BuildDockerArgs(RunRequest{
		Image:              "claude:go",
		CWD:                "/tmp/work",
		SourceWorkspaceDir: "/workspace-source",
		Prompt:             "build project",
	})
	if err != nil {
		t.Fatalf("build args: %v", err)
	}

	if got := strings.Join(args, " "); !strings.Contains(got, "--model opus") {
		t.Fatalf("expected default model opus, got %q", got)
	}
}

func TestBuildDockerArgsIncludesDebugFlagForPrompt(t *testing.T) {
	args, err := BuildDockerArgs(RunRequest{
		Image:              "claude:go",
		CWD:                "/tmp/work",
		SourceWorkspaceDir: "/workspace-source",
		Prompt:             "build project",
		Model:              "sonnet",
		Debug:              true,
	})
	if err != nil {
		t.Fatalf("build args: %v", err)
	}

	joined := strings.Join(args, " ")
	if joined != "--model sonnet --debug -vv -v build project" {
		t.Fatalf("unexpected args: %q", joined)
	}
}

func TestBuildDockerArgsRejectsInvalidModel(t *testing.T) {
	_, err := BuildDockerArgs(RunRequest{
		Image:              "claude:go",
		CWD:                "/tmp/work",
		SourceWorkspaceDir: "/workspace-source",
		Prompt:             "build project",
		Model:              "bad",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildDockerArgsPipeline(t *testing.T) {
	cwd := t.TempDir()
	planPath := filepath.Join(cwd, ".agent-cli", "plans", "pipeline.yaml")

	args, err := BuildDockerArgs(RunRequest{
		Image:              "claude:go",
		CWD:                cwd,
		SourceWorkspaceDir: "/workspace-source",
		Pipeline:           planPath,
		Model:              "sonnet",
	})
	if err != nil {
		t.Fatalf("build args: %v", err)
	}

	joined := strings.Join(args, " ")
	if joined != "--model sonnet --pipeline /workspace/.agent-cli/plans/pipeline.yaml" {
		t.Fatalf("unexpected args: %q", joined)
	}
}

func TestBuildDockerArgsPipelineIncludesSortedTemplateVars(t *testing.T) {
	cwd := t.TempDir()
	planPath := filepath.Join(cwd, ".agent-cli", "plans", "pipeline.yaml")

	args, err := BuildDockerArgs(RunRequest{
		Image:              "claude:go",
		CWD:                cwd,
		SourceWorkspaceDir: "/workspace-source",
		Pipeline:           planPath,
		Model:              "sonnet",
		TemplateVars: map[string]string{
			"B_VAR": "two",
			"A_VAR": "one",
		},
	})
	if err != nil {
		t.Fatalf("build args: %v", err)
	}

	joined := strings.Join(args, " ")
	if joined != "--model sonnet --pipeline /workspace/.agent-cli/plans/pipeline.yaml --var A_VAR=one --var B_VAR=two" {
		t.Fatalf("unexpected args: %q", joined)
	}
}

func TestBuildDockerArgsPipelineIncludesDebugFlag(t *testing.T) {
	cwd := t.TempDir()
	planPath := filepath.Join(cwd, ".agent-cli", "plans", "pipeline.yaml")

	args, err := BuildDockerArgs(RunRequest{
		Image:              "claude:go",
		CWD:                cwd,
		SourceWorkspaceDir: "/workspace-source",
		Pipeline:           planPath,
		Model:              "sonnet",
		Debug:              true,
	})
	if err != nil {
		t.Fatalf("build args: %v", err)
	}

	joined := strings.Join(args, " ")
	if joined != "--model sonnet --debug --pipeline /workspace/.agent-cli/plans/pipeline.yaml" {
		t.Fatalf("unexpected args: %q", joined)
	}
}

func TestBuildDockerArgsPipelineRejectsOutsideCWD(t *testing.T) {
	cwd := t.TempDir()
	outside := filepath.Join(t.TempDir(), "pipeline.yaml")

	_, err := BuildDockerArgs(RunRequest{
		Image:              "claude:go",
		CWD:                cwd,
		SourceWorkspaceDir: "/workspace-source",
		Pipeline:           outside,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "pipeline path must be inside cwd") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildDockerArgsRejectsMixedPromptAndPipeline(t *testing.T) {
	_, err := BuildDockerArgs(RunRequest{
		Image:              "claude:go",
		CWD:                "/tmp/work",
		SourceWorkspaceDir: "/workspace-source",
		Prompt:             "build project",
		Pipeline:           "pipeline.yaml",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRunDockerStreamingSuccessUsesSDKAndStreams(t *testing.T) {
	cwd := t.TempDir()
	absCWD, err := filepath.Abs(cwd)
	if err != nil {
		t.Fatalf("abs cwd: %v", err)
	}

	fake := &fakeDockerAPI{
		listResp: []container.Summary{
			{ID: "stale-exited", State: "exited"},
			{ID: "still-running", State: "running"},
		},
		createResp: container.CreateResponse{ID: "new-run-container"},
		logsReader: muxedLogStream(
			[]string{"stdout-line-1", "stdout-line-2"},
			[]string{"stderr-line-1"},
		),
		waitResp: container.WaitResponse{StatusCode: 0},
	}
	withFakeDockerAPI(t, fake)

	var stdoutLines []string
	var stderrLines []string
	out, runErr := RunDockerStreaming(context.Background(), RunRequest{
		Image:              "claude:go",
		CWD:                cwd,
		SourceWorkspaceDir: "/workspace-source",
		GitHubToken:        "gh-token",
		ClaudeToken:        "claude-token",
		GitUserName:        "User",
		GitUserEmail:       "user@example.com",
		Prompt:             "build project",
		Model:              "sonnet",
	}, StreamHooks{
		OnStdoutLine: func(line string) { stdoutLines = append(stdoutLines, line) },
		OnStderrLine: func(line string) { stderrLines = append(stderrLines, line) },
	})
	if runErr != nil {
		t.Fatalf("run docker: %v", runErr)
	}

	if out.ExitCode != 0 {
		t.Fatalf("unexpected exit code: %d", out.ExitCode)
	}
	if got := strings.Join(out.Args, " "); got != "--model sonnet -vv -v build project" {
		t.Fatalf("unexpected args: %q", got)
	}
	if !strings.Contains(out.Stdout, "stdout-line-1\n") || !strings.Contains(out.Stdout, "stdout-line-2\n") {
		t.Fatalf("unexpected stdout: %q", out.Stdout)
	}
	if out.Stderr != "stderr-line-1\n" {
		t.Fatalf("unexpected stderr: %q", out.Stderr)
	}

	if len(stdoutLines) != 2 || stdoutLines[0] != "stdout-line-1" || stdoutLines[1] != "stdout-line-2" {
		t.Fatalf("unexpected stdout hooks: %#v", stdoutLines)
	}
	if len(stderrLines) != 1 || stderrLines[0] != "stderr-line-1" {
		t.Fatalf("unexpected stderr hooks: %#v", stderrLines)
	}

	if fake.createdConfig == nil || fake.createdHost == nil {
		t.Fatal("container config was not captured")
	}
	if fake.createdConfig.Image != "claude:go" {
		t.Fatalf("unexpected image: %s", fake.createdConfig.Image)
	}
	if fake.createdHost.NetworkMode != container.NetworkMode("host") {
		t.Fatalf("unexpected network mode: %s", fake.createdHost.NetworkMode)
	}
	if !fake.createdHost.AutoRemove {
		t.Fatal("expected AutoRemove=true")
	}
	if fake.createdHost.Privileged {
		t.Fatal("expected Privileged=false when ENABLE_DIND is disabled")
	}
	if len(fake.createdHost.Binds) != 1 {
		t.Fatalf("unexpected bind count: %d", len(fake.createdHost.Binds))
	}
	if bind := fake.createdHost.Binds[0]; bind != absCWD+":/workspace-source:ro" {
		t.Fatalf("unexpected bind: %q", bind)
	}
	if got := strings.Join(fake.createdConfig.Cmd, " "); got != "--model sonnet -vv -v build project" {
		t.Fatalf("unexpected container cmd: %q", got)
	}
	if fake.createdConfig.Labels[managedContainerLabelKey] != managedContainerLabelValue {
		t.Fatalf("missing managed label: %#v", fake.createdConfig.Labels)
	}
	if fake.createdConfig.Labels[managedContainerCWDHashLabelKey] == "" {
		t.Fatalf("missing cwd hash label: %#v", fake.createdConfig.Labels)
	}
	if !containsString(fake.createdConfig.Env, "GH_TOKEN=gh-token") ||
		!containsString(fake.createdConfig.Env, "CLAUDE_CODE_OAUTH_TOKEN=claude-token") ||
		!containsString(fake.createdConfig.Env, "SOURCE_WORKSPACE_DIR=/workspace-source") ||
		!containsString(fake.createdConfig.Env, "GIT_USER_NAME=User") ||
		!containsString(fake.createdConfig.Env, "GIT_USER_EMAIL=user@example.com") ||
		!containsString(fake.createdConfig.Env, "PIPELINE_TASK_IDLE_TIMEOUT_SEC=1800") ||
		!containsString(fake.createdConfig.Env, "FORCE_COLOR=1") {
		t.Fatalf("unexpected env: %#v", fake.createdConfig.Env)
	}
	if containsString(fake.createdConfig.Env, "ENABLE_DIND=1") {
		t.Fatalf("did not expect ENABLE_DIND in env when disabled: %#v", fake.createdConfig.Env)
	}

	labels := fake.listOptions.Filters.Get("label")
	if !containsString(labels, managedContainerLabelKey+"="+managedContainerLabelValue) {
		t.Fatalf("expected managed label filter, got %#v", labels)
	}

	if len(fake.removeCalls) != 1 {
		t.Fatalf("expected one stale removal, got %d", len(fake.removeCalls))
	}
	if fake.removeCalls[0].containerID != "stale-exited" {
		t.Fatalf("unexpected removed container: %s", fake.removeCalls[0].containerID)
	}
	if !fake.removeCalls[0].options.Force || !fake.removeCalls[0].options.RemoveVolumes {
		t.Fatalf("unexpected stale remove options: %#v", fake.removeCalls[0].options)
	}
}

func TestRunDockerStreamingInterruptedStopsAndRemoves(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	fake := &fakeDockerAPI{
		createResp:      container.CreateResponse{ID: "run-interrupt"},
		logsReader:      io.NopCloser(bytes.NewReader(nil)),
		waitBlocksOnCtx: true,
		onStart:         cancel,
	}
	withFakeDockerAPI(t, fake)

	out, runErr := RunDockerStreaming(ctx, RunRequest{
		Image:              "claude:go",
		CWD:                t.TempDir(),
		SourceWorkspaceDir: "/workspace-source",
		Prompt:             "build project",
	}, StreamHooks{})

	if !errors.Is(runErr, ErrInterrupted) {
		t.Fatalf("expected ErrInterrupted, got %v", runErr)
	}
	if out.ExitCode != interruptedExitCode {
		t.Fatalf("unexpected exit code: %d", out.ExitCode)
	}
	if len(fake.stopCalls) != 1 || fake.stopCalls[0] != "run-interrupt" {
		t.Fatalf("expected stop call for run-interrupt, got %#v", fake.stopCalls)
	}
	if len(fake.removeCalls) != 1 || fake.removeCalls[0].containerID != "run-interrupt" {
		t.Fatalf("expected cleanup remove for run-interrupt, got %#v", fake.removeCalls)
	}
	if !fake.removeCalls[0].options.Force || !fake.removeCalls[0].options.RemoveVolumes {
		t.Fatalf("unexpected cleanup remove options: %#v", fake.removeCalls[0].options)
	}
}

func TestRunDockerStreamingInterruptedIgnoresNotFoundRemoveRace(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	fake := &fakeDockerAPI{
		createResp:      container.CreateResponse{ID: "run-notfound"},
		logsReader:      io.NopCloser(bytes.NewReader(nil)),
		waitBlocksOnCtx: true,
		onStart:         cancel,
		removeErrByID: map[string]error{
			"run-notfound": errdefs.NotFound(errors.New("container already removed")),
		},
	}
	withFakeDockerAPI(t, fake)

	out, runErr := RunDockerStreaming(ctx, RunRequest{
		Image:              "claude:go",
		CWD:                t.TempDir(),
		SourceWorkspaceDir: "/workspace-source",
		Prompt:             "build project",
	}, StreamHooks{})

	if !errors.Is(runErr, ErrInterrupted) {
		t.Fatalf("expected ErrInterrupted, got %v", runErr)
	}
	if out.ExitCode != interruptedExitCode {
		t.Fatalf("unexpected exit code: %d", out.ExitCode)
	}
}

func TestRunDockerStreamingInterruptedDuringImagePullDrain(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pullReader := newBlockingReadCloser()
	fake := &fakeDockerAPI{
		imagePullReader: pullReader,
	}
	withFakeDockerAPI(t, fake)

	done := make(chan struct{})
	var out RunOutput
	var runErr error
	go func() {
		out, runErr = RunDockerStreaming(ctx, RunRequest{
			Image:              "claude:go",
			CWD:                t.TempDir(),
			SourceWorkspaceDir: "/workspace-source",
			Prompt:             "build project",
		}, StreamHooks{})
		close(done)
	}()

	select {
	case <-pullReader.Started():
	case <-time.After(2 * time.Second):
		t.Fatal("image pull drain did not start")
	}

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("run did not stop after cancellation during image pull drain")
	}

	if !errors.Is(runErr, ErrInterrupted) {
		t.Fatalf("expected ErrInterrupted, got %v", runErr)
	}
	if out.ExitCode != interruptedExitCode {
		t.Fatalf("unexpected exit code: %d", out.ExitCode)
	}
	if !pullReader.Closed() {
		t.Fatal("expected image pull reader to be closed on cancellation")
	}
}

func TestRunDockerStreamingEnableDinDSetsPrivilegedAndEnv(t *testing.T) {
	fake := &fakeDockerAPI{
		createResp: container.CreateResponse{ID: "dind-run"},
		logsReader: muxedLogStream(
			[]string{"ok"},
			nil,
		),
		waitResp: container.WaitResponse{StatusCode: 0},
	}
	withFakeDockerAPI(t, fake)

	out, runErr := RunDockerStreaming(context.Background(), RunRequest{
		Image:              "claude:go",
		CWD:                t.TempDir(),
		SourceWorkspaceDir: "/workspace-source",
		Prompt:             "build project",
		EnableDinD:         true,
	}, StreamHooks{})
	if runErr != nil {
		t.Fatalf("run docker: %v", runErr)
	}
	if out.ExitCode != 0 {
		t.Fatalf("unexpected exit code: %d", out.ExitCode)
	}
	if fake.createdHost == nil {
		t.Fatal("container host config was not captured")
	}
	if !fake.createdHost.Privileged {
		t.Fatal("expected Privileged=true when ENABLE_DIND=1")
	}
	if fake.createdHost.NetworkMode != container.NetworkMode("bridge") {
		t.Fatalf("expected bridge network mode for DinD, got %s", fake.createdHost.NetworkMode)
	}
	if fake.createdConfig == nil {
		t.Fatal("container config was not captured")
	}
	if !containsString(fake.createdConfig.Env, "ENABLE_DIND=1") {
		t.Fatalf("expected ENABLE_DIND env, got %#v", fake.createdConfig.Env)
	}
}

func TestRunDockerStreamingIdleTimeout(t *testing.T) {
	fake := &fakeDockerAPI{
		createResp:      container.CreateResponse{ID: "run-idle-timeout"},
		logsReader:      io.NopCloser(bytes.NewReader(nil)),
		waitBlocksOnCtx: true,
	}
	withFakeDockerAPI(t, fake)

	out, runErr := RunDockerStreaming(context.Background(), RunRequest{
		Image:              "claude:go",
		CWD:                t.TempDir(),
		SourceWorkspaceDir: "/workspace-source",
		Prompt:             "build project",
		RunIdleTimeoutSec:  1,
	}, StreamHooks{})
	if !errors.Is(runErr, ErrIdleTimeout) {
		t.Fatalf("expected ErrIdleTimeout, got %v", runErr)
	}
	if out.ExitCode != -1 {
		t.Fatalf("unexpected exit code: %d", out.ExitCode)
	}
	if len(fake.stopCalls) != 1 || fake.stopCalls[0] != "run-idle-timeout" {
		t.Fatalf("expected stop call for run-idle-timeout, got %#v", fake.stopCalls)
	}
	if len(fake.removeCalls) != 1 || fake.removeCalls[0].containerID != "run-idle-timeout" {
		t.Fatalf("expected cleanup remove for run-idle-timeout, got %#v", fake.removeCalls)
	}
}

func TestRunDockerStreamingIdleTimeoutResetsOnLogActivity(t *testing.T) {
	fake := &fakeDockerAPI{
		createResp: container.CreateResponse{ID: "run-idle-reset"},
		logsReader: delayedMuxedLogStream([]string{"tick1", "tick2", "tick3", "tick4"}, 400*time.Millisecond),
		waitResp:   container.WaitResponse{StatusCode: 0},
	}
	withFakeDockerAPI(t, fake)

	out, runErr := RunDockerStreaming(context.Background(), RunRequest{
		Image:              "claude:go",
		CWD:                t.TempDir(),
		SourceWorkspaceDir: "/workspace-source",
		Prompt:             "build project",
		RunIdleTimeoutSec:  1,
	}, StreamHooks{})
	if runErr != nil {
		t.Fatalf("run docker: %v", runErr)
	}
	if out.ExitCode != 0 {
		t.Fatalf("unexpected exit code: %d", out.ExitCode)
	}
	if !strings.Contains(out.Stdout, "tick4\n") {
		t.Fatalf("unexpected stdout: %q", out.Stdout)
	}
}

func TestRunDockerStreamingPipelineTaskIdleTimeoutEnvOverride(t *testing.T) {
	fake := &fakeDockerAPI{
		createResp: container.CreateResponse{ID: "run-timeout-env"},
		logsReader: muxedLogStream(
			[]string{"ok"},
			nil,
		),
		waitResp: container.WaitResponse{StatusCode: 0},
	}
	withFakeDockerAPI(t, fake)

	_, runErr := RunDockerStreaming(context.Background(), RunRequest{
		Image:                      "claude:go",
		CWD:                        t.TempDir(),
		SourceWorkspaceDir:         "/workspace-source",
		Prompt:                     "build project",
		PipelineTaskIdleTimeoutSec: 99,
	}, StreamHooks{})
	if runErr != nil {
		t.Fatalf("run docker: %v", runErr)
	}
	if fake.createdConfig == nil {
		t.Fatal("container config was not captured")
	}
	if !containsString(fake.createdConfig.Env, "PIPELINE_TASK_IDLE_TIMEOUT_SEC=99") {
		t.Fatalf("expected PIPELINE_TASK_IDLE_TIMEOUT_SEC env, got %#v", fake.createdConfig.Env)
	}
}

func muxedLogStream(stdoutLines, stderrLines []string) io.ReadCloser {
	buf := &bytes.Buffer{}
	stdout := stdcopy.NewStdWriter(buf, stdcopy.Stdout)
	stderr := stdcopy.NewStdWriter(buf, stdcopy.Stderr)

	for _, line := range stdoutLines {
		_, _ = io.WriteString(stdout, line+"\n")
	}
	for _, line := range stderrLines {
		_, _ = io.WriteString(stderr, line+"\n")
	}

	return io.NopCloser(bytes.NewReader(buf.Bytes()))
}

func delayedMuxedLogStream(stdoutLines []string, delay time.Duration) io.ReadCloser {
	reader, writer := io.Pipe()

	go func() {
		defer writer.Close()
		stdout := stdcopy.NewStdWriter(writer, stdcopy.Stdout)
		for index, line := range stdoutLines {
			if index > 0 {
				time.Sleep(delay)
			}
			if _, err := io.WriteString(stdout, line+"\n"); err != nil {
				return
			}
		}
	}()

	return reader
}

type blockingReadCloser struct {
	closedCh  chan struct{}
	startedCh chan struct{}
	once      sync.Once
}

func newBlockingReadCloser() *blockingReadCloser {
	return &blockingReadCloser{
		closedCh:  make(chan struct{}),
		startedCh: make(chan struct{}),
	}
}

func (r *blockingReadCloser) Read(_ []byte) (int, error) {
	r.once.Do(func() {
		close(r.startedCh)
	})
	<-r.closedCh
	return 0, io.EOF
}

func (r *blockingReadCloser) Close() error {
	select {
	case <-r.closedCh:
	default:
		close(r.closedCh)
	}
	return nil
}

func (r *blockingReadCloser) Closed() bool {
	select {
	case <-r.closedCh:
		return true
	default:
		return false
	}
}

func (r *blockingReadCloser) Started() <-chan struct{} {
	return r.startedCh
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
