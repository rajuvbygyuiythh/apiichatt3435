// =============================================================================
//  app-skeleton.go — starting point for the Go side of a derived dashboard
// -----------------------------------------------------------------------------
//  Drop this next to main.go (package main), then:
//    1. rename App methods' bodies to your domain
//    2. add internal/model structs + internal/uistore storage (see reference/go/)
//    3. wails.Run(&options.App{ ... Bind: []interface{}{app} })
//
//  Every exported method on App becomes callable from JS as
//  window.go.main.App.Method(...). Keep the signatures from 08-API-CONTRACT.md
//  so the starter frontend works without a single change.
// =============================================================================

package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the only type bound to the frontend.
type App struct {
	ctx     context.Context // Wails runtime context (needed for events)
	dataDir string

	mu      sync.Mutex // guards everything below
	running bool
	cancel  context.CancelFunc

	logs    []LogEntry // in-memory ring; replace with Badger persistence
	logSeq  int
	logLock sync.Mutex
}

// LogEntry matches the frontend's expected shape (08-API-CONTRACT.md).
type LogEntry struct {
	ID   int    `json:"id"`
	Time string `json:"time"`
	Kind string `json:"kind"` // in | out | skip | system | error
	Text string `json:"text"`
	Meta string `json:"meta,omitempty"`
}

// Settings is your snapshot payload. Replace the fields; keep the shape:
// one object per section, so SaveSettings can merge by section name.
type Settings struct {
	Setup       map[string]any `json:"setup"`
	Connections map[string]any `json:"connections"`
	Credentials map[string]any `json:"credentials"`
	Actions     map[string]any `json:"actions"`
	Timings     map[string]any `json:"timings"`
	Profile     map[string]any `json:"profile"`
}

// Snapshot is what GetSnapshot returns and what every mutating method returns.
type Snapshot struct {
	Settings Settings         `json:"settings"`
	Strategy string           `json:"strategy"`
	Stats    map[string]int   `json:"stats"`
	Items    []map[string]any `json:"items"`
	Runtime  map[string]any   `json:"runtime"`
}

type ConnResult struct {
	Entry     string `json:"entry"`
	OK        bool   `json:"ok"`
	LatencyMS int    `json:"latency_ms"`
	Message   string `json:"message"`
}

func NewApp(dataDir string) (*App, error) {
	return &App{dataDir: dataDir}, nil
}

// --- storage stubs (replace with internal/uistore) ---------------------------

var settingsCache = Settings{
	Setup:       map[string]any{"name": "", "mode": "balanced", "concurrency": 2, "timeout_seconds": 60, "headless": false, "notes": ""},
	Connections: map[string]any{"enabled": false, "scheme": "http", "entries": ""},
	Credentials: map[string]any{"base_url": "", "api_key": "", "model": "", "temperature": 0.8},
	Actions:     map[string]any{"strategy": "single", "sequence": "", "targets": ""},
	Timings:     map[string]any{"poll_min": 5, "poll_max": 12, "action_min": 3, "action_max": 8, "timeout_min": 60, "timeout_max": 90, "retry_min": 10, "retry_max": 30},
	Profile:     map[string]any{"name": "default", "backend": "rod", "headless": false},
}

func SettingsDefaults() Settings { return settingsCache }

// --- lifecycle --------------------------------------------------------------

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.emit("ui:running", false)
	a.log("system", "app started")
}

func (a *App) shutdown(_ context.Context) {
	_ = a.StopRun()
}

// --- contract methods -------------------------------------------------------

func (a *App) GetSnapshot() (Snapshot, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return Snapshot{
		Settings: settingsCache,
		Strategy: fmt.Sprint(settingsCache.Actions["strategy"]),
		Stats:    map[string]int{"profiles": 2, "completed": 0, "queued": 0, "failed": 0},
		Items:    []map[string]any{},
		Runtime:  map[string]any{"running": a.running, "kind": "wails", "data_dir": a.dataDir},
	}, nil
}

// SaveSettings merges one section and returns the fresh snapshot.
func (a *App) SaveSettings(section string, values map[string]any) (Snapshot, error) {
	a.mu.Lock()
	switch section {
	case "setup":
		for k, v := range values {
			settingsCache.Setup[k] = v
		}
	case "connections":
		for k, v := range values {
			settingsCache.Connections[k] = v
		}
	case "credentials":
		for k, v := range values {
			settingsCache.Credentials[k] = v
		}
	case "actions":
		for k, v := range values {
			settingsCache.Actions[k] = v
		}
	case "timings":
		for k, v := range values {
			settingsCache.Timings[k] = v
		}
	case "profile":
		for k, v := range values {
			settingsCache.Profile[k] = v
		}
	default:
		a.mu.Unlock()
		return Snapshot{}, fmt.Errorf("unknown settings section %q", section)
	}
	a.mu.Unlock()

	// TODO: persist (uistore.SaveSettings) + write the JSON mirror here.
	return a.GetSnapshot()
}

func (a *App) TestConnection(conn map[string]any) (map[string][]ConnResult, error) {
	entries := splitLines(fmt.Sprint(conn["entries"]))
	results := make([]ConnResult, 0, len(entries))
	for _, entry := range entries {
		start := time.Now()
		// TODO: dial / ping / request. This stub reports success.
		results = append(results, ConnResult{
			Entry: entry, OK: true,
			LatencyMS: int(time.Since(start).Milliseconds()),
			Message:   "ok",
		})
	}
	return map[string][]ConnResult{"results": results}, nil
}

func (a *App) TestAPI(creds map[string]any) (string, error) {
	if fmt.Sprint(creds["base_url"]) == "" {
		return "", errors.New("base URL is empty")
	}
	// TODO: one real call with context + timeout (see reference/go/openai-client.go).
	return "200 OK · stub response · 0 tokens", nil
}

func (a *App) StartRun(payload map[string]any) (map[string]any, error) {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return nil, errors.New("already running")
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.running, a.cancel = true, cancel
	a.mu.Unlock()

	a.emit("ui:running", true)
	go a.work(ctx, fmt.Sprint(payload["profile"]))
	return map[string]any{"ok": true, "message": "started"}, nil
}

func (a *App) StopRun() error {
	a.mu.Lock()
	cancel := a.cancel
	a.cancel, a.running = nil, false
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	a.emit("ui:running", false)
	a.log("system", "run stopped by user")
	return nil
}

func (a *App) LaunchLogin(payload map[string]any) (map[string]any, error) {
	// TODO: open a browser for a manual login, then save the session.
	a.log("system", fmt.Sprintf("login requested for profile %v", payload["profile"]))
	return map[string]any{"ok": true, "message": "login requested"}, nil
}

func (a *App) GetLogs(after int) (map[string]any, error) {
	a.logLock.Lock()
	defer a.logLock.Unlock()
	out := make([]LogEntry, 0, len(a.logs))
	for _, entry := range a.logs {
		if entry.ID > after {
			out = append(out, entry)
		}
	}
	return map[string]any{"version": a.logSeq, "entries": out}, nil
}

func (a *App) ClearLogs() (map[string]any, error) {
	a.logLock.Lock()
	a.logs = nil
	a.logLock.Unlock()
	return map[string]any{"ok": true}, nil
}

func (a *App) OpenDataFolder() (string, error) {
	// TODO: os.StartProcess("explorer", ...) / xdg-open / open
	return filepath.Join(a.dataDir, "data"), nil
}

func (a *App) LoadFolder() (map[string]any, error) {
	// TODO: read *.txt from data/<folder>/
	return map[string]any{"count": 0, "names": []string{}}, nil
}

// --- internals --------------------------------------------------------------

// work is your job. Log every meaningful step; the UI renders it live.
func (a *App) work(ctx context.Context, profile string) {
	defer func() {
		a.mu.Lock()
		a.running = false
		a.mu.Unlock()
		a.emit("ui:running", false)
		a.log("system", "run finished")
	}()

	a.log("system", "run started (profile: "+profile+")")
	ticker := time.NewTicker(1500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.log("out", "stub step executed")
		}
	}
}

func splitLines(raw string) []string {
	out := make([]string, 0, 8)
	for _, line := range strings.Split(raw, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func (a *App) emit(name string, data any) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, name, data)
}

// log persists to disk first, then appends for the UI poller, then emits.
func (a *App) log(kind, message string) {
	a.logLock.Lock()
	a.logSeq++
	entry := LogEntry{ID: a.logSeq, Time: time.Now().Format("15:04:05"), Kind: kind, Text: message}
	a.logs = append(a.logs, entry)
	if len(a.logs) > 2000 {
		a.logs = a.logs[len(a.logs)-2000:]
	}
	a.logLock.Unlock()
	a.emit("ui:log", entry)
}
