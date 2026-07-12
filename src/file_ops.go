package main

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
)

type FileDetails struct {
	Index         int    `json:"index"`
	FileID        int64  `json:"fileID"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	MimeType      string `json:"mimeType"`
	Size          string `json:"size"`
	SizeBytes     int64  `json:"sizeBytes"`
	Date          string `json:"date"`
	PreviewURL    string `json:"previewURL"`
	FileURL       string `json:"fileURL"`
	MediaURL      string `json:"mediaURL"`
	MediaURLError string `json:"mediaURLError"`
	OnDisk        bool   `json:"onDisk"`
	DiskPath      string `json:"diskPath"`
}

func (s *BunkrService) albumFileAt(index int) (AlbumFile, *Album, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.activeAlbum == nil {
		return AlbumFile{}, nil, fmt.Errorf("no album loaded")
	}
	if index < 0 || index >= len(s.activeAlbum.Files) {
		return AlbumFile{}, nil, fmt.Errorf("file index out of range")
	}
	copyAlbum := *s.activeAlbum
	return s.activeAlbum.Files[index], &copyAlbum, nil
}

func (s *BunkrService) GetDownloadedFileIndices() ([]int, error) {
	s.mu.RLock()
	album := s.activeAlbum
	outputFolder := s.outputFolder
	s.mu.RUnlock()

	if album == nil || outputFolder == "" {
		return nil, nil
	}
	if _, err := os.Stat(outputFolder); err != nil {
		return nil, nil
	}

	indices := make([]int, 0)
	for i, file := range album.Files {
		destPath := downloadDestPath(outputFolder, album.Title, file)
		if _, err := os.Stat(destPath); err == nil {
			indices = append(indices, i)
		}
	}
	return indices, nil
}

func (s *BunkrService) GetFileDetails(index int) (*FileDetails, error) {
	file, album, err := s.albumFileAt(index)
	if err != nil {
		return nil, err
	}

	details := &FileDetails{
		Index:      index,
		FileID:     file.FileID,
		Name:       file.Name,
		Type:       file.Type,
		MimeType:   file.MimeType,
		Size:       file.Size,
		SizeBytes:  file.SizeBytes,
		Date:       file.Date,
		PreviewURL: file.PreviewURL,
		FileURL:    file.FileURL,
	}

	s.mu.RLock()
	outputFolder := s.outputFolder
	s.mu.RUnlock()
	if outputFolder != "" {
		destPath := downloadDestPath(outputFolder, album.Title, file)
		if _, err := os.Stat(destPath); err == nil {
			details.OnDisk = true
			details.DiskPath = destPath
		}
	}

	if file.FileID > 0 {
		mediaURL, err := s.ResolveMediaURL(file.FileID)
		if err != nil {
			details.MediaURLError = err.Error()
		} else {
			details.MediaURL = mediaURL
		}
	}

	return details, nil
}

func (s *BunkrService) DownloadFileAtIndex(index int) error {
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

	file, album, err := s.albumFileAt(index)
	if err != nil {
		return err
	}

	s.mu.RLock()
	outputFolder := s.outputFolder
	s.mu.RUnlock()
	if outputFolder == "" {
		return fmt.Errorf("select an output folder first")
	}
	if _, err := os.Stat(outputFolder); err != nil {
		return fmt.Errorf("output folder not found: %s", outputFolder)
	}

	appLog("info", "download", "starting single file download: %q", file.Name)

	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.downloadCancel = cancel
	s.mu.Unlock()

	s.emitDownloadProgress(DownloadProgress{
		Running:        true,
		CompletedCount: 0,
		TotalCount:     1,
		FileStatus:     "starting",
	})

	go s.runDownload(ctx, album, outputFolder, []AlbumFile{file})
	return nil
}
