package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMediaCachePath(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	path, err := mediaCachePath(61611570, "photo.png")
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(path) {
		t.Fatalf("expected absolute path, got %q", path)
	}
	if filepath.Base(path) != "photo.png" {
		t.Fatalf("unexpected filename in path: %q", path)
	}
}

func TestCachedMediaPathHit(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	dest, err := mediaCachePath(42, "cached.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	service := NewBunkrService()
	got, ok := service.cachedMediaPath(42, "cached.jpg", 5)
	if !ok || got != dest {
		t.Fatalf("expected cache hit for %q, got %q ok=%v", dest, got, ok)
	}
}

func TestCachedMediaPathMissWhenSizeMismatch(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	dest, err := mediaCachePath(99, "size.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte("123456"), 0o644); err != nil {
		t.Fatal(err)
	}

	service := NewBunkrService()
	if _, ok := service.cachedMediaPath(99, "size.jpg", 100); ok {
		t.Fatal("expected cache miss when size does not match")
	}
}

func TestIsImageFile(t *testing.T) {
	if !isImageFile(AlbumFile{Type: "Image"}) {
		t.Fatal("expected image type to match")
	}
	if isImageFile(AlbumFile{Type: "Video", MimeType: "video/mp4"}) {
		t.Fatal("expected video to be rejected")
	}
}
