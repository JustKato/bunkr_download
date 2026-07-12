package main

import (
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
)

const mediaCacheURLPrefix = "/media-cache/"

type PreviewMediaSource struct {
	URL  string `json:"url"`
	Kind string `json:"kind"`
}

func previewKindForFile(file AlbumFile) string {
	if isPDF(file) {
		return "pdf"
	}
	if isVideoFile(file) {
		return "video"
	}
	if isImageFile(file) {
		return "image"
	}
	return "file"
}

func isPDF(file AlbumFile) bool {
	if strings.EqualFold(filepath.Ext(file.Name), ".pdf") {
		return true
	}
	return strings.EqualFold(file.MimeType, "application/pdf")
}

func isVideoFile(file AlbumFile) bool {
	switch strings.ToLower(file.Type) {
	case "video":
		return true
	}
	return strings.HasPrefix(strings.ToLower(file.MimeType), "video/")
}

func mediaCachePublicURL(fileID int64, filename string) string {
	return mediaCacheURLPrefix + strconv.FormatInt(fileID, 10) + "/" + url.PathEscape(sanitizeFileName(filename))
}

func (s *BunkrService) PreparePreviewMedia(index int) (*PreviewMediaSource, error) {
	file, _, err := s.albumFileAt(index)
	if err != nil {
		return nil, err
	}
	if !CanPreview(file) {
		return nil, fmt.Errorf("no preview available for this file")
	}

	appLog("info", "preview", "preparing preview for %q (fileID=%d)", file.Name, file.FileID)
	if err := s.ensureMediaCached(file); err != nil {
		appLog("error", "preview", "cache failed for %q: %v", file.Name, err)
		return nil, err
	}

	kind := previewKindForFile(file)
	source := &PreviewMediaSource{
		URL:  mediaCachePublicURL(file.FileID, file.Name),
		Kind: kind,
	}
	appLog("info", "preview", "preview ready for %q at %s", file.Name, source.URL)
	return source, nil
}

func (s *BunkrService) ensureMediaCached(file AlbumFile) error {
	if _, ok := s.cachedMediaPath(file.FileID, file.Name, file.SizeBytes); ok {
		return nil
	}
	return s.cacheMediaFile(file)
}

func serveMediaCache(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rel := strings.TrimPrefix(r.URL.Path, mediaCacheURLPrefix)
	parts := strings.SplitN(rel, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		http.NotFound(w, r)
		return
	}

	fileID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || fileID <= 0 {
		http.NotFound(w, r)
		return
	}

	filename, err := url.PathUnescape(parts[1])
	if err != nil || filename == "" {
		http.NotFound(w, r)
		return
	}

	path, err := mediaCachePath(fileID, filename)
	if err != nil {
		http.Error(w, "invalid media path", http.StatusBadRequest)
		return
	}

	root, err := mediaCacheRoot()
	if err != nil {
		http.Error(w, "cache unavailable", http.StatusInternalServerError)
		return
	}

	cleanRoot := filepath.Clean(root)
	cleanPath := filepath.Clean(path)
	if !strings.HasPrefix(cleanPath, cleanRoot+string(filepath.Separator)) {
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, cleanPath)
}
