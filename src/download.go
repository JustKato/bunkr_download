package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const downloadProgressEvent = "download:progress"

type DownloadOptions struct {
	Types           []string `json:"types"`
	IncludePatterns []string `json:"includePatterns"`
}

type DownloadProgress struct {
	Running        bool   `json:"running"`
	Cancelled      bool   `json:"cancelled"`
	CurrentName    string `json:"currentName"`
	CurrentIndex   int    `json:"currentIndex"`
	CurrentBytes   int64  `json:"currentBytes"`
	CurrentTotal   int64  `json:"currentTotal"`
	CompletedCount int    `json:"completedCount"`
	TotalCount     int    `json:"totalCount"`
	TotalBytes     int64  `json:"totalBytes"`
	CompletedBytes int64  `json:"completedBytes"`
	StartedAtMs    int64  `json:"startedAtMs"`
	FileStatus     string `json:"fileStatus"`
	Error          string `json:"error"`
}

func (s *BunkrService) GetOutputFolder() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.outputFolder
}

func (s *BunkrService) ChooseOutputFolder() (string, error) {
	app := application.Get()
	if app == nil {
		return "", fmt.Errorf("application not ready")
	}

	dialog := app.Dialog.OpenFile().
		SetTitle("Select Download Folder").
		CanChooseDirectories(true).
		CanChooseFiles(false).
		CanCreateDirectories(true)

	path, err := dialog.PromptForSingleSelection()
	if err != nil {
		return "", err
	}
	if path == "" {
		return s.GetOutputFolder(), nil
	}

	if err := s.SetOutputFolder(path); err != nil {
		return "", err
	}
	return path, nil
}

func (s *BunkrService) OpenOutputFolder() error {
	folder := s.GetOutputFolder()
	if folder == "" {
		return fmt.Errorf("no output folder selected")
	}
	if _, err := os.Stat(folder); err != nil {
		return fmt.Errorf("output folder not found")
	}
	return openPath(folder)
}

func (s *BunkrService) Quit() {
	app := application.Get()
	if app != nil {
		app.Quit()
	}
}

func (s *BunkrService) CancelDownload() {
	if s.downloadCancel != nil {
		s.downloadCancel()
	}
}

func (s *BunkrService) GetDownloadProgress() DownloadProgress {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.downloadProgress
}

func (s *BunkrService) StartDownload(opts DownloadOptions) error {
	s.mu.RLock()
	album := s.activeAlbum
	outputFolder := s.outputFolder
	s.mu.RUnlock()

	if album == nil {
		return fmt.Errorf("load an album before downloading")
	}
	if outputFolder == "" {
		return fmt.Errorf("select an output folder first")
	}
	if _, err := os.Stat(outputFolder); err != nil {
		return fmt.Errorf("output folder not found: %s", outputFolder)
	}

	if atomic.LoadInt32(&s.downloadRunning) == 1 {
		s.mu.RLock()
		cancel := s.downloadCancel
		running := s.downloadProgress.Running
		s.mu.RUnlock()
		if cancel != nil || running {
			return fmt.Errorf("download already in progress")
		}
		atomic.StoreInt32(&s.downloadRunning, 0)
		appLog("warn", "download", "recovered stale download state")
	}

	filtered := filterDownloadFiles(album.Files, opts)
	if len(filtered) == 0 {
		return fmt.Errorf("no files match the current filters")
	}

	appLog("info", "download", "starting album download: %d files to %q", len(filtered), outputFolder)

	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.downloadCancel = cancel
	s.mu.Unlock()

	s.emitDownloadProgress(DownloadProgress{
		Running:        true,
		CompletedCount: 0,
		TotalCount:     len(filtered),
		FileStatus:     "starting",
	})

	go s.runDownload(ctx, album, outputFolder, filtered)
	return nil
}

func (s *BunkrService) runDownload(ctx context.Context, album *Album, outputFolder string, files []AlbumFile) {
	atomic.StoreInt32(&s.downloadRunning, 1)
	defer func() {
		atomic.StoreInt32(&s.downloadRunning, 0)
		s.mu.Lock()
		s.downloadCancel = nil
		s.mu.Unlock()
		appLog("info", "download", "download worker finished")
	}()

	settings := s.GetSettings()
	destRoot := outputFolder
	if settings.CreateAlbumSubfolder {
		destRoot = filepath.Join(outputFolder, sanitizePathName(album.Title))
	}
	if err := os.MkdirAll(destRoot, 0o755); err != nil {
		appLog("error", "download", "creating download folder: %v", err)
		s.emitDownloadProgress(DownloadProgress{
			Running: false,
			Error:   fmt.Sprintf("creating download folder: %v", err),
		})
		return
	}

	tracker := newProgressTracker(len(files), sumFileBytes(files))
	s.emitProgress(tracker.snapshot(true, false, ""))

	jobs := make(chan downloadJob, len(files))
	for _, file := range files {
		jobs <- downloadJob{
			file:     file,
			index:    fileAlbumIndex(album.Files, file),
			destPath: downloadDestPath(outputFolder, album.Title, file, settings.CreateAlbumSubfolder),
		}
	}
	close(jobs)

	workers := settings.ParallelDownloads
	if workers < 1 {
		workers = 1
	}

	var wg sync.WaitGroup
	var failOnce sync.Once
	var firstErr error

	emit := func(running bool, cancelled bool, errMsg string) {
		s.emitProgress(tracker.snapshot(running, cancelled, errMsg))
	}

	worker := func() {
		defer wg.Done()
		for job := range jobs {
			if ctx.Err() != nil {
				return
			}

			if settings.SkipExistingFiles {
				if _, err := os.Stat(job.destPath); err == nil {
					tracker.markSkipped(job.file, job.index)
					emit(true, false, "")
					continue
				}
			}

			tracker.setPhase(job.file.Name, job.index, job.file.SizeBytes, "resolving")
			emit(true, false, "")

			if err := s.downloadGate.Wait(ctx); err != nil {
				return
			}

			tracker.setPhase(job.file.Name, job.index, job.file.SizeBytes, "downloading")
			emit(true, false, "")

			err := s.downloadFile(ctx, job.file, job.destPath, func(bytesDone, bytesTotal int64) {
				tracker.updateActive(job.file, job.index, bytesDone, bytesTotal, "downloading")
				emit(true, false, "")
			})
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				appLog("error", "download", "failed for %q: %v", job.file.Name, err)
				if settings.ContinueOnFileFailure {
					tracker.setPhase(job.file.Name, job.index, job.file.SizeBytes, "failed")
					emit(true, false, "")
					continue
				}
				failOnce.Do(func() {
					firstErr = fmt.Errorf("download failed for %s: %w", job.file.Name, err)
					s.CancelDownload()
				})
				return
			}

			tracker.markDone(job.file, job.index)
			emit(true, false, "")
		}
	}

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go worker()
	}
	wg.Wait()

	if ctx.Err() != nil {
		emit(false, true, "")
		return
	}

	if firstErr != nil {
		emit(false, true, firstErr.Error())
		return
	}

	emit(false, false, "")

	if settings.OpenOutputFolderOnComplete {
		if err := openPath(destRoot); err != nil {
			appLog("warn", "download", "opening output folder: %v", err)
		}
	}
}

type downloadJob struct {
	file     AlbumFile
	index    int
	destPath string
}

func (s *BunkrService) emitProgress(progress DownloadProgress) {
	s.emitDownloadProgress(progress)
}

func (s *BunkrService) downloadFile(ctx context.Context, file AlbumFile, destPath string, onProgress func(int64, int64)) error {
	if cachedPath, ok := s.cachedMediaPath(file.FileID, file.Name, file.SizeBytes); ok {
		appLog("info", "download", "copying cached file %q", file.Name)
		return copyCachedFile(ctx, cachedPath, destPath, file.SizeBytes, onProgress)
	}

	appLog("info", "download", "resolving media URL for %q (fileID=%d)", file.Name, file.FileID)
	mediaURL, err := s.resolveMediaURLWithContext(ctx, file.FileID)
	if err != nil {
		return err
	}

	referer := fmt.Sprintf("%s/file/%d", bunkrDownloadRef, file.FileID)

	response, err := s.doRequestWithRetry(ctx, s.downloadClient, nil, func() (*http.Request, error) {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, mediaURL, nil)
		if reqErr != nil {
			return nil, reqErr
		}
		req.Header.Set("Referer", referer)
		req.Header.Set("Origin", bunkrDownloadRef)
		req.Header.Set("User-Agent", httpUserAgent)
		return req, nil
	})
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("download failed: %s", response.Status)
	}

	total := file.SizeBytes
	if total <= 0 {
		total = response.ContentLength
	}

	tempPath := destPath + ".part"
	out, err := os.Create(tempPath)
	if err != nil {
		return err
	}

	written, err := copyWithProgress(ctx, out, response.Body, total, onProgress)
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

func copyWithProgress(ctx context.Context, dst io.Writer, src io.Reader, total int64, onProgress func(int64, int64)) (int64, error) {
	buf := make([]byte, 32*1024)
	var written int64

	for {
		select {
		case <-ctx.Done():
			return written, ctx.Err()
		default:
		}

		n, readErr := src.Read(buf)
		if n > 0 {
			wn, writeErr := dst.Write(buf[:n])
			written += int64(wn)
			if writeErr != nil {
				return written, writeErr
			}
			if onProgress != nil && (total > 0 || written% (256*1024) < int64(n)) {
				onProgress(written, total)
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				return written, nil
			}
			return written, readErr
		}
	}
}

func (s *BunkrService) emitDownloadProgress(progress DownloadProgress) {
	s.mu.Lock()
	s.downloadProgress = progress
	s.mu.Unlock()

	app := application.Get()
	if app == nil {
		return
	}
	app.Event.Emit(downloadProgressEvent, progress)
}

func filterDownloadFiles(files []AlbumFile, opts DownloadOptions) []AlbumFile {
	typeSet := map[string]bool{}
	for _, t := range opts.Types {
		typeSet[normalizeDownloadType(t)] = true
	}

	patterns := parseIncludePatterns(opts.IncludePatterns)
	filtered := make([]AlbumFile, 0, len(files))
	for _, file := range files {
		fileType := normalizeDownloadType(file.Type)
		if len(typeSet) > 0 && !typeSet[fileType] {
			continue
		}
		if len(patterns) > 0 && !matchesIncludePatterns(file.Name, patterns) {
			continue
		}
		filtered = append(filtered, file)
	}
	return filtered
}

func parseIncludePatterns(raw []string) []string {
	var patterns []string
	for _, entry := range raw {
		for _, part := range strings.Split(entry, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				patterns = append(patterns, part)
			}
		}
	}
	return patterns
}

func matchesIncludePatterns(name string, patterns []string) bool {
	lowerName := strings.ToLower(name)
	for _, pattern := range patterns {
		pattern = strings.ToLower(strings.TrimSpace(pattern))
		if pattern == "" {
			continue
		}
		if ok, _ := filepath.Match(pattern, lowerName); ok {
			return true
		}
		if strings.Contains(lowerName, strings.Trim(pattern, "*")) {
			return true
		}
	}
	return false
}

func normalizeDownloadType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "image":
		return "image"
	case "video":
		return "video"
	case "audio":
		return "audio"
	default:
		return "file"
	}
}

func fileAlbumIndex(files []AlbumFile, target AlbumFile) int {
	for i, file := range files {
		if file.FileID > 0 && file.FileID == target.FileID {
			return i
		}
		if file.Name == target.Name && file.FileURL == target.FileURL {
			return i
		}
	}
	return -1
}

func openPath(path string) error {
	switch runtime.GOOS {
	case "linux", "freebsd", "netbsd", "openbsd":
		return exec.Command("xdg-open", path).Start()
	case "darwin":
		return exec.Command("open", path).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", path).Start()
	default:
		return fmt.Errorf("unsupported platform")
	}
}
