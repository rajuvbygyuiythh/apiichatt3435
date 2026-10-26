# TEST REPORT — Multi-Site Console

**Step 5 · desktop shell + polish + packaging — 2026-10-26**
Every number below comes from a command executed in this sandbox (Go 1.27.3, Node + jsdom).
Where a check could not be executed here, §6 says so plainly.

---

## 1 · Build & static checks

| Check | Command | Result |
|---|---|---|
| Formatting | `gofmt -l .` (core, cmd, desktop) | clean |
| Compile | `go build ./...` | **PASS** |
| Static analysis | `go vet ./...` | **PASS** (no findings) |
| Frontend syntax | `node --check cmd/console/ui/app.js` | **PASS** |
| Mock syntax | `node --check cmd/console/ui/js/mock-backend.js` | **PASS** |
| Real-UI test syntax | `node --check tests/ui-real.mjs` | **PASS** |
| UI sync for desktop | `./scripts/sync-ui.sh` | **PASS** (asset folder refreshed) |

## 2 · Go tests — **40 / 40 PASS**

```
?   	multisiteconsole/cmd/console	[no test files]
ok  	multisiteconsole/internal/core	37.5s
```

Coverage spans steps 2–4: registry, settings store, log ring + cursor, accounts, pacing, run
lifecycle, HTTP surface, diagnostics, sessions, file log + redaction, sequence folder strategy,
WebSocket framing, CDP call/reply, browser discovery, driver send/snapshot, and a **full real run
cycle against a scripted Chrome** (launch → navigate → inject driver → send → partner reply →
SKIP → `session.json` → redacted file log).

## 3 · Live UI on the mock backend (jsdom) — **62 / 62 PASS**

`node tests/ui-mock.mjs`

Boot/bridge detection, site cards, stats, checklist, tables, all 10 sub-panels, save setting +
draft survives polling, strategy switch, Test Site, driver viewer, diagnostics panel (9 checks,
refresh, follows site switch), sessions table + delete, log filters (kind **and** site), browser
status/save/detect, Browser Check → PASS with screenshot path, worker run, stop, clear, 3 themes,
**footer version + data path, browser pill, Open data folder toast**, 0 runtime errors.

## 4 · Live UI against the real Go backend (jsdom + fetch) — **30 / 30 PASS**

Server: `go run ./cmd/console -addr 0.0.0.0:8090 -data /tmp/msdata` → **v0.5.0-step5**.

| Group | Result |
|---|---|
| Bridge → real HTTP, 5 sites, stats, version, demo banner hidden | PASS |
| `SaveSettings` + persistence round-trip | PASS |
| `TestSite` real probe (`FAIL · EOF` — sandbox blocks the sites) | PASS |
| `GetSiteDiagnostics` (9 checks, 11/11 selectors, latency history) | PASS |
| Sessions: save / list / delete on disk | PASS |
| `GetBrowserStatus` → `found:false` + candidate list, explained in the UI | PASS |
| Browser Check on a machine without a browser fails cleanly | PASS |
| Worker run states `simulation` honestly, 24 log rows, Stop settles | PASS |
| Auto-register, proxy dial (1/2), 0 uncaught errors | PASS |

## 5 · Desktop shell (Wails v2)

| Item | Status |
|---|---|
| `desktop/go.mod` (separate module + `replace multisiteconsole => ../`) | written |
| `desktop/main.go` — window, embedded assets, `bootstrap()` paths | written, `gofmt` clean |
| `desktop/frontend/dist` — synced copy of the dashboard | present |
| Native binding surface (19 methods, one object argument each) | written to match `app.js` exactly |
| `scripts/install.bat`, `run-web.bat`, `build-desktop.bat`, `sync-ui.bat/.sh` | written |
| `wails build` **executed** | **no** — see §6 |

## 6 · Not run (honest list)

- **`wails build`** — the Wails module cannot be downloaded here (network is allow-listed to
  GitHub/npm/PyPI; the Go module proxy is not reachable), so the desktop binary is built on the
  user's Windows machine with `scripts\install.bat` + `scripts\build-desktop.bat`. The Go shell is
  compiled only there; the module boundary guarantees the root module and all tests stay green
  either way.
- **A real Chrome window** — no browser binary in this sandbox. `GetBrowserStatus` reports
  `found:false`; the whole CDP path is covered by the scripted Chrome in `step4_test.go`.
- **Real chat on the 5 live sites** — outbound network is restricted here; probes answer `EOF`.

## 7 · Reproduce

```bash
gofmt -l . && go vet ./... && go test ./...
go run ./cmd/console -addr 0.0.0.0:8090 -data /tmp/msdata &
node tests/ui-mock.mjs
BASE=http://127.0.0.1:8090 node tests/ui-real.mjs
```

Windows:

```bat
scripts\run-web.bat            :: browser mode, no dependencies
scripts\install.bat            :: Go check + modules + Wails CLI
scripts\build-desktop.bat      :: Multi-Site-Console.exe in build\bin
```

## 8 · History

| Step | Evidence |
|---|---|
| 2 | Go 16/16 · UI mock 37/37 · UI real 18/18 — `0c27ddb` |
| 3 | Go 30/30 · UI mock 50/50 · UI real 25/25 — `30176d6` |
| 4 | Go 40/40 · UI mock 58/58 · UI real 31/31 — `d3a786d` |
| 5 | Go 40/40 · UI mock 62/62 · UI real 30/30 — this commit |
