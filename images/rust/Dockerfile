# syntax=docker/dockerfile:1.7
ARG BASE_IMAGE=claude:latest
FROM ${BASE_IMAGE}

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

USER root

# Rust tooling requires ssl and pkg-config in many crates.
RUN apt-get update && apt-get install -y --no-install-recommends \
    pkg-config \
    libssl-dev \
    && rm -rf /var/lib/apt/lists/*

ENV RUSTUP_HOME=/home/claude/.rustup
ENV CARGO_HOME=/home/claude/.cargo
ENV PATH="/home/claude/.cargo/bin:/home/claude/.local/bin:${PATH}"

RUN mkdir -p /home/claude/.cargo /home/claude/.rustup && \
    chown -R claude:claude /home/claude/.cargo /home/claude/.rustup

COPY --chown=claude:claude rust/claude-config/ /home/claude/.claude/

USER claude

# Install latest stable Rust and core components.
RUN --mount=type=cache,target=/home/claude/.cargo/registry,uid=1000,gid=1000 \
    --mount=type=cache,target=/home/claude/.cargo/git,uid=1000,gid=1000 \
    curl -fsSL https://sh.rustup.rs | sh -s -- -y --profile default --default-toolchain stable && \
    rustup component add rustfmt clippy rust-analyzer

# Keep installs split for faster retries when one tool fails.
RUN --mount=type=cache,target=/home/claude/.cargo/registry,uid=1000,gid=1000 \
    --mount=type=cache,target=/home/claude/.cargo/git,uid=1000,gid=1000 \
    --mount=type=cache,target=/home/claude/.cargo/target,uid=1000,gid=1000 \
    cargo install --locked cargo-nextest cargo-edit
