package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAPIUsageMigratesLegacyRequestCounts(t *testing.T) {
	dir := t.TempDir()
	usedAt := time.Date(2026, 5, 15, 8, 30, 0, 0, time.UTC)
	legacy := APIUsageState{
		Records: []APIUsageRecord{
			{
				APIKeyID:     "key-1",
				APIKeyName:   "client",
				Model:        "test/model",
				Source:       "local",
				SourceType:   "local",
				Requests:     42,
				InputTokens:  100,
				OutputTokens: 23,
				TotalTokens:  123,
				LastUsedAt:   usedAt,
			},
		},
	}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy usage: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, APIUsageFile), data, 0o600); err != nil {
		t.Fatalf("write legacy usage: %v", err)
	}

	store := NewAPIUsageStore(dir)
	state, err := store.List(APIUsageListOptions{})
	if err != nil {
		t.Fatalf("list usage: %v", err)
	}
	if len(state.Records) != 1 {
		t.Fatalf("records = %#v, want one", state.Records)
	}
	record := state.Records[0]
	if record.Requests != 42 || record.InputTokens != 100 || record.OutputTokens != 23 || record.TotalTokens != 123 {
		t.Fatalf("migrated record = %#v, want legacy totals preserved", record)
	}
}

func TestAPIUsageCompactsEventsByDayAndSource(t *testing.T) {
	dir := t.TempDir()
	store := NewAPIUsageStore(dir)
	first := time.Date(2026, 5, 15, 9, 0, 0, 0, time.UTC)
	events := []APIUsageEvent{
		{
			APIKeyID:     "key-1",
			APIKeyName:   "client",
			Model:        "test/model",
			Source:       "provider:a",
			SourceType:   "provider",
			SourceName:   "Provider A",
			InputTokens:  1,
			OutputTokens: 2,
			CreatedAt:    first,
		},
		{
			APIKeyID:     "key-1",
			APIKeyName:   "client",
			Model:        "test/model",
			Source:       "provider:a",
			SourceType:   "provider",
			SourceName:   "Provider A",
			InputTokens:  3,
			OutputTokens: 4,
			CreatedAt:    first.Add(2 * time.Hour),
		},
		{
			APIKeyID:     "key-1",
			APIKeyName:   "client",
			Model:        "test/model",
			Source:       "provider:a",
			SourceType:   "provider",
			SourceName:   "Provider A",
			InputTokens:  5,
			OutputTokens: 6,
			CreatedAt:    first.AddDate(0, 0, 1),
		},
	}
	for _, event := range events {
		if err := store.Add(event); err != nil {
			t.Fatalf("add usage: %v", err)
		}
	}

	data, err := os.ReadFile(filepath.Join(dir, APIUsageFile))
	if err != nil {
		t.Fatalf("read usage file: %v", err)
	}
	var persisted APIUsageState
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("decode persisted usage: %v", err)
	}
	if len(persisted.Events) != 2 {
		t.Fatalf("events = %#v, want two daily buckets", persisted.Events)
	}
	if persisted.Events[0].Requests != 2 || persisted.Events[0].InputTokens != 4 || persisted.Events[0].OutputTokens != 6 {
		t.Fatalf("first bucket = %#v, want same-day usage compacted", persisted.Events[0])
	}

	state, err := store.List(APIUsageListOptions{})
	if err != nil {
		t.Fatalf("list usage: %v", err)
	}
	if len(state.Records) != 1 || state.Records[0].Requests != 3 || state.Records[0].TotalTokens != 21 {
		t.Fatalf("records = %#v, want aggregate across buckets", state.Records)
	}
}
