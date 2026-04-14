#!/bin/bash
set -e

# Git config — GitLab (default)
git config --global user.name "james"
git config --global user.email "james@opencsg.com"

# Git config — GitHub override
mkdir -p ~/.config/git
cat > ~/.config/git/github-config <<'EOF'
[user]
    name = ganisback
EOF
git config --global "includeIf.hasconfig:remote.*.url:*github.com*.path" "$HOME/.config/git/github-config"

# Install web dependencies
cd web && npm install
