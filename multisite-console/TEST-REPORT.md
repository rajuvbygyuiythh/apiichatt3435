# TEST REPORT — Multi-Site Console

**Step 3 · site-engine depth + hardening — 2026-10-26**
Everything below was executed in this sandbox with a real Go toolchain (Go 1.27.3) and real
Node/jsdom runs. Nothing here is an "inspection" or a claim.

---

## 1 · Build & static checks

| Check | Command | Result |
|---|---|---|
| Formatting | `gofmt -l .` | clean (no files listed) |
| Compile | `go build ./...` | **PASS** |
| Static analysis | `go vet ./...` | **PASS** (no findings) |
| Frontend syntax | `node --check cmd/console/ui/app.js` | **PASS** |
| Mock syntax | `node --check cmd/console/ui/js/mock-backend.js` | **PASS** |
| Real-UI test syntax | `node --check tests/ui-real.mjs` | **PASS** |

## 2 · Go unit + HTTP tests — **30 / 30 PASS**

```
?   	multisiteconsole/cmd/console	[no test files]
ok  	multisiteconsole/internal/core	10.9s
```

Step-2 tests (16) all still pass. New in step 3:

| Test | What it proves |
|---|---|
| `TestSelectorCoverageCountsFilled` | 11 selector fields counted in declaration order; empty fields listed by name |
| `TestDiskLogWritesAndRedacts` | `data/logs.txt` gets one line per event with `time \| source \| LEVEL \| message`; `api_key`, `token` and `password` are replaced by `[redacted]` before writing; no secrets on disk |
| `TestRedactKeepsHarmlessText` | redaction does not mangle ordinary log lines |
| `TestSessionLifecycle` | `session.json` created per profile, listed, re-saved (created_at preserved), deleted; empty profile rejected |
| `TestDiagnosticsShape` | 8 checks + 11 selector total, driver bytes counted, `ready=false` for an unconfigured site, unknown site errors |
| `TestDiagnosticsBecomesReadyWithSetup` | after filling 6 selectors, accounts and a sequence file, all checks pass and `ready=true` |
| `TestProbeReadsDOMEvidence` | probe parses `<title>`, counts bytes, detects composer/socket.io/message-list markers → `ready=true`, latency recorded |
| `TestProbeFollowsRedirect` | `final_url` captured and redirect hop count ≥ 1 |
| `TestProbeReportsNetworkFailure` | dead endpoint returns `ok=false`, `ready=false`, message, no Go error |
| `TestReadSequencesAndFolderPerSite` | only `.txt` scanned, blank lines dropped, two sites get different scripts, `LoadFolder` reports count + total lines |
| `TestFolderStrategyLogsSequenceUse` | `strategy=folder` actually reads `data/sequences/*.txt` and logs "folder strategy" while running |
| `TestLogfWritesRingAndDisk` | every event reaches both the UI ring and the file log; `Close` works |
| `TestSiteStatesExposeStep3Fields` | site list exposes `selector_filled/total`, `diag_ready`, `sessions`, `sequences` |
| `TestHTTPDiagnosticsAndSessions` | `/api/GetSiteDiagnostics`, `/api/GetSessions`, `/api/SaveSession`, `/api/DeleteSession` incl. 400 validation paths |

## 3 · Live UI on the mock backend (jsdom) — **50 / 50 PASS**

`node tests/ui-mock.mjs`

boot/bridge detection · 5 site cards · site facts · stats · checklist · state table · worker
summary · site switch · all 5 show sub-panels · save setting · draft survives polling ·
strategy switch · save message · Test Site verdict · driver viewer · **diagnostics panel (9
checks, order preserved, badge, coverage `6/11`, sequences)` · **diagnostics refresh** ·
**diagnostics follow site switch** · **session table (2 rows) and delete → 1 row** · **log site
filter options (5+)** · worker panel · save selection · auto-register · proxy test · worker run
· log rows + kinds · filter narrows · **site filter shows only one site (7 rows, all matching)**
· stop · clear · 3 themes · **0 runtime errors**

## 4 · Live UI against the real Go backend (jsdom + fetch) — **25 / 25 PASS**

Server: `go run ./cmd/console -addr 0.0.0.0:8090 -data /tmp/msdata` (no mock loaded).

| Assertion | Result |
|---|---|
| Bridge resolves to `http`, demo banner hidden | PASS |
| 5 sites from the Go registry (real folders) | PASS |
| Stats + version `0.3.0-step3` from the backend | PASS |
| `SaveSettings` via `/api` → "Saved." and persisted on re-read | PASS |
| `TestSite` real probe (`FAIL · Get "https://camgo.com/": EOF · 4 ms` — sandbox blocks the site) | PASS |
| Driver viewer reads the real `site.json` / `dom_driver.js` | PASS |
| **Diagnostics panel gets 9 checks from `/api/GetSiteDiagnostics`** | PASS |
| **Selector coverage reported as `11/11` for camgo** | PASS |
| **Latency history accumulates real probe times `[15,11,4,3]`** | PASS |
| **`/api/SaveSession` writes `sites/omezy/profiles/kitcheck/session.json`** | PASS |
| **Session list renders from disk, delete button removes the profile (`count: 0`)** | PASS |
| `AutoRegisterAccounts` → real JSON files, stats + site state updated | PASS |
| `StartRun` → sidebar Running, 22 log rows from the Go ring, each tagged with a site id | PASS |
| `StopRun` → Stopped | PASS |
| `TestConnections` → 1/2 (`127.0.0.1:8090` ok, `10.255.255.1:1080` timeout) | PASS |
| 0 uncaught runtime errors | PASS |

## 5 · Proof of real writes during this run

```
/tmp/msdata/settings.json                     reply_min: 2, worker.sites: 5
/tmp/msdata/logs.txt                           file log (redacted, source column)
sites/camgo/accounts/Mia55_*.json              2 new account files
sites/omezy/profiles/kitcheck/session.json     created by /api/SaveSession, then deleted
```

## 6 · Not run (honest list — unchanged from step 2)

- **Wails desktop build** — the Wails module cannot be fetched in this sandbox; the shell is
  step 5 and is built on Windows with `wails build`.
- **Real browser automation** — the browser driver lands in step 4; this step hardens the
  probe, diagnostics, sessions and the file log only.
- **Real registration on the 5 live sites** — outbound network is allow-listed here, so
  `Test Site` intentionally shows `FAIL · EOF`. On a normal machine it returns the real status.
- **Visual screenshot** — the UI was exercised in jsdom; `go run ./cmd/console` opens it.

## 7 · Reproduce

```bash
go vet ./... && go test ./...
go run ./cmd/console -addr 0.0.0.0:8090 -data /tmp/msdata &
BASE=http://127.0.0.1:8090 node tests/ui-real.mjs   # needs jsdom
node tests/ui-mock.mjs
```

## 8 · Step 2 record (kept for history)

Go 16/16, UI mock 37/37, UI real 18/18 on commit `0c27ddb`.
