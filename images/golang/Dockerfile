# syntax=docker/dockerfile:1.7
ARG BASE_IMAGE=claude:latest
FROM ${BASE_IMAGE}

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

USER root

# Install latest stable Go by default. You can pin it with GO_DL_VERSION (e.g. 1.25.2).
ARG GO_DL_VERSION=""
RUN ARCH="$(dpkg --print-architecture)" && \
    case "${ARCH}" in \
      amd64) GOARCH="amd64" ;; \
      arm64) GOARCH="arm64" ;; \
      *) echo "Unsupported architecture: ${ARCH}" && exit 1 ;; \
    esac && \
    if [ -z "${GO_DL_VERSION}" ]; then \
      GO_VERSION="$(curl -fsSL https://go.dev/VERSION?m=text | head -n1)"; \
    else \
      GO_VERSION="go${GO_DL_VERSION}"; \
    fi && \
    curl -fsSL "https://go.dev/dl/${GO_VERSION}.linux-${GOARCH}.tar.gz" -o /tmp/go.tar.gz && \
    rm -rf /usr/local/go && \
    tar -C /usr/local -xzf /tmp/go.tar.gz && \
    rm -f /tmp/go.tar.gz

ENV GOROOT=/usr/local/go
ENV GOPATH=/home/claude/go
ENV PATH="/usr/local/go/bin:/home/claude/go/bin:/home/claude/.local/bin:${PATH}"

RUN mkdir -p /home/claude/go/bin /home/claude/go/pkg && \
    chown -R claude:claude /home/claude/go

COPY --chown=claude:claude golang/claude-config/ /home/claude/.claude/

USER claude

# Install popular Go development, testing, and quality tools.
RUN --mount=type=cache,target=/home/claude/.cache/go-build,uid=1000,gid=1000 \
    --mount=type=cache,target=/home/claude/go/pkg,uid=1000,gid=1000 \
    go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.9.0 && \
    go install github.com/onsi/ginkgo/v2/ginkgo@latest && \
    go install golang.org/x/tools/gopls@latest && \
    go install mvdan.cc/gofumpt@latest
