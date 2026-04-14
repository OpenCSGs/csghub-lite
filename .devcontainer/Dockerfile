FROM mcr.microsoft.com/devcontainers/go:1.24

RUN apt-get update && apt-get install -y --no-install-recommends \
    cmake g++ make curl git \
    && rm -rf /var/lib/apt/lists/*

# Install Claude Code CLI
RUN npm install -g @anthropic-ai/claude-code
