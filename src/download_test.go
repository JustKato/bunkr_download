package main

import (
	"path/filepath"
	"testing"
	"time"
)

func TestCleanDisplayNameStripsWeirdRunes(t *testing.T) {
	got := cleanDisplayName("Photo\u0080\u0099s.jpg")
	if got != "Photos.jpg" {
		t.Fatalf("unexpected cleaned name: %q", got)
	}
}

func TestSanitizePathName(t *testing.T) {
	if got := sanitizePathName(`Album: Test / Part 1`); got != "Album_Test_Part_1" {
		t.Fatalf("unexpected sanitized path: %q", got)
	}
	if got := sanitizePathName(""); got != "album" {
		t.Fatalf("expected default album folder name, got %q", got)
	}
}

func TestSanitizeFileName(t *testing.T) {
	if got := sanitizeFileName(`weird\u0080name test.jpg`); got != "weirdname_test.jpg" {
		t.Fatalf("unexpected sanitized file name: %q", got)
	}
}

func TestDownloadDestPathUsesSanitizedNames(t *testing.T) {
	file := AlbumFile{Name: "My Photo.jpg"}
	got := downloadDestPath("/tmp/out", "Album: One", file, true)
	want := filepath.Join("/tmp/out", "Album_One", "My_Photo.jpg")
	if got != want {
		t.Fatalf("unexpected dest path: %q", got)
	}

	flat := downloadDestPath("/tmp/out", "Album: One", file, false)
	flatWant := filepath.Join("/tmp/out", "My_Photo.jpg")
	if flat != flatWant {
		t.Fatalf("unexpected flat dest path: %q", flat)
	}
}

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

func TestComputeETASeconds(t *testing.T) {
	remaining := computeETASeconds(50, 100, 10*time.Second)
	if remaining < 9.5 || remaining > 10.5 {
		t.Fatalf("expected ~10s remaining, got %v", remaining)
	}
	if got := formatETA(remaining); got != "10s" {
		t.Fatalf("unexpected eta format: %q", got)
	}
	if computeETASeconds(0, 100, time.Second) >= 0 {
		t.Fatal("expected invalid eta for zero progress")
	}
}

func TestProgressTrackerBytes(t *testing.T) {
	tracker := newProgressTracker(2, 200)
	file := AlbumFile{FileID: 1, Name: "a.jpg", SizeBytes: 100}
	tracker.markDone(file, 0)
	snap := tracker.snapshot(true, false, "")
	if snap.CompletedBytes != 100 || snap.CompletedCount != 1 {
		t.Fatalf("unexpected snapshot after done: %#v", snap)
	}
	tracker.updateActive(AlbumFile{FileID: 2, Name: "b.jpg", SizeBytes: 100}, 1, 40, 100, "downloading")
	snap = tracker.snapshot(true, false, "")
	if snap.CompletedBytes != 140 {
		t.Fatalf("expected active bytes included, got %d", snap.CompletedBytes)
	}
}

func TestNormalizeAppSettingsDefaults(t *testing.T) {
	settings := normalizeAppSettings(defaultAppSettings())
	if settings.PageSize != defaultPageSize {
		t.Fatalf("expected default page size, got %d", settings.PageSize)
	}
	if settings.ParallelDownloads != defaultParallelDownloads {
		t.Fatalf("expected default parallel downloads, got %d", settings.ParallelDownloads)
	}
	if !settings.SkipExistingFiles || !settings.CreateAlbumSubfolder {
		t.Fatal("expected default download behavior flags")
	}
}

func TestNormalizeAppSettingsClamps(t *testing.T) {
	settings := normalizeAppSettings(AppSettings{
		PageSize:          3,
		MaxAlbumHistory:   99999,
		MaxHTTPRetries:    99,
		ParallelDownloads: 99,
	})
	if settings.PageSize != minPageSize {
		t.Fatalf("expected clamped page size, got %d", settings.PageSize)
	}
	if settings.MaxAlbumHistory != maxMaxAlbumHistoryLimit {
		t.Fatalf("expected clamped history limit, got %d", settings.MaxAlbumHistory)
	}
	if settings.ParallelDownloads != maxParallelDownloads {
		t.Fatalf("expected clamped parallel downloads, got %d", settings.ParallelDownloads)
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