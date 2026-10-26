# TEST REPORT — Multi-Site Console

**Step 4 · real browser driver (CDP) — 2026-10-26**
Every number below comes from a command that was actually executed in this sandbox with
Go 1.27.3 and Node/jsdom. Where something could not be executed it is listed in §6.

---

## 1 · Build & static checks

| Check | Command | Result |
|---|---|---|
| Formatting | `gofmt -l .` | clean |
| Compile | `go build ./...` | **PASS** |
| Static analysis | `go vet ./...` | **PASS** (no findings) |
| Frontend syntax | `node --check cmd/console/ui/app.js` | **PASS** |
| Mock syntax | `node --check cmd/console/ui/js/mock-backend.js` | **PASS** |
| Real-UI test syntax | `node --check tests/ui-real.mjs` | **PASS** |

## 2 · Go tests — **40 / 40 PASS, 0 skipped**

```
ok  	multisiteconsole/internal/core	40.4s
```

Step 2 (16) and step 3 (14) tests still pass. New in step 4:

| Test | What it proves |
|---|---|
| `TestWebSocketClientEcho` | hand-rolled WS client: upgrade, masked client frames, 70 000-byte payload (64-bit length path), server echo |
| `TestWebSocketRejectsNonUpgrade` | a non-101 handshake is an error, not a panic |
| `TestBrowseLaunchNavigateEvaluate` | full launch → `/json/list` → attach page socket → `Page.navigate` → `Runtime.evaluate` → `Page.captureScreenshot` (PNG bytes decoded) |
| `TestLaunchBrowserFailsFastWithoutTarget` | a browser that never opens the debug port fails inside the timeout |
| `TestDiscoverBrowserHonoursEnv` | `MULTISITE_BROWSER` wins, source reported, `--version` parsed |
| `TestDiscoverBrowserMissing` | missing browser → `found=false` + note + candidate list (no crash) |
| `TestDriverSendAndSnapshot` | driver decodes the page snapshot, inserts text via `Input.insertText`, clicks SEND/SKIP, reads `probe()` |
| `TestDriverSendFailsWithoutComposer` | no composer → explicit error instead of a silent no-op |
| `TestRunRealBrowserCycle` | **the real runner**: launch → navigate → install driver → snapshot → 2 outbound lines → partner reply logged as `in` → SKIP → `session.json` written → `data/logs.txt` mirrored → 1 completed / 0 failed |
| `TestRunSimulationWhenNoBrowser` | without a browser the run says `SIMULATION` in the log instead of pretending |

The real-cycle test uses a scripted Chrome (`fakeChrome`) that speaks the DevTools protocol over a
real socket, so the WebSocket framing, the CDP id/reply matching, the launch handshake, the driver
injection, the send path, the message pump and the run bookkeeping are all executed — not mocked at
the function level.

## 3 · Live UI on the mock backend (jsdom) — **58 / 58 PASS**

`node tests/ui-mock.mjs` — adds to the step-3 suite: browser status line, headless/page-timeout
defaults, save browser settings, Detect, **Browser Check → PASS with screenshot path**, plus all
step-3 diagnostics/sessions/log-filter assertions and 0 runtime errors.

## 4 · Live UI against the real Go backend (jsdom + fetch) — **31 / 31 PASS**

Server: `go run ./cmd/console -addr 0.0.0.0:8090 -data /tmp/msdata` (v0.4.0-step4), no mock loaded.

| Assertion | Result |
|---|---|
| Bridge → real HTTP, 5 sites, stats, version `v0.4.0-step4` | PASS |
| `SaveSettings` + persistence | PASS |
| `TestSite` real probe (`FAIL · EOF` — sandbox blocks the sites) | PASS |
| Diagnostics: 9 checks, `11/11` selectors, latency history `[68,3]` | PASS |
| Sessions: create / list / delete on disk | PASS |
| **`GetBrowserStatus` → `found:false` + 8 candidate paths on this Linux box** | PASS |
| **missing browser explained in the UI, Browser Check fails cleanly (no crash)** | PASS |
| **worker run states `simulation` honestly instead of faking a browser** | PASS |
| Auto-register, StartRun/StopRun, 24 log rows, proxy dial 1/2 | PASS |
| 0 uncaught runtime errors | PASS |

## 5 · What was really written during these runs

```
sites/<id>/profiles/<profile>/session.json      written by the real run (fake Chrome) and by SaveSession
data/logs.txt                                    every event, secrets redacted
data/screenshots/<site>-<stamp>.png              Browser Check (unit test decodes PNG bytes)
```

## 6 · Not run (honest list)

- **A real Chrome/Edge window** — this sandbox has no browser binary and no route to a download
  mirror. `GetBrowserStatus` correctly reports `found:false`; the launch → CDP path is covered by the
  scripted Chrome above. On a Windows machine with Chrome installed the same code path runs
  unchanged (see §7).
- **Real chat against the 5 live sites** — outbound network here is allow-listed to GitHub/npm/PyPI,
  so site probes answer `EOF`.
- **Wails desktop build** — step 5, built on Windows with `wails build`.

## 7 · Reproduce (with a real browser)

```bash
go vet ./... && go test ./...
go run ./cmd/console -addr 0.0.0.0:8090 -data ./data &
# open http://127.0.0.1:8090 → Worker ▸ Settings ▸ Browser driver ▸ Detect
# Show ▸ Site ▸ Browser Check   (opens headless Chrome, injects the site driver, reports phase)
```

## 8 · History

| Step | Evidence |
|---|---|
| 2 | Go 16/16, UI mock 37/37, UI real 18/18 — commit `0c27ddb` |
| 3 | Go 30/30, UI mock 50/50, UI real 25/25 — commit `30176d6` |
| 4 | Go 40/40, UI mock 58/58, UI real 31/31 — this commit |
