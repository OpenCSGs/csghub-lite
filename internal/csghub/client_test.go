package csghub

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		token   string
		wantURL string
	}{
		{
			name:    "default URL",
			baseURL: "",
			token:   "tok",
			wantURL: "https://hub.opencsg.com",
		},
		{
			name:    "custom URL",
			baseURL: "https://custom.example.com",
			token:   "tok2",
			wantURL: "https://custom.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewClient(tt.baseURL, tt.token)
			if c.BaseURL() != tt.wantURL {
				t.Errorf("BaseURL() = %q, want %q", c.BaseURL(), tt.wantURL)
			}
			if c.Token() != tt.token {
				t.Errorf("Token() = %q, want %q", c.Token(), tt.token)
			}
		})
	}
}

func TestGetJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Authorization = %q, want %q", r.Header.Get("Authorization"), "Bearer test-token")
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("Accept = %q, want %q", r.Header.Get("Accept"), "application/json")
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"msg": "OK"})
	}))
	defer server.Close()

	c := NewClient(server.URL, "test-token")
	var out map[string]string
	if err := c.getJSON(context.Background(), "/test", &out); err != nil {
		t.Fatalf("getJSON error: %v", err)
	}
	if out["msg"] != "OK" {
		t.Errorf("msg = %q, want %q", out["msg"], "OK")
	}
}

func TestGetJSON_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	c := NewClient(server.URL, "")
	var out map[string]string
	err := c.getJSON(context.Background(), "/missing", &out)
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestGetJSON_NoAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("Authorization should be empty when no token is set")
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"msg": "OK"})
	}))
	defer server.Close()

	c := NewClient(server.URL, "")
	var out map[string]string
	if err := c.getJSON(context.Background(), "/test", &out); err != nil {
		t.Fatalf("getJSON error: %v", err)
	}
}

func TestDownloadHTTPClient(t *testing.T) {
	c := NewClient("", "")
	client := c.DownloadHTTPClient()
	if client.Timeout != 0 {
		t.Errorf("Timeout = %v, want 0 (no timeout)", client.Timeout)
	}
}
