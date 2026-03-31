# Local API OpenAPI Spec

`local-api.json` is the source of truth for the local `csghub-lite` `/api/*` and `/v1/*` endpoints.

When you add, remove, or rename an `/api/*` or `/v1/*` route:

1. Update `internal/server/routes.go`.
2. Update `openapi/local-api.json`.
3. Run `go test ./internal/server`.

`internal/server/openapi_sync_test.go` will fail if the route list and OpenAPI paths drift out of sync.
