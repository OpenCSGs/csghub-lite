# Local Preview Port

- Do not start a separate frontend dev server for this workspace.
- For local preview, redeploy, and verification, use only
  `http://localhost:11435`.
- Before any local preview or redeploy, stop the old preview process first.
- Never keep multiple preview servers alive at the same time. If an older
  `csghub-lite` preview is still running, kill it and then start one fresh
  instance.
- If frontend files change, rebuild `web`, sync `web/dist` into
  `internal/server/static`, and then restart the single backend preview on
  `:11435`.
- Do not start or keep `5173` or any alternate preview port running unless the
  user explicitly asks for it.

```sh
pid11435=$(lsof -tiTCP:11435 -sTCP:LISTEN 2>/dev/null || true)
pid11436=$(lsof -tiTCP:11436 -sTCP:LISTEN 2>/dev/null || true)
pids=$(printf '%s\n%s\n' "$pid11435" "$pid11436" | awk 'NF' | sort -u)
if [ -n "$pids" ]; then kill $pids; fi
cd web && npm run build
cd ..
cp -r web/dist/. internal/server/static/
go run ./cmd/csghub-lite serve --listen :11435
```
