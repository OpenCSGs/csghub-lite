# Storage And Temporary Files

- Runtime files created by csghub-lite must live under the configured storage
  root, which defaults to `~/.csghub-lite`.
- Temporary files, upload staging files, generated chunks, caches, extracted
  assets, worker scripts, and runtime shims must use the lite storage tree,
  preferably `Config.TempDir()` or `config.TempDirForStorage(...)` for temp data.
- Do not use `os.TempDir()`, Python `tempfile` defaults, `/tmp`, the current
  working directory, or user folders such as `Downloads` for product runtime
  files unless the user explicitly selected that path as a csghub-lite storage
  location.
- For subprocesses that may create temporary files, pass `TMPDIR`, `TMP`, and
  `TEMP` pointing at the lite temp directory.
- Tests may use test-scoped temporary directories, but production code should
  keep all runtime output inside the lite storage root.
