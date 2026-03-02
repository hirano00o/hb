package article

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReplaceLocalImages_NoImages(t *testing.T) {
	body := "# Hello\n\nNo images here.\n"
	got, err := ReplaceLocalImages(context.Background(), body, "/tmp", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != body {
		t.Errorf("expected unchanged body, got %q", got)
	}
}

func TestReplaceLocalImages_RemoteURLSkipped(t *testing.T) {
	body := "![alt](https://example.com/image.png)\n"
	got, err := ReplaceLocalImages(context.Background(), body, "/tmp", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != body {
		t.Errorf("expected unchanged body for remote URL, got %q", got)
	}
}

func TestReplaceLocalImages_HTTPSkipped(t *testing.T) {
	body := "![alt](http://example.com/image.png)\n"
	got, err := ReplaceLocalImages(context.Background(), body, "/tmp", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != body {
		t.Errorf("expected unchanged body for http URL, got %q", got)
	}
}

func TestReplaceLocalImages_LocalReplaced(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "photo.jpg")
	if err := os.WriteFile(imgPath, []byte("fake-jpeg"), 0o644); err != nil {
		t.Fatal(err)
	}

	uploader := func(_ context.Context, filePath string) (string, error) {
		if filePath != imgPath {
			t.Errorf("unexpected filePath: got %q, want %q", filePath, imgPath)
		}
		return "[f:id:user:20260303120000j:image]", nil
	}

	body := "![myalt](photo.jpg)\n"
	got, err := ReplaceLocalImages(context.Background(), body, dir, uploader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "[f:id:user:20260303120000j:image]\n"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestReplaceLocalImages_Mixed(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "local.png")
	if err := os.WriteFile(imgPath, []byte("fake-png"), 0o644); err != nil {
		t.Fatal(err)
	}

	uploader := func(_ context.Context, _ string) (string, error) {
		return "[f:id:user:20260303120001p:image]", nil
	}

	body := "![a](https://example.com/remote.png) and ![b](local.png)\n"
	got, err := ReplaceLocalImages(context.Background(), body, dir, uploader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "![a](https://example.com/remote.png) and [f:id:user:20260303120001p:image]\n"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestReplaceLocalImages_UploaderError(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "bad.jpg")
	if err := os.WriteFile(imgPath, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	uploadErr := errors.New("upload failed")
	uploader := func(_ context.Context, _ string) (string, error) {
		return "", uploadErr
	}

	body := "![alt](bad.jpg)\n"
	_, err := ReplaceLocalImages(context.Background(), body, dir, uploader)
	if !errors.Is(err, uploadErr) {
		t.Errorf("expected upload error, got %v", err)
	}
}
