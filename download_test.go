package main

import "testing"

func TestFilterDownloadFilesByType(t *testing.T) {
	files := []AlbumFile{
		{Name: "photo.jpg", Type: "Image"},
		{Name: "clip.mp4", Type: "Video"},
		{Name: "song.mp3", Type: "Audio"},
		{Name: "notes.txt", Type: "File"},
	}

	filtered := filterDownloadFiles(files, DownloadOptions{
		Types: []string{"Image", "Video"},
	})
	if len(filtered) != 2 {
		t.Fatalf("expected 2 files, got %d", len(filtered))
	}
}

func TestFilterDownloadFilesByIncludePatterns(t *testing.T) {
	files := []AlbumFile{
		{Name: "set-01.jpg", Type: "Image"},
		{Name: "other.png", Type: "Image"},
		{Name: "clip.mp4", Type: "Video"},
	}

	filtered := filterDownloadFiles(files, DownloadOptions{
		Types:           []string{"Image", "Video"},
		IncludePatterns: []string{"*set*"},
	})
	if len(filtered) != 1 || filtered[0].Name != "set-01.jpg" {
		t.Fatalf("unexpected filtered files: %#v", filtered)
	}
}

func TestSanitizePathName(t *testing.T) {
	if got := sanitizePathName(`Album: Test / Part 1`); got != "Album_ Test _ Part 1" {
		t.Fatalf("unexpected sanitized path: %q", got)
	}
	if got := sanitizePathName(""); got != "album" {
		t.Fatalf("expected default album folder name, got %q", got)
	}
}

func TestMatchesIncludePatterns(t *testing.T) {
	patterns := parseIncludePatterns([]string{"*.jpg, *set*"})
	if !matchesIncludePatterns("photo.jpg", patterns) {
		t.Fatal("expected *.jpg to match photo.jpg")
	}
	if !matchesIncludePatterns("my-set-01.png", patterns) {
		t.Fatal("expected *set* to match my-set-01.png")
	}
	if matchesIncludePatterns("notes.txt", patterns) {
		t.Fatal("expected notes.txt to be rejected")
	}
}

func TestNormalizeDownloadType(t *testing.T) {
	if normalizeDownloadType("Image") != "image" {
		t.Fatal("expected image type normalization")
	}
	if normalizeDownloadType("Other") != "file" {
		t.Fatal("expected unknown types to map to file")
	}
}
