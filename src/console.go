package main

import (
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	consoleLogEvent   = "console:log"
	maxConsoleEntries = 2000
)

type ConsoleEntry struct {
	Time    int64  `json:"time"`
	Level   string `json:"level"`
	Source  string `json:"source"`
	Message string `json:"message"`
}

type consoleBuffer struct {
	mu      sync.RWMutex
	entries []ConsoleEntry
}

var appConsole = &consoleBuffer{
	entries: make([]ConsoleEntry, 0, 256),
}

func init() {
	log.SetOutput(appConsole)
	log.SetFlags(log.LstdFlags)
}

func (b *consoleBuffer) Write(p []byte) (int, error) {
	message := strings.TrimSpace(string(p))
	if message != "" {
		appLog("info", "go", "%s", message)
	}
	return len(p), nil
}

func appLog(level, source, format string, args ...any) {
	entry := ConsoleEntry{
		Time:    time.Now().UnixMilli(),
		Level:   normalizeConsoleLevel(level),
		Source:  source,
		Message: fmt.Sprintf(format, args...),
	}

	appConsole.mu.Lock()
	appConsole.entries = append(appConsole.entries, entry)
	if len(appConsole.entries) > maxConsoleEntries {
		appConsole.entries = append([]ConsoleEntry(nil), appConsole.entries[len(appConsole.entries)-maxConsoleEntries:]...)
	}
	appConsole.mu.Unlock()

	app := application.Get()
	if app != nil {
		app.Event.Emit(consoleLogEvent, entry)
	}
}

func normalizeConsoleLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug", "info", "warn", "warning", "error":
		if level == "warning" {
			return "warn"
		}
		return strings.ToLower(level)
	default:
		return "info"
	}
}

func (s *BunkrService) AppendConsoleLog(level, source, message string) {
	appLog(level, source, "%s", message)
}

func (s *BunkrService) GetConsoleLogs() []ConsoleEntry {
	appConsole.mu.RLock()
	defer appConsole.mu.RUnlock()
	out := make([]ConsoleEntry, len(appConsole.entries))
	copy(out, appConsole.entries)
	return out
}

func (s *BunkrService) ClearConsoleLogs() {
	appConsole.mu.Lock()
	appConsole.entries = appConsole.entries[:0]
	appConsole.mu.Unlock()
	appLog("info", "console", "console cleared")
}

func newAssetHandler(frontend fs.FS) http.Handler {
	assets := application.AssetFileServerFS(frontend)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, mediaCacheURLPrefix) {
			serveMediaCache(w, r)
			return
		}
		assets.ServeHTTP(w, r)
	})
}
