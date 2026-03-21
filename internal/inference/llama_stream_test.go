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

func TestHandleStreamSameChunkDuplicateContentAndReasoning(t *testing.T) {
	e := &llamaEngine{}
	var n int
	sse := "data: {\"choices\":[{\"delta\":{\"content\":\"你好\",\"reasoning_content\":\"你好\"}}]}\n\n" +
		"data: [DONE]\n\n"
	_, err := e.handleStream(strings.NewReader(sse), func(s string) {
		n++
		if s != "你好" {
			t.Errorf("unexpected token %q", s)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("onToken called %d times, want 1 (no duplicate fields)", n)
	}
}

func TestHandleNonStreamDuplicateReasoningAndContent(t *testing.T) {
	e := &llamaEngine{}
	body := `{"choices":[{"message":{"reasoning_content":"你好","content":"你好"}}]}`
	got, err := e.handleNonStream(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if got != "你好" {
		t.Errorf("got %q, want single 你好", got)
	}
}
