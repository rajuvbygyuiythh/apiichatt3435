# reference/ — the original source, read-only

These are the **actual files** from the ChitChat build that produced the design in
`starter/`. They are here so an agent can resolve any ambiguity ("what exactly did the
status line look like?") without guessing.

> ⚠️ **Read, don't ship.** Nothing here is licensed for your product's behaviour, and it
> contains the original domain (chat bot / persona / Snap-Telegram targeting). Copy
> *structure and styling* only. Secrets are already empty/placeholders.

## frontend/

| File | What it is | Study it for |
|---|---|---|
| `dashboard-index.html` | The full dashboard markup (560 lines): sidebar, 2 modes × 5 sub-panels, stats row, log cards, profile pickers | Exact DOM for every component, aria attributes, id naming. This is the design truth |
| `dashboard-styles.css` | The complete design system (1 088 lines) — identical to `starter/styles.css`'s top part | Any rule you are unsure about |
| `dashboard-app.js` | The original shell + domain players (1 024 lines) | `formStore`, `fillX(prefix)`, `renderLogFeed`, event wiring, boot retries |
| `studio-index.html` / `studio-styles.css` / `studio-app.js` | A **second** dashboard (`apichat.exe`): 3 tabs, blue theme, same button/card language | A smaller layout; the blue palette (shipped as the `ocean` theme); `callStudio()` dual bridge (Wails **or** `/api/…`) |

## go/

| File | What it is | Study it for |
|---|---|---|
| `chitchatui-main.go` | `main.go`: `go:embed`, `resolveDataDir()`, `wails.Run` options | Window size/colour, asset server, WebView2 data path, bind |
| `apichat-main.go` | `StudioApp` + `HTTPHandler()` serving `/api/<Method>` | The browser-mode JSON API, error envelope, embed of a second UI |
| `app-methods-excerpt.go` | The `App` struct + `emit/log` plumbing, lifted out of `app.go` | Lock discipline, emit vocabulary, "disk log first" |
| `model-ui.go` | All settings structs, JSON tags, `Default*()`, `UISnapshot`, `LogEntry` | Struct/default conventions to copy |
| `uistore-store.go` | Badger store: open, `settings/*` keys, log ring, JSON mirror | Persistence patterns, normalisation on load |
| `openai-client.go` | One OpenAI-compatible `/chat/completions` call | `context` + timeout, body cap, status handling, error text for the UI |
| `install.bat` / `run-dashboard.bat` / `test.bat` | Installer / launcher / local tester | Build commands, toolchain checks, WebView2 install, exe-lock handling |
| `wails.json`, `package.json` | Project config + dev-only test tooling | Names to rename; the `node --test` verification setup |

## How to use these files while building

1. Pick the component in `dashboard-index.html`, copy its markup into your project.
2. If a class isn't obvious, `grep -n "\.class-name" dashboard-styles.css`.
3. For behaviour, find the matching function in `dashboard-app.js` (it is ordered:
   helpers → routing → settings read/fill → logging → actions → wiring → boot).
4. For the backend shape, read the Go files in this order: `model-ui.go` →
   `uistore-store.go` → `chitchatui-main.go` → `app-methods-excerpt.go` → `apichat-main.go`.
5. Then implement with the *generic* names from `08-API-CONTRACT.md` — do not carry the
   original method names into a new product.

## What is intentionally missing

`internal/bot/**`, `internal/apichat/**` (engine, director, guard, prompt, sanitize),
`internal/browser/**` (site selectors, DOM scripts), `apitest/`, persona/sequence data and
the `docs/` of the original. Those are the product's behaviour, not its design. The reusable
*architectural ideas* from them are described instead in `07-BACKEND-PATTERNS.md`.
