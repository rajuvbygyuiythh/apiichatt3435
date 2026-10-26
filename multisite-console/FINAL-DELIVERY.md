# Multi-Site Console — final delivery (v1.0.0)

A Go + WebView console that drives **5 real chat sites**, one isolated folder per site, with a
Show mode (one visible browser) and a Worker mode (headless, several sites), a live log feed,
per-site diagnostics, saved sessions and a redacted file log.

Built in 5 approved steps; each step was tested before the next one started.

| Step | Delivered | Evidence |
|---|---|---|
| 1 | Design kit zip (docs + runnable starter + reference code) | `chitchat-design-kit.zip`, TEST-REPORT in the kit |
| 2 | Scaffold: 5-site registry, stdlib Go core, 13 contract methods, kit UI | Go 16/16, UI mock 37/37, UI real 18/18 |
| 3 | Depth: diagnostics (11-selector coverage), sessions, file log with redaction, real folder strategy, richer probes | Go 30/30, UI mock 50/50, UI real 25/25 |
| 4 | Real browser driver: stdlib WebSocket + Chrome DevTools Protocol, headless checks, real Run/Stop/Login | Go 40/40, UI mock 58/58, UI real 31/31 |
| 5 | Wails desktop shell, Windows scripts, final polish + packaging | see §4 |

## 1 · Layout

```
multisite-console/
├── cmd/console/main.go          web mode entry point (flags: -data -sites -ui -addr)
├── cmd/console/ui/              dashboard (index.html, styles.css, app.js, js/mock-backend.js)
├── internal/core/               stdlib-only engine (registry, settings, log, probes, sessions,
│                                diagnostics, WebSocket + CDP browser driver, runner)
├── desktop/                     Wails v2 shell (own module) + frontend/dist assets
├── scripts/                     install.bat, run-web.bat, build-desktop.bat, sync-ui.*
├── sites/<id>/                  site.json, dom_driver.js, <id>_protection.txt, accounts/, profiles/
├── tests/                       ui-mock.mjs, ui-real.mjs (jsdom)
└── NOTES.md · TEST-REPORT.md · SITE-ANALYSIS.md · README.md
```

## 2 · Run it now (no build, no dependencies beyond Go)

```bat
scripts\run-web.bat
:: or
go run ./cmd/console -addr 127.0.0.1:8090 -data data
```

Open <http://127.0.0.1:8090>. Without a browser installed the console still runs and says
`SIMULATION` in the log; with Chrome/Edge it drives real pages.

## 3 · Desktop build (Windows)

```bat
scripts\install.bat
scripts\build-desktop.bat
build\bin\Multi-Site-Console.exe
```

Settings/logs live in `%APPDATA%\MultiSiteConsole\data`; profiles in `%APPDATA%\MultiSiteConsole\sites`
(or `sites\` next to the exe). Details: `desktop/README.md`.

## 4 · Verification at delivery

| Suite | Result |
|---|---|
| `gofmt -l .` / `go vet ./...` / `go build ./...` | clean |
| `go test ./...` | **40 / 40 PASS** (incl. a full real-browser run cycle against a scripted Chrome) |
| `node tests/ui-mock.mjs` | **58 / 58 PASS** |
| `BASE=http://127.0.0.1:8090 node tests/ui-real.mjs` | **31 / 31 PASS** |

Honest limits: this sandbox has no Chrome binary and no route to the 5 live sites, so the real
browser and real chat runs are covered by (a) the scripted DevTools server in the Go tests and
(b) the `GetBrowserStatus` failure path in the UI tests. Everything else — settings, sessions,
diagnostics, the file log, the runner, pacing, Stop/cancel — runs for real.

## 5 · Adding a sixth site

1. `cp -r sites/viby sites/newsite`
2. Edit `sites/newsite/site.json` (id, name, start_url, selectors).
3. Adjust `dom_driver.js` if the markup differs.
4. Restart. The registry, the pickers, the tables and the worker list pick it up automatically —
   no Go change, no rebuild of the core.
