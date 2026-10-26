# 02 · ARCHITECTURE

## 1. The shape in one picture

```
┌─────────────────────────────── desktop or browser ───────────────────────────────┐
│                                                                                  │
│   index.html ─── styles.css ────────────────────────────────────────────────┐    │
│        │                                                                   │    │
│        └── app.js  ──►  bridge.invoke(method, arg)                         │    │
│                            │                                               │    │
│             ┌──────────────┼────────────────────────┐                      │    │
│             ▼              ▼                        ▼                      │    │
│      window.go.main.App   fetch("/api/<Method>")   window.__consoleMock   │    │
│      (Wails binding)      (JSON over HTTP)         (in-page mock)         │    │
│             │              │                        │                      │    │
└─────────────┼──────────────┼────────────────────────┼──────────────────────┘────┘
              │              │                        │
      ┌───────▼──────────────▼───────┐         nothing leaves the tab
      │  Go  ·  package main          │
      │  App struct (methods = API)   │
      │    ├─ settings  (uistore)     │
      │    ├─ logs      (append-only) │
      │    ├─ workers   (run/stop)    │
      │    └─ events    → ui:log, ui:running
      └───────┬───────────────────────┘
              │
      ┌───────▼────────────┐      ┌──────────────────────────────┐
      │ data/              │      │ external services            │
      │  ui/badger/        │      │  OpenAI-compatible API       │
      │  logs.txt          │      │  target website (browser)    │
      │  webview/          │      │  proxies                     │
      └────────────────────┘      └──────────────────────────────┘

  Events direction:  Go ──(runtime.EventsEmit)──► app.js subscribers ──► refresh logs/state
```

Two transports, one contract. That is the most important architectural decision in the
kit: the same `App` method names are reachable via the Wails binding *or* via plain HTTP
JSON (`/api/<Method>`), and an in-page mock implements the same names. The frontend always
calls `bridge.invoke("GetSnapshot")` and never knows which one answered.

## 2. Layers, and what belongs in each

| Layer | Files | May contain | Must not contain |
|---|---|---|---|
| Presentation | `index.html`, `styles.css` | Structure, tokens, component classes | Copy that a user could mistake for logic, inline logic |
| UI controller | `app.js` | Routing, form store, bridge, render functions, action wiring | Business rules, HTTP details, `fetch` outside the bridge |
| Application | `app.go` (Go `App`) | Method-per-user-intent, mutex-guarded state, emits | HTML, CSS, string templating of UI |
| Domain | `internal/*` (bot/engine/client/…) | Rules, workers, automation, model calls | Anything that knows about a browser DOM element id |
| Persistence | `internal/uistore`, `internal/store` | Keyed JSON docs, log ring, mirror files | UI concerns, formatting for display |

Dependency direction is one-way: presentation → controller → application → domain →
persistence. A domain package never imports the UI controller.

## 3. Request lifecycle (one click)

Example: user presses **Run** in Studio.

```
1  app.js  runJob()
     ├─ reads Setup / Connections / Credentials / Actions from the DOM
     ├─ invoke("SaveSettings", "setup", {...})   ×4      ← every section saved first
     ├─ clears the local log buffer + rewrites the list
     ├─ invoke("StartRun", {profile})
     └─ status line: "Running. Live logs will appear here."
2  Go App.StartRun()
     ├─ guards with a mutex (already running? return early)
     ├─ builds the worker config from the saved stores
     ├─ starts goroutine(s), stores a context.CancelFunc
     └─ emit("ui:running", true)
3  worker logs a line
     └─ app.log(level, msg)
          ├─ disk logger (logs.txt) — always, even if DB is down
          ├─ Badger append (id = monotonically increasing)
          └─ emit("ui:log", entry)                       ← Wails only
4  app.js
     ├─ if event present: refreshLogs(tab) immediately
     └─ else: 1 s poll  GetLogs(afterVersion)  → append → render → keep scroll pinned
5  stats refresh every 3 s: GetSnapshot() → renderStats() (+ untouched fields only)
```

## 4. Data directory

Resolved at startup, always **inside the project copy** (never `%APPDATA%`), so two copies
of the app cannot fight over one profile:

```
<project>/
└── data/
    ├── ui/badger/          settings + log ring (BadgerDB)
    ├── webview/            WebView2 user-data dir (per copy)
    ├── logs.txt            append-only diagnostic log
    └── <domain folders>    e.g. sequences/, prompts/, captures/
```

Why this matters (real bug, kept as a lesson): the original app let WebView2 default to
`%APPDATA%\<exe name>`. Opening a second copy failed with `0x800700AA` ("resource is in
use") because both tried to attach to the same WebView2 profile. Setting
`WebviewUserDataPath` to `data/webview` fixed it. **Keep that in every derived app.**

## 5. Storage model

BadgerDB, one JSON document per key, not a relational schema:

```
settings/browser   → BrowserSettings   {profile, backend, headless, use_saved_session}
settings/message   → MessageSettings   {chat_mode, sequence, usernames, ai_base_url, …}
settings/proxy     → ProxySettings     {enabled, scheme, entries}
settings/general   → GeneralSettings   {first_message, poll_min_seconds, …}
logs/<zero-padded>  → LogEntry         {time, level, message}
```

Plus `message_settings.json` — a small JSON **mirror** written next to the DB so a second
process (e.g. a studio window) can pick up changes without sharing a handle. Loading logic
is "newest wins" between the DB and the mirror.

Rules:
- A struct always has `json:` tags and is safe to marshal with zero values.
- Loading never fails hard: `LoadX()` returns defaults + `(value, error)`, callers seed
  defaults on first run.
- Saving always normalises enum-ish fields (`backend != rod|chromedp → rod`).

## 6. Why vanilla HTML/CSS/JS and not a framework

- Wails embeds `dist/` with `go:embed`; a framework adds a Node build step to a Go repo.
- The design is a fixed desktop console — ~1.5 k lines of hand-written CSS beats utility
  classes for fidelity, and there is no routing library to fight.
- The whole UI is testable in Node with jsdom, no bundler, no hydration.
- The "app-like" feel (persistent sidebar, no page reloads) comes from the panel toggling
  pattern, not from a router.

If you must port it, port **the patterns** (`06-FRONTEND-PATTERNS.md`) and keep
`styles.css` byte-identical.

## 7. Concurrency model (Go)

- One `App` instance lives for the process. Shared state behind `sync.Mutex`; every public
  method locks, then calls unexported helpers.
- Long-running work takes a `context.Context`; `StopRun()` cancels it and clears the
  cancel func under the mutex.
- Per-worker bookkeeping is a `map[string]bool` (`activeWorkers`) guarded by the same mutex.
- No goroutine writes to the UI directly — everything goes through `emit()` or the log
  append, which is a single call site.

## 8. Failure philosophy

| Failure | Behaviour |
|---|---|
| Badger won't open | Fatal at startup with a clear message (`log.Fatal`) — nothing else can work |
| Disk log unavailable | Continue; console writer still active |
| Log persistence error | Print to disk log, do not emit a broken entry |
| Domain action error | Return `error` → JS shows it in the status line (red) |
| Frontend exception | `window.onerror` → `runtime.LogError` → `logs.txt` |
| Backend missing (browser) | Bridge falls back to mock; `demo-banner` tells the truth |
