package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultAppSettings(t *testing.T) {
	settings := defaultAppSettings()
	if len(settings.FilterTypes) != 4 {
		t.Fatalf("expected 4 default filter types, got %d", len(settings.FilterTypes))
	}
}

func TestSaveAndLoadAppSettings(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)

	settings := AppSettings{
		OutputFolder:    "/tmp/downloads",
		FilterTypes:     []string{"Image", "Video"},
		IncludePatterns: "*.jpg, *set*",
	}
	if err := saveAppSettings(settings); err != nil {
		t.Fatalf("saveAppSettings failed: %v", err)
	}

	loaded, err := loadAppSettings()
	if err != nil {
		t.Fatalf("loadAppSettings failed: %v", err)
	}
	if loaded.OutputFolder != settings.OutputFolder {
		t.Fatalf("unexpected output folder: %q", loaded.OutputFolder)
	}
	if len(loaded.FilterTypes) != 2 {
		t.Fatalf("unexpected filter types: %#v", loaded.FilterTypes)
	}
	if loaded.IncludePatterns != settings.IncludePatterns {
		t.Fatalf("unexpected include patterns: %q", loaded.IncludePatterns)
	}

	path := filepath.Join(configDir, settingsAppName, "settings.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("settings file not written: %v", err)
	}
}

func TestBunkrServiceSaveSettings(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)

	service := NewBunkrService()
	settings := AppSettings{
		OutputFolder:    "/home/user/bunkr",
		FilterTypes:     []string{"Image"},
		IncludePatterns: "*.png",
	}
	if err := service.SaveSettings(settings); err != nil {
		t.Fatalf("SaveSettings failed: %v", err)
	}
	if got := service.GetOutputFolder(); got != "/home/user/bunkr" {
		t.Fatalf("unexpected output folder: %q", got)
	}
	if got := service.GetSettings(); got.IncludePatterns != "*.png" {
		t.Fatalf("unexpected settings: %#v", got)
	}
}
