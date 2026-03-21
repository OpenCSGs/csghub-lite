package inference

import (
	"strings"
	"testing"
)

func TestHandleStreamReasoningContent(t *testing.T) {
	e := &llamaEngine{}
	var tokens strings.Builder
	sse := "" +
		"data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"Hi\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\" there\"}}]}\n\n" +
		"data: [DONE]\n\n"

	full, err := e.handleStream(strings.NewReader(sse), func(s string) {
		tokens.WriteString(s)
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "Hi there"
	if full != want {
		t.Errorf("full = %q, want %q", full, want)
	}
	if tokens.String() != want {
		t.Errorf("streamed tokens = %q, want %q", tokens.String(), want)
	}
}

func TestHandleNonStreamReasoningOnly(t *testing.T) {
	e := &llamaEngine{}
	body := `{"choices":[{"message":{"reasoning_content":"Answer","content":""}}]}`
	got, err := e.handleNonStream(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if got != "Answer" {
		t.Errorf("got %q, want Answer", got)
	}
}

func TestHandleNonStreamBothReasoningAndContent(t *testing.T) {
	e := &llamaEngine{}
	body := `{"choices":[{"message":{"reasoning_content":"think","content":"ok"}}]}`
	got, err := e.handleNonStream(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if got != "thinkok" {
		t.Errorf("got %q, want thinkok", got)
	}
}
