package main

import (
	"testing"
)

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
