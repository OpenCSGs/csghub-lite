# API Swagger Sync

- Any change to local HTTP APIs must update Swagger/OpenAPI in the same task.
- This applies to new, removed, or renamed `/api/*` and `/v1/*` routes, and to
  request/response shape changes on existing endpoints.
- Update `openapi/local-api.json` whenever the route surface or schema changes.
- Keep the embedded docs copy in sync when needed for local preview or committed
  static assets.
- Run `go test ./internal/server` after API doc changes so route/doc drift is
  caught by `openapi_sync_test.go`.

```go
// If you add a route in internal/server/routes.go:
mux.HandleFunc("GET /api/models/{namespace}/{name}/manifest", s.handleModelManifest)

// You must also document it in:
// openapi/local-api.json
```
