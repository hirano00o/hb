package hatena

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUploadImage_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/atom/post", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusCreated)
		// Minimal Fotlife AtomPub response with hatena:syntax
		w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
<entry xmlns="http://purl.org/atom/ns#" xmlns:hatena="http://www.hatena.ne.jp/info/xmlns#">
  <title>photo.jpg</title>
  <hatena:syntax>[f:id:user:20260303120000j:image]</hatena:syntax>
</entry>`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	imgPath := filepath.Join(dir, "photo.jpg")
	// Minimal valid JPEG header bytes
	jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00}
	if err := os.WriteFile(imgPath, jpegData, 0o644); err != nil {
		t.Fatal(err)
	}

	c := NewClient("user", "example.hateblo.jp", "key")
	c.SetFotolifeURL(srv.URL + "/atom/post")

	syntax, err := c.UploadImage(context.Background(), imgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if syntax != "[f:id:user:20260303120000j:image]" {
		t.Errorf("unexpected syntax: %q", syntax)
	}
}

func TestUploadImage_FileNotFound(t *testing.T) {
	c := NewClient("user", "example.hateblo.jp", "key")
	_, err := c.UploadImage(context.Background(), "/nonexistent/photo.jpg")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestUploadImage_FileTooLarge(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "big.jpg")
	// Write a file just over 10 MB.
	data := make([]byte, maxImageSize+1)
	if err := os.WriteFile(imgPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	c := NewClient("user", "example.hateblo.jp", "key")
	_, err := c.UploadImage(context.Background(), imgPath)
	if err == nil {
		t.Fatal("expected error for oversized file")
	}
	if !strings.Contains(err.Error(), "exceeds maximum size") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestUploadImage_Unauthorized(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/atom/post", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	imgPath := filepath.Join(dir, "photo.jpg")
	if err := os.WriteFile(imgPath, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := NewClient("user", "example.hateblo.jp", "key")
	c.SetFotolifeURL(srv.URL + "/atom/post")

	_, err := c.UploadImage(context.Background(), imgPath)
	if err == nil {
		t.Fatal("expected error for 401")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected 401 error, got: %v", err)
	}
}

func TestUploadImage_ServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/atom/post", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	imgPath := filepath.Join(dir, "photo.jpg")
	if err := os.WriteFile(imgPath, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := NewClient("user", "example.hateblo.jp", "key")
	c.SetFotolifeURL(srv.URL + "/atom/post")

	_, err := c.UploadImage(context.Background(), imgPath)
	if err == nil {
		t.Fatal("expected error for 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected 500 error, got: %v", err)
	}
}
