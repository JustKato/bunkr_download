//go:build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	linuxIconThemeName = "com.danlegt.bunkrdownload"
	linuxDisplayName   = "Bunkr Downloader"
)

var (
	invalidLinuxAppNameChars = regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	leadingLinuxDigits       = regexp.MustCompile(`^[0-9]+`)
)

func installLinuxDesktopIntegration() {
	if os.Getenv("BUNKRDOWNLOAD_SKIP_DESKTOP_ICON") != "" {
		return
	}
	if len(appIcon) == 0 {
		return
	}

	execPath, err := os.Executable()
	if err != nil {
		return
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return
	}

	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return
		}
		dataHome = filepath.Join(home, ".local", "share")
	}

	iconChanged := installLinuxIconTheme(dataHome)
	if err := writeLinuxDesktopEntry(dataHome, execPath); err != nil {
		return
	}
	if iconChanged {
		updateLinuxIconCache(filepath.Join(dataHome, "icons", "hicolor"))
	}
}

func installLinuxIconTheme(dataHome string) bool {
	sizes := []int{32, 48, 64, 128, 256}
	changed := false

	for _, size := range sizes {
		dir := filepath.Join(dataHome, "icons", "hicolor",
			fmt.Sprintf("%dx%d", size, size), "apps")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			continue
		}

		iconPath := filepath.Join(dir, linuxIconThemeName+".png")
		if existing, err := os.ReadFile(iconPath); err == nil && string(existing) == string(appIcon) {
			continue
		}
		if err := os.WriteFile(iconPath, appIcon, 0o644); err != nil {
			continue
		}
		changed = true
	}

	return changed
}

func writeLinuxDesktopEntry(dataHome, execPath string) error {
	appDir := filepath.Join(dataHome, "applications")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return err
	}

	startupClass := wailsGTKApplicationID(linuxDisplayName)
	desktopPath := filepath.Join(appDir, startupClass+".desktop")
	content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Version=1.0
Name=%s
Comment=Bunkr album downloader
Exec=%q
Icon=%s
Terminal=false
Categories=Network;Utility;
StartupNotify=true
StartupWMClass=%s
X-GNOME-Startup-Class=%s
X-KDE-StartupClass=%s
`, linuxDisplayName, execPath, linuxIconThemeName, startupClass, startupClass, startupClass)

	if existing, err := os.ReadFile(desktopPath); err == nil && string(existing) == content {
		return nil
	}

	return os.WriteFile(desktopPath, []byte(content), 0o644)
}

func updateLinuxIconCache(iconThemeDir string) {
	for _, cmd := range []string{"gtk4-update-icon-cache", "gtk-update-icon-cache"} {
		if path, err := exec.LookPath(cmd); err == nil {
			_ = exec.Command(path, "-f", iconThemeDir).Run()
			return
		}
	}
}

func wailsGTKApplicationID(name string) string {
	sanitized := sanitizeLinuxAppName(name)
	return "org.wails." + sanitized
}

func sanitizeLinuxAppName(name string) string {
	name = invalidLinuxAppNameChars.ReplaceAllString(name, "_")
	name = leadingLinuxDigits.ReplaceAllString(name, "_$0")
	for strings.Contains(name, "__") {
		name = strings.ReplaceAll(name, "__", "_")
	}
	name = strings.Trim(name, "_")
	if name == "" {
		name = "wailsapp"
	}
	return strings.ToLower(name)
}
