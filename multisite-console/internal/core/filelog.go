package core

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// DiskLog appends one plain-text line per event to data/logs.txt.
//
// Why a file at all: a desktop app with no console must still leave a trace of a
// crash or a failed run. The ring buffer is for the UI (recent history, cursor
// polling); this file is for the user who says "it stopped working last night".
//
// Secrets are redacted before anything is written (see secretPatterns).
type DiskLog struct {
	mu   sync.Mutex
	path string
	file *os.File
}

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(api[_-]?key["'\s:=]+)([A-Za-z0-9\-_\.]{8,})`),
	regexp.MustCompile(`(?i)(access[_-]?token["'\s:=]+)([A-Za-z0-9\-_\.]{8,})`),
	regexp.MustCompile(`(?i)(\btoken["'\s:=]+)([A-Za-z0-9\-_\.]{8,})`),
	regexp.MustCompile(`(?i)(password["'\s:=]+)([^\s"',}]+)`),
	regexp.MustCompile(`(?i)(bearer\s+)([A-Za-z0-9\-_\.]{8,})`),
	regexp.MustCompile(`(?i)(://[^:/\s]+:)([^@\s]+)(@)`),
}

// Redact removes anything that looks like a credential.
func Redact(text string) string {
	out := text
	for _, pattern := range secretPatterns {
		out = pattern.ReplaceAllString(out, "${1}[redacted]")
	}
	return out
}

// OpenDiskLog creates (or appends to) data/logs.txt.
func OpenDiskLog(dataDir string) (*DiskLog, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	path := filepath.Join(dataDir, "logs.txt")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}
	return &DiskLog{path: path, file: file}, nil
}

// Path is the log file location (shown in the UI).
func (d *DiskLog) Path() string { return d.path }

// Write appends one formatted line. A failed write must never break the caller.
func (d *DiskLog) Write(source, level, message string) {
	if d == nil || d.file == nil {
		return
	}
	line := fmt.Sprintf("%s | %-6s | %-6s | %s\n",
		time.Now().Format("2006-01-02 15:04:05"), source, strings.ToUpper(level), Redact(message))
	d.mu.Lock()
	_, _ = d.file.WriteString(line)
	d.mu.Unlock()
}

// Close flushes and closes the file (safe to call twice).
func (d *DiskLog) Close() error {
	if d == nil || d.file == nil {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	err := d.file.Close()
	d.file = nil
	return err
}
