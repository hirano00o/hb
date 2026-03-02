package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/hirano00o/hb/hatena"
)

func TestParseFilterDate(t *testing.T) {
	want := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"2026-01-15", false},
		{"2026/01/15", false},
		{"20260115", false},
		{"2026.01.15", true},
		{"invalid", true},
		{"", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseFilterDate(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for %q, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.input, err)
			}
			if !got.Equal(want) {
				t.Errorf("got %v, want %v", got, want)
			}
		})
	}
}

func TestFilterEntriesByDate(t *testing.T) {
	mkEntry := func(year int, month time.Month, day int) *hatena.Entry {
		return &hatena.Entry{
			Date:    time.Date(year, month, day, 12, 0, 0, 0, time.FixedZone("JST", 9*3600)),
			EditURL: "https://example.com/" + time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Format("20060102"),
		}
	}

	entries := []*hatena.Entry{
		mkEntry(2025, 12, 31),
		mkEntry(2026, 1, 1),
		mkEntry(2026, 1, 15),
		mkEntry(2026, 3, 1),
		mkEntry(2026, 3, 2),
	}

	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		from, to time.Time
		wantLen  int
		wantURLs []string
	}{
		{
			name:     "no filter",
			wantLen:  5,
		},
		{
			name:     "from only",
			from:     from,
			wantLen:  4,
			wantURLs: []string{"20260101", "20260115", "20260301", "20260302"},
		},
		{
			name:     "to only",
			to:       to,
			wantLen:  4,
			wantURLs: []string{"20251231", "20260101", "20260115", "20260301"},
		},
		{
			name:     "from and to",
			from:     from,
			to:       to,
			wantLen:  3,
			wantURLs: []string{"20260101", "20260115", "20260301"},
		},
		{
			name:     "from equals to (single day)",
			from:     from,
			to:       from,
			wantLen:  1,
			wantURLs: []string{"20260101"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterEntriesByDate(entries, tt.from, tt.to)
			if len(got) != tt.wantLen {
				t.Errorf("len=%d, want %d", len(got), tt.wantLen)
			}
			for i, wantURL := range tt.wantURLs {
				if i >= len(got) {
					break
				}
				if !strings.Contains(got[i].EditURL, wantURL) {
					t.Errorf("entry[%d] EditURL=%q, want suffix %q", i, got[i].EditURL, wantURL)
				}
			}
		})
	}
}

