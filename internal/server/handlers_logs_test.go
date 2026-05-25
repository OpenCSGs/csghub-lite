package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsQuietRequestLogIncludesAppsPolling(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/apps", nil)
	if !isQuietRequestLog(req) {
		t.Fatal("/api/apps should be quiet for fast successful polling requests")
	}
}

func TestIsQuietRequestLogKeepsAppActionsVisible(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/apps/start", nil)
	if isQuietRequestLog(req) {
		t.Fatal("app action requests should remain visible in request logs")
	}
}
