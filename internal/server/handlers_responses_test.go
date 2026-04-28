package server

import "testing"

func TestNormalizeResponsesVisibleTextStripsThinkBlocks(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "leading think block",
			in:   "<think>The user greeted us.</think>\n\nHi! How can I help?",
			want: "Hi! How can I help?",
		},
		{
			name: "case insensitive tags",
			in:   "<THINK>hidden</THINK>\nOK",
			want: "OK",
		},
		{
			name: "middle think block",
			in:   "hello <think>hidden</think> world",
			want: "hello  world",
		},
		{
			name: "unfinished think block",
			in:   "visible <think>hidden",
			want: "visible ",
		},
		{
			name: "plain text",
			in:   "show <thinking> as normal text",
			want: "show <thinking> as normal text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeResponsesVisibleText(tt.in); got != tt.want {
				t.Fatalf("normalizeResponsesVisibleText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResponsesThinkTagFilterHandlesSplitTags(t *testing.T) {
	filter := newResponsesThinkTagFilter()
	chunks := []string{
		"<thi",
		"nk>hidden reasoning</th",
		"ink>\n\nHi",
		" there",
	}
	var got string
	for _, chunk := range chunks {
		got += filter.Push(chunk)
	}
	got += filter.Flush()

	if want := "Hi there"; got != want {
		t.Fatalf("streamed visible text = %q, want %q", got, want)
	}
}

func TestResponsesThinkTagFilterPreservesNonTagPrefixAcrossChunks(t *testing.T) {
	filter := newResponsesThinkTagFilter()
	got := filter.Push("use <thi")
	got += filter.Push("s normally")
	got += filter.Flush()

	if want := "use <this normally"; got != want {
		t.Fatalf("streamed visible text = %q, want %q", got, want)
	}
}
