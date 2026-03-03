package hatena

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestParseEntry_Single(t *testing.T) {
	data, err := os.ReadFile("testdata/entry_single.xml")
	if err != nil {
		t.Fatal(err)
	}
	e, err := parseEntry(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.Title != "Test Entry Title" {
		t.Errorf("title: got %q", e.Title)
	}
	if e.Content != "This is the **body** content." {
		t.Errorf("content: got %q", e.Content)
	}
	if e.Draft {
		t.Error("draft should be false")
	}
	if e.EditURL != "https://blog.hatena.ne.jp/user/example.hateblo.jp/atom/entry/123456789" {
		t.Errorf("editURL: got %q", e.EditURL)
	}
	if e.URL != "https://example.hateblo.jp/entry/2026/03/01/120000" {
		t.Errorf("url: got %q", e.URL)
	}
	if e.CustomURL != "my-custom-path" {
		t.Errorf("customURL: got %q", e.CustomURL)
	}
	if len(e.Categories) != 2 || e.Categories[0] != "Go" || e.Categories[1] != "CLI" {
		t.Errorf("categories: got %v", e.Categories)
	}
	wantDate := time.Date(2026, 3, 1, 3, 0, 0, 0, time.UTC) // published +09:00 → UTC
	if !e.Date.UTC().Equal(wantDate) {
		t.Errorf("date: got %v, want %v", e.Date.UTC(), wantDate)
	}
	wantUpdated := time.Date(2026, 3, 1, 3, 0, 0, 0, time.UTC) // updated +09:00 → UTC
	if !e.Updated.UTC().Equal(wantUpdated) {
		t.Errorf("updated: got %v, want %v", e.Updated.UTC(), wantUpdated)
	}
}

func TestParseEntry_Draft(t *testing.T) {
	data, err := os.ReadFile("testdata/entry_draft.xml")
	if err != nil {
		t.Fatal(err)
	}
	e, err := parseEntry(data)
	if err != nil {
		t.Fatal(err)
	}
	if !e.Draft {
		t.Error("draft should be true")
	}
	if e.Title != "Draft Entry" {
		t.Errorf("title: got %q", e.Title)
	}
}

func TestParseFeed_Pagination(t *testing.T) {
	data, err := os.ReadFile("testdata/feed_page1.xml")
	if err != nil {
		t.Fatal(err)
	}
	entries, nextURL, err := parseFeed(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Title != "Entry One" {
		t.Errorf("title: got %q", entries[0].Title)
	}
	if !strings.Contains(nextURL, "page=2") {
		t.Errorf("nextURL: got %q", nextURL)
	}
}

func TestParseFeed_LastPage(t *testing.T) {
	data, err := os.ReadFile("testdata/feed_page2.xml")
	if err != nil {
		t.Fatal(err)
	}
	entries, nextURL, err := parseFeed(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if nextURL != "" {
		t.Errorf("nextURL should be empty, got %q", nextURL)
	}
}

func TestParseEntry_Scheduled(t *testing.T) {
	data, err := os.ReadFile("testdata/entry_scheduled.xml")
	if err != nil {
		t.Fatal(err)
	}
	e, err := parseEntry(data)
	if err != nil {
		t.Fatal(err)
	}
	if !e.Draft {
		t.Error("draft should be true for scheduled entry")
	}
	wantScheduled := time.Date(2026, 4, 1, 3, 0, 0, 0, time.UTC) // +09:00 → UTC
	if e.ScheduledAt.IsZero() {
		t.Error("ScheduledAt should be set for scheduled entry")
	}
	if !e.ScheduledAt.UTC().Equal(wantScheduled) {
		t.Errorf("ScheduledAt: got %v, want %v", e.ScheduledAt.UTC(), wantScheduled)
	}
}

func TestParseEntry_Draft_NoScheduled(t *testing.T) {
	data, err := os.ReadFile("testdata/entry_draft.xml")
	if err != nil {
		t.Fatal(err)
	}
	e, err := parseEntry(data)
	if err != nil {
		t.Fatal(err)
	}
	if !e.ScheduledAt.IsZero() {
		t.Errorf("ScheduledAt should be zero for plain draft, got %v", e.ScheduledAt)
	}
}

func TestMarshalEntry_Scheduled(t *testing.T) {
	scheduledAt := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	e := &Entry{
		Title:       "Scheduled",
		Content:     "body",
		Draft:       false, // intentionally false; marshalEntry must force draft=yes
		ScheduledAt: scheduledAt,
	}
	data, err := marshalEntry(e)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parseEntry(data)
	if err != nil {
		t.Fatalf("round-trip parse: %v\nXML:\n%s", err, data)
	}
	if !parsed.Draft {
		t.Error("draft should be true for scheduled entry after round-trip")
	}
	if !parsed.ScheduledAt.Equal(scheduledAt) {
		t.Errorf("ScheduledAt: got %v, want %v", parsed.ScheduledAt, scheduledAt)
	}
	// updated must reflect scheduledAt, not Updated field
	if !parsed.Updated.Equal(scheduledAt) {
		t.Errorf("Updated should equal ScheduledAt: got %v, want %v", parsed.Updated, scheduledAt)
	}
}

func TestMarshalEntry_Scheduled_OverridesUpdated(t *testing.T) {
	scheduledAt := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	updated := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	e := &Entry{
		Title:       "Scheduled Override",
		Content:     "body",
		Draft:       true,
		Updated:     updated,
		ScheduledAt: scheduledAt,
	}
	data, err := marshalEntry(e)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parseEntry(data)
	if err != nil {
		t.Fatalf("round-trip parse: %v\nXML:\n%s", err, data)
	}
	// ScheduledAt takes priority over Updated
	if !parsed.ScheduledAt.Equal(scheduledAt) {
		t.Errorf("ScheduledAt: got %v, want %v", parsed.ScheduledAt, scheduledAt)
	}
}

func TestMarshalEntry_RoundTrip(t *testing.T) {
	e := &Entry{
		Title:      "Round Trip",
		Content:    "body content",
		Draft:      true,
		Categories: []string{"Go", "Test"},
		CustomURL:  "round-trip",
		Updated:    time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
	}
	data, err := marshalEntry(e)
	if err != nil {
		t.Fatal(err)
	}
	// parse back
	parsed, err := parseEntry(data)
	if err != nil {
		t.Fatalf("round-trip parse: %v\nXML:\n%s", err, data)
	}
	if parsed.Title != e.Title {
		t.Errorf("title: got %q", parsed.Title)
	}
	if parsed.Content != e.Content {
		t.Errorf("content: got %q", parsed.Content)
	}
	if !parsed.Draft {
		t.Error("draft should be true")
	}
	if len(parsed.Categories) != 2 {
		t.Errorf("categories: got %v", parsed.Categories)
	}
	if parsed.CustomURL != "round-trip" {
		t.Errorf("customURL: got %q", parsed.CustomURL)
	}
}
