# TEST REPORT — Multi-Site Console, step 2 (2026-10-26)

Everything below was executed in this session. The Go toolchain is real (Go 1.27.3), so
`go vet`, `go build` and `go test` are genuine results, not inspections.

## 1 · Build & static checks

| Check | Command | Result |
|---|---|---|
| Formatting | `gofmt -l .` | clean (no files listed) |
| Compile | `go build ./...` | **PASS** |
| Static analysis | `go vet ./...` | **PASS** (no findings) |
| Frontend syntax | `node --check cmd/console/ui/app.js` | **PASS** |
| Mock syntax | `node --check cmd/console/ui/js/mock-backend.js` | **PASS** |

## 2 · Go unit + HTTP tests (`go test ./...`)

```
?   	multisiteconsole/cmd/console	[no test files]
ok  	multisiteconsole/internal/core	2.754s
```

| Test | What it proves |
|---|---|
| `TestLoadSites` | 5 folders → 5 configs, 5 distinct families, ids/names present |
| `TestLoadSitesSkipsBroken` | a corrupt `site.json` is skipped and reported, 1 good site survives |
| `TestDefaultsAndNormalise` | bad enums repaired, ranges clamped, min≦max enforced, site list de-duplicated |
| `TestStorePersistsAcrossReopen` | saved section survives a full process restart |
| `TestStoreRejectsUnknownSection` | unknown section returns an error (400 at HTTP level) |
| `TestSnapshotShape` | snapshot carries sites, stats, runtime kind/version, non-empty statuses |
| `TestAutoRegisterAccountsWritesFiles` | N accounts written to `sites/<id>/accounts/`, counted in site state, bad site/count rejected |
| `TestRunLifecycle` | start → second start rejected → site completes → stop → log kinds include `out`+`system` |
| `TestLogRingCursor` | cursor semantics, ring cap, clear invalidates the cursor |
| `TestPaceIsAtLeastOneSecond` | configured pacing honoured, floor of 1 s |
| `TestHTTPHealthAndSnapshot` | `/api/health`, `/api/GetSnapshot` |
| `TestHTTPSaveSettingsAndErrors` | `POST /api/SaveSettings` applies + persists; unknown section → 400 + `{"error"}` |
| `TestHTTPLogsAndClear` | `GetLogs` streams, `ClearLogs` resets the feed and bumps the version |
| `TestHTTPSiteDriverAndFolder` | real `dom_driver.js` + `site.json` read from disk; `LoadFolder` counts `.txt` |
| `TestHTTPRunAndStop` | `StartRun` → running → empty site list rejected → `StopRun` settles |
| `TestHTTPStaticUIServed` | UI served from the same origin, API marked `no-store` |

## 3 · Live UI on the mock backend (jsdom, 37 assertions)

Executed against `cmd/console/ui/index.html` + `js/mock-backend.js`:

boot/bridge detection · 5 site cards · site facts · stats (sites, accounts) · checklist (5)
· state table (5) · worker summary · site switch updates facts + run summary · all 5 show
sub-panels · save setting · draft survives 3 s polling · strategy switch reveals folder block
· save message · **Test Site verdict** · **DOM driver viewer** · worker panel · save selection
(3 selected) · auto-register 2 accounts + table rows · proxy test (2 verdicts) · worker run
(10 log rows, ≥2 kinds) · filter · stop · clear · 3 themes · **0 runtime errors**

**Result: 37 / 37 PASS**

## 4 · Live UI against the real Go backend (jsdom + fetch, 18 assertions)

Server: `go run ./cmd/console -addr 0.0.0.0:8090 -data /tmp/msdata`, no mock loaded.

| Assertion | Result |
|---|---|
| Bridge resolves to `http` (real API, demo banner hidden) | PASS |
| 5 sites come from the Go registry | PASS |
| Stats + version come from the backend | PASS |
| `SaveSettings` via `/api` → status "Saved." | PASS |
| Value persisted in Go (`reply_min === 2` on re-read) | PASS |
| `TestSite` returns a real verdict (network blocked in sandbox → `FAIL · EOF · 14 ms`, rendered correctly) | PASS |
| Driver viewer reads the real `site.json`/`dom_driver.js` from `sites/` | PASS |
| `AutoRegisterAccounts` → 2 JSON files on disk, table rendered | PASS |
| Stats + site state reflect new accounts | PASS |
| `StartRun` → sidebar Running (Go goroutine) | PASS |
| Live logs from the Go ring, 14 rows, each tagged with a site id | PASS |
| `StopRun` → Stopped | PASS |
| `TestConnections` → 1/2 passed (`127.0.0.1:8090` ok, `10.255.255.1:1080` i/o timeout) | PASS |
| 0 uncaught runtime errors | PASS |

**Result: 18 / 18 PASS**

## 5 · Files created by the live run (proof of real writes)

```
sites/camgo/accounts/Hannah86_1793037159776643717.json
sites/camgo/accounts/Mia55_1793037159776749139.json
data/settings.json      (reply_min: 2, worker.sites: [camgo chat42 chatusa omezy viby])
```

## 6 · NOT RUN (honest list)

- **Wails desktop build** — the Wails module cannot be downloaded in this sandbox; the shell
  is step 5 and is built on Windows with `wails build` (runbook: design-kit §11).
- **Real browser automation (Rod)** — same download restriction; the job runner in step 2 is
  a real goroutine with real pacing and real logging, but it does not drive a browser yet
  (step 4).
- **Real registration on the 5 live sites** — outbound network is restricted to a small
  allow-list here, so the probe intentionally shows `FAIL · EOF`. On a normal machine with
  internet, `Test Site` returns the site's real HTTP status.
- **Visual browser screenshot** — the UI was exercised in jsdom; a human click-through is one
  `go run ./cmd/console` away.

## 7 · Reproduce

```bash
go vet ./... && go test ./...
go run ./cmd/console -addr 0.0.0.0:8090 -data /tmp/msdata &
node tests/ui-real.mjs          # needs jsdom (npm i jsdom)
node tests/ui-mock.mjs
```
