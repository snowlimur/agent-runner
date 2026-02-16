# Use the official Node.js image based on Debian Bookworm
FROM node:20-bookworm-slim

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

# Install system dependencies
# ripgrep, git, curl, jq are required for Claude Code
# gh is required for GitHub workflow
# sudo is for running commands as root when needed
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    curl \
    wget \
    jq \
    ripgrep \
    sudo \
    build-essential \
    python3 \
    python3-pip \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Install Docker Engine + Docker Compose plugin (for optional DinD mode)
RUN install -m 0755 -d /etc/apt/keyrings && \
    curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc && \
    chmod a+r /etc/apt/keyrings/docker.asc && \
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/debian $(. /etc/os-release && echo \"${VERSION_CODENAME}\") stable" > /etc/apt/sources.list.d/docker.list && \
    apt-get update && apt-get install -y --no-install-recommends \
    docker-ce \
    docker-ce-cli \
    containerd.io \
    docker-buildx-plugin \
    docker-compose-plugin \
    && rm -rf /var/lib/apt/lists/*

# Install Taskfile
RUN curl -1sLf 'https://dl.cloudsmith.io/public/task/task/setup.deb.sh' | sudo -E bash
RUN apt-get update && apt-get install -y task \
    && rm -rf /var/lib/apt/lists/*

# Install GitHub CLI (following official Debian instructions)
RUN mkdir -p -m 755 /etc/apt/keyrings && \
    curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg -o /etc/apt/keyrings/githubcli-archive-keyring.gpg && \
    chmod go+r /etc/apt/keyrings/githubcli-archive-keyring.gpg && \
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" > /etc/apt/sources.list.d/github-cli.list && \
    apt-get update && apt-get install -y gh && \
    rm -rf /var/lib/apt/lists/*

# Remove the `node` user (UID 1000) from the base image
# to avoid UID conflict with the `claude` user
RUN userdel -r node 2>/dev/null || true

# Create unprivileged `claude` user
# UID 1000 usually matches the first Linux user on host systems
ARG USER_ID=1000
ARG GROUP_ID=1000
RUN groupadd -f -g $GROUP_ID claude && \
    useradd -m -u $USER_ID -g $GROUP_ID -s /bin/bash claude && \
    echo "claude ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers
RUN usermod -aG docker claude

# Configure directories
RUN mkdir -p /home/claude/.claude /workspace /var/lib/docker /var/run && \
    chown -R claude:claude /home/claude/.claude /workspace

# Install Claude Code as `claude`
# so the binary ends up in /home/claude/.local/bin/
RUN sudo -u claude bash -lc 'curl -fsSL https://claude.ai/install.sh -o /tmp/claude-install.sh && \
    bash /tmp/claude-install.sh && \
    rm -f /tmp/claude-install.sh'

RUN printf '{\n  "hasCompletedOnboarding": true,\n  "theme": "dark"\n}\n' > /home/claude/.claude.json && \
    chown claude:claude /home/claude/.claude.json

# Install entrypoint runtime dependencies.
WORKDIR /opt/entrypoint
COPY entrypoint/package.json entrypoint/package-lock.json ./
RUN npm ci --omit=dev

# Copy entrypoint script and modular runtime files
COPY entrypoint/entrypoint.mjs ./entrypoint.mjs
COPY entrypoint/lib ./lib
RUN chmod +x ./entrypoint.mjs

USER claude
WORKDIR /workspace

# Add ~/.local/bin to PATH (install.sh puts the `claude` binary there)
ENV PATH="/home/claude/.local/bin:${PATH}"
ENV DOCKER_HOST="unix:///var/run/docker.sock"
ENV DOCKER_TLS_CERTDIR=""
ENV ENABLE_DIND="0"
ENV DIND_STORAGE_DRIVER="overlay2"
ENV DIND_STARTUP_TIMEOUT_SEC="45"

# Environment variables for headless operation
ENV CI=true
ENV TERM=xterm-256color

ENTRYPOINT ["/opt/entrypoint/entrypoint.mjs"]
