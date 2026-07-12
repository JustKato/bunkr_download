package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractAlbumID(t *testing.T) {
	cases := map[string]string{
		"https://example.invalid/album-redacted":          "nwyCAZhf",
		"https://example.invalid/album-redacted/":      "nwyCAZhf",
		"https://bunkr.cr/f/somefile":          "",
		"https://example.com/a/nwyCAZhf":       "nwyCAZhf",
		"https://example.invalid/album-redacted": "bztbcGqM",
	}

	for raw, want := range cases {
		if got := extractAlbumID(raw); got != want {
			t.Errorf("extractAlbumID(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestRecordAlbumHistoryDedupesAndLimits(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	if err := recordAlbumHistory("https://example.invalid/album-redacted", "First"); err != nil {
		t.Fatalf("recordAlbumHistory failed: %v", err)
	}
	if err := recordAlbumHistory("https://example.invalid/album-redacted", "Second"); err != nil {
		t.Fatalf("recordAlbumHistory failed: %v", err)
	}
	if err := recordAlbumHistory("https://example.invalid/album-redacted", "First Updated"); err != nil {
		t.Fatalf("recordAlbumHistory failed: %v", err)
	}

	history, err := loadAlbumHistory()
	if err != nil {
		t.Fatalf("loadAlbumHistory failed: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(history))
	}
	if history[0].ID != "one" || history[0].Title != "First Updated" {
		t.Fatalf("expected updated first entry, got %#v", history[0])
	}
	if history[1].ID != "two" {
		t.Fatalf("expected second entry to remain, got %#v", history[1])
	}

	path, err := albumHistoryFilePath()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("history file not written: %v", err)
	}
	if filepath.Base(path) != "album_history.json" {
		t.Fatalf("unexpected history file name: %s", path)
	}
}

func TestGetAlbumHistoryReturnsEmptyOnMissingFile(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	service := NewBunkrService()
	history := service.GetAlbumHistory()
	if len(history) != 0 {
		t.Fatalf("expected empty history, got %#v", history)
	}
}
