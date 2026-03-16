package hatena

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func newTestClient(t *testing.T, mux *http.ServeMux) *Client {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c := NewClient("user", "example.hateblo.jp", "key")
	c.SetBaseURL(srv.URL)
	return c
}

func TestListEntries_Pagination(t *testing.T) {
	page1, err := os.ReadFile("testdata/feed_page1.xml")
	if err != nil {
		t.Fatal(err)
	}
	page2, err := os.ReadFile("testdata/feed_page2.xml")
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/user/example.hateblo.jp/atom/entry", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery == "page=2" {
			w.Write(page2)
			return
		}
		// rewrite next link to point to the test server
		body := strings.Replace(string(page1),
			"https://blog.hatena.ne.jp/user/example.hateblo.jp/atom/entry?page=2",
			r.Host+"/user/example.hateblo.jp/atom/entry?page=2", 1)
		// need full URL including scheme
		body = strings.Replace(body,
			r.Host+"/user/example.hateblo.jp/atom/entry?page=2",
			"http://"+r.Host+"/user/example.hateblo.jp/atom/entry?page=2", 1)
		w.Write([]byte(body))
	})

	c := newTestClient(t, mux)
	entries, err := c.ListEntries(context.Background(), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
}

func TestListEntries_MaxPages(t *testing.T) {
	// feedPage builds a feed XML using the request's host so the next-page URL
	// is determined at request time, avoiding any variable-before-assignment race.
	feedPage := func(r *http.Request, entryID, entryTitle, nextQuery string) string {
		base := "http://" + r.Host
		nextLink := ""
		if nextQuery != "" {
			nextLink = `<link rel="next" href="` + base + `/user/example.hateblo.jp/atom/entry?` + nextQuery + `"/>`
		}
		return `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:app="http://www.w3.org/2007/app">
  ` + nextLink + `
  <entry>
    <link rel="edit" href="` + base + `/user/example.hateblo.jp/atom/entry/` + entryID + `"/>
    <title>` + entryTitle + `</title>
    <updated>2026-03-01T12:00:00+09:00</updated>
    <published>2026-03-01T12:00:00+09:00</published>
    <content type="text/x-markdown">body</content>
    <app:control><app:draft>no</app:draft></app:control>
  </entry>
</feed>`
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/user/example.hateblo.jp/atom/entry", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.RawQuery {
		case "page=2":
			w.Write([]byte(feedPage(r, "2", "Entry Two", "page=3")))
		case "page=3":
			w.Write([]byte(feedPage(r, "3", "Entry Three", "")))
		default:
			w.Write([]byte(feedPage(r, "1", "Entry One", "page=2")))
		}
	})

	c := newTestClient(t, mux)

	entries, err := c.ListEntries(context.Background(), 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries (maxPages=2), got %d", len(entries))
	}
}

func TestGetEntry_OK(t *testing.T) {
	data, err := os.ReadFile("testdata/entry_single.xml")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/user/example.hateblo.jp/atom/entry/123456789", func(w http.ResponseWriter, r *http.Request) {
		w.Write(data)
	})
	c := newTestClient(t, mux)
	entry, err := c.GetEntry(context.Background(), c.baseURL+"/user/example.hateblo.jp/atom/entry/123456789")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Title != "Test Entry Title" {
		t.Errorf("title: got %q", entry.Title)
	}
}

func TestGetEntry_NotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/user/example.hateblo.jp/atom/entry/999", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	c := newTestClient(t, mux)
	_, err := c.GetEntry(context.Background(), c.baseURL+"/user/example.hateblo.jp/atom/entry/999")
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("expected 404 error, got %v", err)
	}
}

func TestGetEntry_Unauthorized(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/user/example.hateblo.jp/atom/entry/1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	c := newTestClient(t, mux)
	_, err := c.GetEntry(context.Background(), c.baseURL+"/user/example.hateblo.jp/atom/entry/1")
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Errorf("expected 401 error, got %v", err)
	}
}

func TestCreateEntry_OK(t *testing.T) {
	respData, err := os.ReadFile("testdata/entry_single.xml")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/user/example.hateblo.jp/atom/entry", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusCreated)
		w.Write(respData)
	})
	c := newTestClient(t, mux)
	e := &Entry{Title: "New", Content: "body"}
	created, err := c.CreateEntry(context.Background(), e)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.Title != "Test Entry Title" {
		t.Errorf("title: got %q", created.Title)
	}
}

func TestUpdateEntry_OK(t *testing.T) {
	respData, err := os.ReadFile("testdata/entry_single.xml")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/user/example.hateblo.jp/atom/entry/123456789", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Write(respData)
	})
	c := newTestClient(t, mux)
	e := &Entry{Title: "Updated", Content: "updated body"}
	updated, err := c.UpdateEntry(context.Background(), c.baseURL+"/user/example.hateblo.jp/atom/entry/123456789", e)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Title != "Test Entry Title" {
		t.Errorf("title: got %q", updated.Title)
	}
}

func TestUpdateEntry_NoContent(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/user/example.hateblo.jp/atom/entry/123", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	c := newTestClient(t, mux)
	e := &Entry{Title: "Updated", Content: "body"}
	got, err := c.UpdateEntry(context.Background(), c.baseURL+"/user/example.hateblo.jp/atom/entry/123", e)
	if err != nil {
		t.Fatalf("expected no error for 204 No Content, got %v", err)
	}
	if got != e {
		t.Errorf("expected sent entry to be returned on 204, got %v", got)
	}
}

func TestDeleteEntry_NoContent(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/user/example.hateblo.jp/atom/entry/123", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	c := newTestClient(t, mux)
	err := c.DeleteEntry(context.Background(), c.baseURL+"/user/example.hateblo.jp/atom/entry/123")
	if err != nil {
		t.Fatalf("expected no error for 204 No Content, got %v", err)
	}
}


func TestGetEntry_ContextCancel(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/user/example.hateblo.jp/atom/entry/1", func(w http.ResponseWriter, r *http.Request) {
		// server receives request but client already cancelled
		w.WriteHeader(http.StatusOK)
	})
	c := newTestClient(t, mux)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	_, err := c.GetEntry(ctx, c.baseURL+"/user/example.hateblo.jp/atom/entry/1")
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}
