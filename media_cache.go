package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func mediaCacheRoot() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("locating cache directory: %w", err)
	}
	return filepath.Join(cacheDir, settingsAppName, "media"), nil
}

func mediaCachePath(fileID int64, filename string) (string, error) {
	root, err := mediaCacheRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, fmt.Sprintf("%d", fileID), sanitizeFileName(filename)), nil
}

func (s *BunkrService) cachedMediaPath(fileID int64, filename string, expectedSize int64) (string, bool) {
	path, err := mediaCachePath(fileID, filename)
	if err != nil {
		return "", false
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		return "", false
	}
	if expectedSize > 0 && info.Size() != expectedSize {
		return "", false
	}
	return path, true
}

func (s *BunkrService) CacheMediaFile(fileID int64) error {
	if fileID <= 0 {
		return fmt.Errorf("invalid file id")
	}

	s.mu.RLock()
	album := s.activeAlbum
	s.mu.RUnlock()
	if album == nil {
		return fmt.Errorf("no album loaded")
	}

	var file *AlbumFile
	for i := range album.Files {
		if album.Files[i].FileID == fileID {
			file = &album.Files[i]
			break
		}
	}
	if file == nil {
		return fmt.Errorf("file not found in active album")
	}
	if !isImageFile(*file) {
		return nil
	}

	if _, ok := s.cachedMediaPath(fileID, file.Name, file.SizeBytes); ok {
		return nil
	}

	s.cacheMu.Lock()
	if s.cacheInflight == nil {
		s.cacheInflight = make(map[int64]bool)
	}
	if s.cacheInflight[fileID] {
		s.cacheMu.Unlock()
		return nil
	}
	s.cacheInflight[fileID] = true
	s.cacheMu.Unlock()

	go func(f AlbumFile) {
		defer func() {
			s.cacheMu.Lock()
			delete(s.cacheInflight, f.FileID)
			s.cacheMu.Unlock()
		}()
		_ = s.cacheMediaFile(f)
	}(*file)

	return nil
}

func isImageFile(file AlbumFile) bool {
	switch strings.ToLower(file.Type) {
	case "image":
		return true
	}
	return strings.HasPrefix(strings.ToLower(file.MimeType), "image/")
}

func (s *BunkrService) cacheMediaFile(file AlbumFile) error {
	if _, ok := s.cachedMediaPath(file.FileID, file.Name, file.SizeBytes); ok {
		return nil
	}

	mediaURL, err := s.ResolveMediaURL(file.FileID)
	if err != nil {
		return err
	}

	destPath, err := mediaCachePath(file.FileID, file.Name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}

	referer := fmt.Sprintf("%s/file/%d", bunkrDownloadRef, file.FileID)
	request, err := http.NewRequest(http.MethodGet, mediaURL, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Referer", referer)
	request.Header.Set("Origin", bunkrDownloadRef)
	request.Header.Set("User-Agent", httpUserAgent)

	response, err := s.downloadClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("cache download failed: %s", response.Status)
	}

	tempPath := destPath + ".part"
	out, err := os.Create(tempPath)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, response.Body)
	closeErr := out.Close()
	if err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	if closeErr != nil {
		_ = os.Remove(tempPath)
		return closeErr
	}
	return os.Rename(tempPath, destPath)
}

func copyCachedFile(ctx context.Context, srcPath, destPath string, total int64, onProgress func(int64, int64)) error {
	in, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer in.Close()

	if total <= 0 {
		if info, statErr := in.Stat(); statErr == nil {
			total = info.Size()
		}
	}

	tempPath := destPath + ".part"
	out, err := os.Create(tempPath)
	if err != nil {
		return err
	}

	written, err := copyWithProgress(ctx, out, in, total, onProgress)
	closeErr := out.Close()
	if err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	if closeErr != nil {
		_ = os.Remove(tempPath)
		return closeErr
	}
	if err := os.Rename(tempPath, destPath); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	if onProgress != nil {
		onProgress(written, total)
	}
	return nil
}
