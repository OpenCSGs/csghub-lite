# Claude OSS Mirror

- When working on Claude Code native installer distribution, use
  `scripts/sync-claude-code-oss.sh` as the canonical sync path.
- Load OSS credentials only from `local/secrets.env` and keep them local-only.
  Use the `STARHUB_OSS_*` and `STARHUB_CLAUDE_*` variables already defined
  there.
- Keep the public mirror layout versioned:
  - `claude-code-releases/latest`
  - `claude-code-releases/<version>/manifest.json`
  - `claude-code-releases/<version>/<platform>/<binary>`
- Mirror every platform published in the upstream manifest, including macOS,
  Linux, Linux musl, and Windows variants.
- Use proxy only for upstream external downloads such as Anthropic/GCS. Upload
  to Alibaba OSS directly without proxy; `scripts/sync-claude-code-oss.sh`
  handles this split.
- Do not update `latest` until all artifacts for that version, plus the
  rewritten `manifest.json`, have uploaded successfully.
- Keep the mirrored `manifest.json` compatible with the Claude install scripts
  in `internal/apps/scripts/claude-code-install.sh` and
  `internal/apps/scripts/claude-code-install.ps1`.
- If multiple Python versions are installed, run the sync script with a Python
  that can import `oss2`.
