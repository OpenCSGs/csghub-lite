package api

import (
	"testing"
	"time"
)

func TestParseKeepAlive(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantSet   bool
		wantValue time.Duration
		wantErr   bool
	}{
		{name: "unset", input: "", wantSet: false, wantValue: 0},
		{name: "forever", input: "-1", wantSet: true, wantValue: KeepAliveForever},
		{name: "duration", input: "5m", wantSet: true, wantValue: 5 * time.Minute},
		{name: "reject invalid duration", input: "later", wantSet: true, wantErr: true},
		{name: "reject other negative duration", input: "-5m", wantSet: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotSet, err := ParseKeepAlive(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseKeepAlive(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if gotSet != tt.wantSet {
				t.Fatalf("ParseKeepAlive(%q) set = %v, want %v", tt.input, gotSet, tt.wantSet)
			}
			if err == nil && got != tt.wantValue {
				t.Fatalf("ParseKeepAlive(%q) = %s, want %s", tt.input, got, tt.wantValue)
			}
		})
	}
}
