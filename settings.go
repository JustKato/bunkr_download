package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const settingsAppName = "bunkrdownload"

type AppSettings struct {
	OutputFolder    string   `json:"outputFolder"`
	FilterTypes     []string `json:"filterTypes"`
	IncludePatterns string   `json:"includePatterns"`
	PaginationMode  string   `json:"paginationMode"`
	ViewMode        string   `json:"viewMode"`
}

func defaultAppSettings() AppSettings {
	return AppSettings{
		FilterTypes:    []string{"Image", "Video", "Audio", "File"},
		PaginationMode: "pagination",
		ViewMode:       "list",
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
	if len(settings.FilterTypes) == 0 {
		settings.FilterTypes = defaultAppSettings().FilterTypes
	}
	settings.PaginationMode = normalizePaginationMode(settings.PaginationMode)
	settings.ViewMode = normalizeViewMode(settings.ViewMode)
	return settings, nil
}

func saveAppSettings(settings AppSettings) error {
	path, err := settingsFilePath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating settings directory: %w", err)
	}

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

func (s *BunkrService) SaveSettings(settings AppSettings) error {
	settings.OutputFolder = strings.TrimSpace(settings.OutputFolder)
	settings.IncludePatterns = strings.TrimSpace(settings.IncludePatterns)
	if len(settings.FilterTypes) == 0 {
		settings.FilterTypes = defaultAppSettings().FilterTypes
	}
	settings.PaginationMode = normalizePaginationMode(settings.PaginationMode)
	settings.ViewMode = normalizeViewMode(settings.ViewMode)

	s.mu.Lock()
	s.settings = settings
	s.outputFolder = settings.OutputFolder
	s.mu.Unlock()

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
	s.settings = settings
	s.outputFolder = settings.OutputFolder
}
