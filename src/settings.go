package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const settingsAppName = "bunkrdownload"

const (
	defaultPageSize           = 25
	minPageSize               = 10
	maxPageSize               = 100
	defaultMaxAlbumHistory    = 1000
	minMaxAlbumHistory        = 50
	maxMaxAlbumHistoryLimit   = 5000
	defaultMaxHTTPRetries     = 5
	minMaxHTTPRetries         = 0
	maxMaxHTTPRetries         = 10
	defaultParallelDownloads  = 1
	minParallelDownloads      = 1
	maxParallelDownloads      = 8
)

type AppSettings struct {
	OutputFolder               string   `json:"outputFolder"`
	FilterTypes                []string `json:"filterTypes"`
	IncludePatterns            string   `json:"includePatterns"`
	PaginationMode             string   `json:"paginationMode"`
	ViewMode                   string   `json:"viewMode"`
	OpenOutputFolderOnComplete bool     `json:"openOutputFolderOnComplete"`
	PageSize                   int      `json:"pageSize"`
	MaxAlbumHistory            int      `json:"maxAlbumHistory"`
	SkipExistingFiles          bool     `json:"skipExistingFiles"`
	CreateAlbumSubfolder       bool     `json:"createAlbumSubfolder"`
	ContinueOnFileFailure      bool     `json:"continueOnFileFailure"`
	MaxHTTPRetries             int      `json:"maxHttpRetries"`
	ParallelDownloads          int      `json:"parallelDownloads"`
}

func defaultAppSettings() AppSettings {
	return AppSettings{
		FilterTypes:          []string{"Image", "Video", "Audio", "File"},
		PaginationMode:       "pagination",
		ViewMode:             "list",
		PageSize:             defaultPageSize,
		MaxAlbumHistory:      defaultMaxAlbumHistory,
		SkipExistingFiles:      true,
		CreateAlbumSubfolder:   true,
		MaxHTTPRetries:         defaultMaxHTTPRetries,
		ParallelDownloads:      defaultParallelDownloads,
	}
}

func normalizePaginationMode(value string) string {
	if value == "infinite-scroll" {
		return "infinite-scroll"
	}
	return "pagination"
}

func normalizeViewMode(value string) string {
	if value == "gallery" {
		return "gallery"
	}
	return "list"
}

func normalizeAppSettings(settings AppSettings) AppSettings {
	if len(settings.FilterTypes) == 0 {
		settings.FilterTypes = defaultAppSettings().FilterTypes
	}
	settings.PaginationMode = normalizePaginationMode(settings.PaginationMode)
	settings.ViewMode = normalizeViewMode(settings.ViewMode)
	if settings.PageSize <= 0 {
		settings.PageSize = defaultPageSize
	} else if settings.PageSize > maxPageSize {
		settings.PageSize = maxPageSize
	} else if settings.PageSize < minPageSize {
		settings.PageSize = minPageSize
	}
	if settings.MaxAlbumHistory <= 0 {
		settings.MaxAlbumHistory = defaultMaxAlbumHistory
	} else if settings.MaxAlbumHistory > maxMaxAlbumHistoryLimit {
		settings.MaxAlbumHistory = maxMaxAlbumHistoryLimit
	} else if settings.MaxAlbumHistory < minMaxAlbumHistory {
		settings.MaxAlbumHistory = minMaxAlbumHistory
	}
	if settings.MaxHTTPRetries < minMaxHTTPRetries {
		settings.MaxHTTPRetries = minMaxHTTPRetries
	} else if settings.MaxHTTPRetries > maxMaxHTTPRetries {
		settings.MaxHTTPRetries = maxMaxHTTPRetries
	}
	if settings.ParallelDownloads <= 0 {
		settings.ParallelDownloads = defaultParallelDownloads
	} else if settings.ParallelDownloads > maxParallelDownloads {
		settings.ParallelDownloads = maxParallelDownloads
	}
	return settings
}

func settingsFilePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locating config directory: %w", err)
	}
	return filepath.Join(configDir, settingsAppName, "settings.json"), nil
}

func loadAppSettings() (AppSettings, error) {
	settings := defaultAppSettings()
	path, err := settingsFilePath()
	if err != nil {
		return settings, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return settings, nil
		}
		return settings, fmt.Errorf("reading settings: %w", err)
	}

	if err := json.Unmarshal(data, &settings); err != nil {
		return defaultAppSettings(), fmt.Errorf("parsing settings: %w", err)
	}
	return normalizeAppSettings(settings), nil
}

func saveAppSettings(settings AppSettings) error {
	path, err := settingsFilePath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating settings directory: %w", err)
	}

	settings = normalizeAppSettings(settings)
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding settings: %w", err)
	}

	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0o644); err != nil {
		return fmt.Errorf("writing settings: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("saving settings: %w", err)
	}
	return nil
}

func (s *BunkrService) GetSettings() AppSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings
}

func (s *BunkrService) maxHTTPRetries() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.settings.MaxHTTPRetries < minMaxHTTPRetries {
		return defaultMaxHTTPRetries
	}
	if s.settings.MaxHTTPRetries > maxMaxHTTPRetries {
		return maxMaxHTTPRetries
	}
	return s.settings.MaxHTTPRetries
}

func (s *BunkrService) maxAlbumHistoryLimit() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.settings.MaxAlbumHistory <= 0 {
		return defaultMaxAlbumHistory
	}
	return s.settings.MaxAlbumHistory
}

func (s *BunkrService) SaveSettings(settings AppSettings) error {
	settings.OutputFolder = strings.TrimSpace(settings.OutputFolder)
	settings.IncludePatterns = strings.TrimSpace(settings.IncludePatterns)
	settings = normalizeAppSettings(settings)

	s.mu.Lock()
	s.settings = settings
	s.outputFolder = settings.OutputFolder
	s.mu.Unlock()

	if err := trimAlbumHistoryToLimit(settings.MaxAlbumHistory); err != nil {
		return err
	}

	return saveAppSettings(settings)
}

func (s *BunkrService) SetOutputFolder(path string) error {
	path = strings.TrimSpace(path)

	s.mu.Lock()
	s.settings.OutputFolder = path
	s.outputFolder = path
	settings := s.settings
	s.mu.Unlock()

	return saveAppSettings(settings)
}

func (s *BunkrService) applyLoadedSettings(settings AppSettings) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings = normalizeAppSettings(settings)
	s.outputFolder = s.settings.OutputFolder
}

func trimAlbumHistoryToLimit(limit int) error {
	if limit <= 0 {
		limit = defaultMaxAlbumHistory
	}
	history, err := loadAlbumHistory()
	if err != nil {
		return err
	}
	if len(history) <= limit {
		return nil
	}
	return saveAlbumHistory(history[:limit])
}
