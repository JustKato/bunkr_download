package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const maxAlbumHistory = 1000

type AlbumHistoryEntry struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

func albumHistoryFilePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locating config directory: %w", err)
	}
	return filepath.Join(configDir, settingsAppName, "album_history.json"), nil
}

func loadAlbumHistory() ([]AlbumHistoryEntry, error) {
	path, err := albumHistoryFilePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading album history: %w", err)
	}

	var history []AlbumHistoryEntry
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, fmt.Errorf("parsing album history: %w", err)
	}
	return history, nil
}

func saveAlbumHistory(history []AlbumHistoryEntry) error {
	path, err := albumHistoryFilePath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating history directory: %w", err)
	}

	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding album history: %w", err)
	}

	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0o644); err != nil {
		return fmt.Errorf("writing album history: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("saving album history: %w", err)
	}
	return nil
}

func extractAlbumID(albumURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(albumURL))
	if err != nil {
		return ""
	}
	path := strings.TrimSuffix(parsed.EscapedPath(), "/")
	if !strings.HasPrefix(path, "/a/") {
		return ""
	}
	return strings.TrimPrefix(path, "/a/")
}

func recordAlbumHistory(albumURL, title string) error {
	id := extractAlbumID(albumURL)
	if id == "" {
		return nil
	}

	entry := AlbumHistoryEntry{
		ID:    id,
		Title: strings.TrimSpace(title),
		URL:   strings.TrimSpace(albumURL),
	}
	if entry.Title == "" {
		entry.Title = "Untitled Bunkr album"
	}

	history, err := loadAlbumHistory()
	if err != nil {
		return err
	}

	filtered := history[:0]
	for _, existing := range history {
		if existing.ID != entry.ID {
			filtered = append(filtered, existing)
		}
	}
	history = append([]AlbumHistoryEntry{entry}, filtered...)
	if len(history) > maxAlbumHistory {
		history = history[:maxAlbumHistory]
	}
	return saveAlbumHistory(history)
}

func (s *BunkrService) GetAlbumHistory() []AlbumHistoryEntry {
	history, err := loadAlbumHistory()
	if err != nil {
		return nil
	}
	return history
}
