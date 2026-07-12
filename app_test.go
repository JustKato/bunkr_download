package main

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestParseAlbumExtractsMixedFileTypes(t *testing.T) {
	pageURL, err := url.Parse("https://example.invalid/album-redacted")
	if err != nil {
		t.Fatal(err)
	}

	htmlContent := `
		<h1>Example Album</h1>
		<span class="font-semibold">(1.24 GB) 2 Files</span>
		<div class="theItem" title="photo.png">
			<span class="type-Image"><img src="/images/image.svg"></span>
			<img class="grid-images_box-img" src="https://static.example/thumb.png">
			<p class="theName">photo.png</p><p class="theSize">420 KB</p>
			<span class="theDate">12:00 01/01/2026</span>
			<a href="/f/photo">download</a>
		</div>
		<div class="theItem" title="clip.mp4">
			<span class="type-Video"></span>
			<video poster="/thumbs/clip.jpg"></video>
			<p class="theName">clip.mp4</p><p class="theSize">1.23 GB</p>
			<span class="theDate">12:01 01/01/2026</span>
			<a href="/f/clip">download</a>
		</div>
	`

	album, err := parseAlbumPage(pageURL, htmlContent)
	if err != nil {
		t.Fatalf("parseAlbumPage returned an error: %v", err)
	}

	if album.Title != "Example Album" || album.TotalSize != "1.24 GB" || album.FileCount != 2 {
		t.Fatalf("unexpected album metadata: %#v", album)
	}
	if len(album.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(album.Files))
	}

	image := album.Files[0]
	if image.Type != "Image" || image.PreviewURL != "https://static.example/thumb.png" || image.FileURL != "https://bunkr.cr/f/photo" {
		t.Fatalf("unexpected image file: %#v", image)
	}

	video := album.Files[1]
	if video.Type != "Video" || video.PreviewURL != "https://bunkr.cr/thumbs/clip.jpg" || video.FileURL != "https://bunkr.cr/f/clip" {
		t.Fatalf("unexpected video file: %#v", video)
	}
}

func TestParseAlbumFilesJS(t *testing.T) {
	pageURL, err := url.Parse("https://example.invalid/album-redacted")
	if err != nil {
		t.Fatal(err)
	}

	htmlContent := `
		<span class="font-semibold">(6.58 MB) 2 Files</span>
		window.albumFiles = [
		{
		  id: 61611570,
		  original: "Kalinka Fox - Midnight (11).png",
		  slug: "YyEP5tNw0wGMi",
		  type: "image/png",
		  extension: "Image",
		  size: 4105810,
		  timestamp: "01:08:38 12/07/2026",
		  thumbnail: "https://static.example/thumb.png"
		},
		{
		  id: 61611569,
		  original: "clip.mp4",
		  slug: "clipSlug",
		  type: "video/mp4",
		  extension: "Video",
		  size: 1234567,
		  timestamp: "01:08:37 12/07/2026",
		  thumbnail: "https://static.example/vthumb.png"
		}
		];
	`

	files, err := parseAlbumFilesJS(pageURL, htmlContent)
	if err != nil {
		t.Fatalf("parseAlbumFilesJS returned an error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}

	image := files[0]
	if image.FileID != 61611570 || image.Name != "Kalinka Fox - Midnight (11).png" || image.Type != "Image" {
		t.Fatalf("unexpected image entry: %#v", image)
	}
	if image.FileURL != "https://bunkr.cr/f/YyEP5tNw0wGMi" || image.Size != "3.92 MB" {
		t.Fatalf("unexpected image urls/size: %#v", image)
	}

	video := files[1]
	if !CanPreview(video) || video.MimeType != "video/mp4" {
		t.Fatalf("unexpected video entry: %#v", video)
	}
}

func TestCanPreview(t *testing.T) {
	cases := []struct {
		file AlbumFile
		want bool
	}{
		{AlbumFile{Type: "Image"}, true},
		{AlbumFile{Type: "Video"}, true},
		{AlbumFile{MimeType: "image/jpeg"}, true},
		{AlbumFile{Name: "notes.pdf"}, true},
		{AlbumFile{Type: "Audio", Name: "song.mp3"}, false},
		{AlbumFile{Type: "File", Name: "archive.zip"}, false},
	}

	for _, tc := range cases {
		if CanPreview(tc.file) != tc.want {
			t.Fatalf("CanPreview(%#v) = %v, want %v", tc.file, !tc.want, tc.want)
		}
	}
}

func TestMediaAPIRequestBodyUsesStringID(t *testing.T) {
	payload, err := mediaAPIRequestBody(61611570)
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) != `{"id":"61611570"}` {
		t.Fatalf("unexpected payload: %s", payload)
	}
}

func TestResolveMediaURLIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("needs network")
	}

	service := NewBunkrService()
	mediaURL, err := service.ResolveMediaURL(61611570)
	if err != nil {
		t.Fatalf("ResolveMediaURL failed: %v", err)
	}
	if !strings.Contains(mediaURL, "token=") || !strings.Contains(mediaURL, "cdn.cr") {
		t.Fatalf("expected signed CDN URL, got %q", mediaURL)
	}

	response, err := http.Get(mediaURL)
	if err != nil {
		t.Fatalf("GET signed URL failed: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("signed URL status %s", response.Status)
	}
}

func TestValidateURLRequiresSupportedBunkrAlbum(t *testing.T) {
	service := NewBunkrService()

	if _, err := service.ValidateURL(" https://example.invalid/album-redacted "); err != nil {
		t.Fatalf("expected valid album URL: %v", err)
	}
	if _, err := service.ValidateURL("https://example.invalid/album-redacted"); err != nil {
		t.Fatalf("expected bunkr.black album URL to be accepted: %v", err)
	}

	for _, raw := range []string{
		"https://example.com/a/Csx7AzrM",
		"https://bunkr.cr/f/Csx7AzrM",
		"file:///etc/passwd",
	} {
		if _, err := service.ValidateURL(raw); err == nil {
			t.Errorf("expected %q to be rejected", raw)
		}
	}
}

func TestCanonicalBunkrAlbumURL(t *testing.T) {
	got, err := canonicalBunkrAlbumURL("https://example.invalid/album-redacted")
	if err != nil {
		t.Fatalf("canonicalBunkrAlbumURL returned an error: %v", err)
	}
	if got != "https://example.invalid/album-redacted" {
		t.Fatalf("expected bunkr.black to rewrite to bunkr.cr, got %q", got)
	}

	unchanged, err := canonicalBunkrAlbumURL("https://example.invalid/album-redacted?foo=bar")
	if err != nil {
		t.Fatalf("canonicalBunkrAlbumURL returned an error: %v", err)
	}
	if unchanged != "https://example.invalid/album-redacted?foo=bar" {
		t.Fatalf("expected query to be preserved, got %q", unchanged)
	}
}
