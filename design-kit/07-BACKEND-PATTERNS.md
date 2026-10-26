# 07 · BACKEND PATTERNS (Go + Wails)

The Go side is deliberately boring: one `App` struct whose **exported methods are the API**,
one store, one logger, one event emitter. Everything else is domain code behind it.

Files to read in `reference/go/`:
`chitchatui-main.go` (wails.Run wiring) · `apichat-main.go` (dual Wails/HTTP app + embed) ·
`model-ui.go` (settings structs) · `uistore-store.go` (Badger persistence) ·
`openai-client.go` (model call) · `wails.json`, `package.json`, `*.bat`.

---

## 1. main(): wire the window, nothing else

```go
//go:embed all:cmd/chitchatui/frontend/dist
var embeddedAssets embed.FS

func main() {
    dataDir, err := resolveDataDir()          // always inside the project copy
    if err != nil { log.Fatal(err) }

    diskLogger := applog.Logger{Path: applog.Path(dataDir), Source: "wails"}
    log.SetOutput(io.MultiWriter(diskLogger, os.Stderr))
    defer diskLogger.Info("DESKTOP STOP | main returned")

    dist, err := fs.Sub(embeddedAssets, "cmd/chitchatui/frontend/dist")
    if err != nil { log.Fatal(err) }

    app, err := NewApp(dataDir)
    if err != nil { log.Fatal(err) }

    webviewDir := filepath.Join(dataDir, "webview")     // per-copy profile (see §6)
    _ = os.MkdirAll(webviewDir, 0o755)

    err = wails.Run(&options.App{
        Title:            "ChitChat Bot",
        Width:            1280, Height: 820,
        MinWidth:         1080, MinHeight: 700,
        BackgroundColour: &options.RGBA{R: 22, G: 23, B: 22, A: 255}, // == --bg
        AssetServer:      &assetserver.Options{Assets: dist},
        OnStartup:        app.startup,
        OnShutdown:       app.shutdown,
        Bind:             []interface{}{app},                        // every method becomes JS-callable
        Windows: &windows.Options{
            WebviewUserDataPath: webviewDir,
            ResizeDebounceMS:    10,
        },
    })
    if err != nil { log.Fatal(err) }
}
```

Rules: `BackgroundColour` must equal the CSS `--bg` or the window flashes white on launch;
`Bind` exposes only the `App` struct — never bind stores or domain types; window title is
the product name, not the mode.

## 2. The App struct (state + locks)

```go
type App struct {
    ctx       context.Context     // Wails runtime context (events need it)
    dataDir   string
    uiStore   *uistore.Store
    mu        sync.Mutex          // guards everything below
    running   bool
    runID     uint64
    botCancel context.CancelFunc
    activeWorkers map[string]bool
}

func NewApp(dataDir string) (*App, error) {
    uiStore, err := uistore.Open(dataDir)
    if err != nil { return nil, err }
    app := &App{dataDir: dataDir, uiStore: uiStore, activeWorkers: map[string]bool{}}
    if err := app.seedDefaults(); err != nil { _ = uiStore.Close(); return nil, err }
    return app, nil
}

func (a *App) startup(ctx context.Context) {
    a.ctx = ctx
    a.emit("ui:running", false)
}

func (a *App) shutdown(_ context.Context) { a.StopBot(); _ = a.uiStore.Close() }
```

Every exported method: `a.mu.Lock(); defer a.mu.Unlock()` → validate → call domain →
persist → emit. Never let a long-running job hold the lock (start it in a goroutine that
takes its own snapshot of the config).

## 3. Settings methods: load → merge → normalise → save → return

```go
func (a *App) SaveProxySettings(s model.ProxySettings) error {
    if s.Scheme != "http" && s.Scheme != "socks5" { s.Scheme = "http" }   // normalise
    return a.uiStore.SaveProxySettings(s)
}

func (a *App) GetInitialState() (model.UISnapshot, error) {   // one snapshot, one call
    return model.UISnapshot{
        Running: a.running,
        Home:    a.homeStats(),
        Browser: browser, Message: message, Proxy: proxy, General: general,
        BrowserOptions: model.DefaultBrowserOptions(),
    }, nil
}
```

Prefer **one** `GetInitialState`/`GetSnapshot` over a dozen getters: one round trip, one
atomic view, far fewer race conditions.

## 4. Persistence (Badger, JSON docs)

```go
const (
    keyBrowser = "settings/browser"
    keyMessage = "settings/message"
    keyProxy   = "settings/proxy"
    keyGeneral = "settings/general"
    logPrefix  = "logs/"
)

func Open(baseDir string) (*Store, error) {
    dbDir := filepath.Join(baseDir, "ui", "badger")
    if err := store.EnsureDir(dbDir); err != nil { return nil, err }
    opts := badger.DefaultOptions(dbDir)
    opts.Logger = nil                    // no Badger chatter in our log
    opts.ValueLogFileSize = 64 << 20
    db, err := badger.Open(opts)
    if err != nil { return nil, err }
    return &Store{baseDir: baseDir, db: db}, nil
}

func (s *Store) LoadProxySettings() (model.ProxySettings, error) {
    settings := model.DefaultProxySettings()
    ok, err := s.loadJSON(keyProxy, &settings)
    if err != nil || !ok { return settings, err }   // defaults, never a nil struct
    if settings.Scheme != "http" && settings.Scheme != "socks5" { settings.Scheme = "http" }
    return settings, nil
}
```

- `loadJSON(key, &out)` returns `(found, error)`; "not found" is **not** an error.
- Structs have `Default*()` constructors — the UI can rely on every field being present.
- A JSON **mirror** file (`message_settings.json`) is written next to the DB so a second
  process/window can pick up changes; on load, newest wins.
- Logs are keyed `logs/<16-digit id>` so a cursor range scan gives you "everything after N".

## 5. Events + dual logging

```go
func (a *App) emit(name string, data any) {
    if a.ctx == nil { return }                    // pre-startup calls are no-ops
    runtime.EventsEmit(a.ctx, name, data)
}

func (a *App) log(level, message string) {
    disk := applog.Logger{Path: applog.Path(a.dataDir), Source: "app"}
    disk.Log(level, message)                      // 1. always on disk
    entry, err := a.uiStore.AppendLog(level, message)
    if err == nil { a.emit("ui:log", entry) }     // 2. UI only if persisted
    else { disk.Error(fmt.Sprintf("UI log persistence failed: %v", err)) }
}
```

Event names are a namespaced vocabulary — keep it small and document it:
`ui:log`, `ui:running`, `ui:show_running`, `ui:showlog`, `ui:botlog`.

Emitting is **an optimisation, not the contract**: the UI also polls `GetLogs(after)` every
second, so a dropped event never loses a line. That is what makes the same frontend work in
a browser (no events at all).

## 6. Per-copy WebView profile (a real bug worth inheriting the fix for)

```go
webviewDir := filepath.Join(dataDir, "webview")
_ = os.MkdirAll(webviewDir, 0o755)
// windows.Options{ WebviewUserDataPath: webviewDir }
```
Default behaviour put the profile in `%APPDATA%\<exe name>`; a second copy of the app then
failed with `0x800700AA` ("resource is in use"). Keep the data-dir-scoped path.

## 7. Data dir resolution

```go
func resolveDataDir() (string, error) {
    wd, _ := os.Getwd()
    if _, err := os.Stat(filepath.Join(wd, "wails.json")); err != nil {
        if exe, e := os.Executable(); e == nil {
            exeDir := filepath.Dir(exe)
            cand := filepath.Clean(filepath.Join(exeDir, "..", ".."))
            if _, se := os.Stat(filepath.Join(cand, "wails.json")); se == nil { wd = cand }
            else if _, se := os.Stat(filepath.Join(exeDir, "wails.json")); se == nil { wd = exeDir }
        }
    }
    return filepath.Join(wd, "data"), nil
}
```

Works both from `wails dev` (cwd = project) and from `build/bin/*.exe` (two levels up).
Never write to the OS user profile; the whole state travels with the project folder.

## 8. HTTP fallback: the same App served to a browser

`apichat-main.go` shows the pattern that makes the frontend portable:

```go
// The UI calls window.go.main.StudioApp when embedded in Wails, and
// /api/<Method> when opened in any browser or test harness.
func (a *StudioApp) HTTPHandler() http.Handler {
    mux := http.NewServeMux()
    writeJSON := func(w http.ResponseWriter, val any, err error) {
        w.Header().Set("Content-Type", "application/json")
        if err != nil { w.WriteHeader(http.StatusBadRequest)
            _ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()}); return }
        _ = json.NewEncoder(w).Encode(val)
    }
    mux.HandleFunc("/api/GetStudioState", func(w http.ResponseWriter, _ *http.Request) {
        st, err := a.GetStudioState(); writeJSON(w, st, err)
    })
    mux.HandleFunc("/api/SaveStudioState", func(w http.ResponseWriter, r *http.Request) {
        var req StudioSaveRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil { writeJSON(w, nil, err); return }
        st, err := a.SaveStudioState(req); writeJSON(w, st, err)
    })
    // …one handler per exported method…
    return mux
}
```

Rule: `GET` when the method takes no argument, `POST` with a JSON body otherwise.
Error shape is always `{"error": "..."}` with a 4xx/5xx status — the bridge's `httpInvoke`
depends on it.

## 9. Run / Stop lifecycle

```go
func (a *App) StartBot() error {
    a.mu.Lock()
    if a.running { a.mu.Unlock(); return errors.New("already running") }
    configs, err := a.buildBotConfigs()        // read stores while locked
    if err != nil { a.mu.Unlock(); return err }
    ctx, cancel := context.WithCancel(context.Background())
    a.running, a.botCancel, a.runID = true, cancel, a.runID+1
    a.mu.Unlock()

    a.emit("ui:running", true)
    go func() { defer a.finishRun(); runWorkers(ctx, configs, a.log) }()
    return nil
}

func (a *App) StopBot() error {
    a.mu.Lock(); cancel := a.botCancel; a.botCancel = nil; a.running = false; a.mu.Unlock()
    if cancel != nil { cancel() }
    a.emit("ui:running", false)
    return nil
}
```

Additional useful bits from the original: `activeWorkers map[string]bool` to show how many
profiles are mid-flight, `runID` to ignore results from a stopped run, and the same
`emit("ui:running", false)` in a deferred cleanup so a panic cannot leave the pill green.

## 10. Model call (OpenAI-compatible) — keep it stdlib

```go
payload := map[string]any{
    "model": e.cfg.Model, "messages": msgs,
    "temperature": e.cfg.Temperature, "max_tokens": e.cfg.MaxTokens,
}
body, _ := json.Marshal(payload)
req, _ := http.NewRequestWithContext(ctx, http.MethodPost, e.cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
req.Header.Set("Content-Type", "application/json")
req.Header.Set("Authorization", "Bearer "+e.cfg.APIKey)
resp, err := (&http.Client{Timeout: e.cfg.Timeout}).Do(req)
…
data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))   // always cap the body
if resp.StatusCode >= 400 { return "", fmt.Errorf("%s", strings.TrimSpace(string(data))) }
```

Shown in `reference/go/openai-client.go`. Notes worth copying: context + timeout, 1 MB body
cap, status check before unmarshal, error text trimmed into the returned `error` so the UI
status line shows the provider's own message.

## 11. Logging to disk (applog)

- One file: `data/logs.txt`, appended, never rotated mid-run.
- Every line carries `time | source | level | message`.
- `applog.CaptureOutput(path)` tees `log`/stdout into the file so a panic message survives a
  GUI build with no console.
- Credentials are redacted before writing (the original has tests for
  `access_token`, `api_key`, `password` in URLs and JSON).
- Minimum level policy: file gets everything; the UI shows what the domain chooses to emit.

## 12. Go style rules used in the codebase

- `package main` at the root, domain code under `internal/`.
- Exported methods = UI API; everything else unexported, same file or one package down.
- Errors: `fmt.Errorf("...: %w", err)`; user-facing text is short and lower-case.
- No `panic` outside `main`; no `log.Fatal` outside `main`.
- Comments: a header comment on every file stating its one job, plus *why* for non-obvious
  decisions (the WebView2 comment above is the model example).
- Table-driven tests next to the code (`x_test.go`), using `t.TempDir()` — never the real
  `data/` directory.
