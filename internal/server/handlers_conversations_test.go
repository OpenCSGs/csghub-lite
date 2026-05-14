package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/opencsgs/csghub-lite/pkg/api"
)

func TestConversationUpdatePreservesMessageMetaSources(t *testing.T) {
	s := newTestServer(t)
	conv := api.Conversation{
		Title: "web search",
		Messages: []api.Message{{
			Role:    "assistant",
			Content: "answer [1]",
			Meta: &api.MessageMeta{
				Sources: []api.WebSearchResult{{
					Title:   "Example Source",
					URL:     "https://example.com/source",
					Snippet: "snippet",
					Engine:  "bing",
				}},
			},
		}},
	}
	body, err := json.Marshal(conv)
	if err != nil {
		t.Fatalf("marshal create: %v", err)
	}
	createReq := httptest.NewRequest(http.MethodPost, "/api/conversations", bytes.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	s.handleConversationCreate(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body=%s", createW.Code, createW.Body.String())
	}
	createdBody := createW.Body.String()
	var created api.Conversation
	if err := json.Unmarshal([]byte(createdBody), &created); err != nil {
		t.Fatalf("decode create: %v", err)
	}

	updateReq := httptest.NewRequest(http.MethodPut, "/api/conversations/"+created.ID, strings.NewReader(createdBody))
	updateReq.SetPathValue("id", created.ID)
	updateReq.Header.Set("Content-Type", "application/json")
	updateW := httptest.NewRecorder()
	s.handleConversationUpdate(updateW, updateReq)
	if updateW.Code != http.StatusOK {
		t.Fatalf("update status = %d, body=%s", updateW.Code, updateW.Body.String())
	}

	got, err := s.conversations.Get(created.ID)
	if err != nil {
		t.Fatalf("get conversation: %v", err)
	}
	if len(got.Messages) != 1 || got.Messages[0].Meta == nil || len(got.Messages[0].Meta.Sources) != 1 {
		t.Fatalf("sources were not preserved: %#v", got.Messages)
	}
	if got.Messages[0].Meta.Sources[0].URL != "https://example.com/source" {
		t.Fatalf("source URL = %q", got.Messages[0].Meta.Sources[0].URL)
	}
}
