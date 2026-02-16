# Runners

Base runner image is defined in `images/Dockerfile` and used by language-specific images in `images/golang` and `images/rust`.

## Optional DinD mode

Set `ENABLE_DIND=1` to start an internal Docker daemon (`dockerd`) inside the runner container.

### Requirements

- The runner container must be started with `--privileged`.
- DinD is rootful and intended for trusted workloads.

### Behavior

- `DOCKER_HOST` defaults to `unix:///var/run/docker.sock` inside the runner.
- `DIND_STORAGE_DRIVER` defaults to `overlay2`.
- If `overlay2` fails to start, entrypoint retries with `vfs`.
- `DIND_STARTUP_TIMEOUT_SEC` defaults to `45` seconds.

### Example

```bash
docker run --rm --privileged \
  -e ENABLE_DIND=1 \
  -e GH_TOKEN=... \
  -e CLAUDE_CODE_OAUTH_TOKEN=... \
  -e SOURCE_WORKSPACE_DIR=/workspace-source \
  -v "$(pwd)":/workspace-source:ro \
  claude:go --model opus -v "run integration tests"
```

### Running docker compose in agent tasks

Inside the task prompt/scripts you can run commands like:

```bash
docker compose up -d
# run tests
docker compose down -v
```

Always tear down nested resources (`docker compose down -v`) at the end of tests to avoid leaked volumes/networks during long-lived sessions.

## Security notes

- `--privileged` grants elevated host capabilities.
- Use isolated CI runners/VMs for untrusted repositories.
