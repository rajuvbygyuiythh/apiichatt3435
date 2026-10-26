# Multi-Site Console (5-site engine)

A desktop-first operations console for **five stranger-chat sites**, one per engine family.
Built on the console design kit (2 modes × 5 sections, dark sidebar shell, live log feed),
with a standard-library Go core that can run in a browser *or* inside a Wails window.

```
Show Mode   1 Site · 2 Setting · 3 Message · 4 Run · 5 Live Logs      (one visible browser)
Worker      1 Sites · 2 Accounts · 3 Message · 4 Settings · 5 Logs     (headless, multi-site)
```

> Status: **step 2 of 5** — the 30% is frozen (`NOTES.md`), the scaffold runs end-to-end on a
> real Go backend. Browser automation and the Wails shell land in steps 4–5. See
> `NOTES.md` §7 for the exact real-vs-simulated table.

## Run it (no toolchain beyond Go)

```bash
go run ./cmd/console                 # http://localhost:8090
```

Flags: `-addr 0.0.0.0:8090` · `-data data` · `-sites sites` · `-ui cmd/console/ui`

Pure-UI mode (no Go at all): open `cmd/console/ui/index.html` over any static server —
the page falls back to `js/mock-backend.js` and shows a `demo` pill.

## Verify it

```bash
export PATH=<go binary>:$PATH        # Go 1.24+
go vet ./...                         # static checks
go test ./...                        # 12 core + HTTP tests, no network needed
node --check cmd/console/ui/app.js
node -e "require('./tests/ui-mock.mjs')" 2>/dev/null || node tests/ui-mock.mjs   # needs jsdom
node tests/ui-real.mjs               # 18 assertions against the running Go server
```

Real results are in `TEST-REPORT.md`.

## Layout

```
cmd/console/main.go            web mode: one HTTP server (UI + /api)
cmd/console/ui/                the dashboard (index.html · styles.css · app.js · js/mock-backend.js)
internal/core/model.go         settings structs, snapshot, log entry, defaults + normalise
internal/core/catalog.go       site registry: reads sites/<id>/site.json, driver, protection note
internal/core/store.go         settings.json persistence (atomic, corrupt-safe)
internal/core/log.go           versioned log ring (append-only, cursor polling)
internal/core/app.go           App: the 13 contract methods + job runner
internal/core/http.go          /api/<Method> surface for web mode
internal/core/*_test.go        registry, store, normalise, accounts, run lifecycle, HTTP
sites/<id>/                    the 5 sites: site.json, dom_driver.js, *_protection.txt
data/                          runtime: settings.json, sequences/, (accounts are per-site)
```

## Contract (all calls take one object argument)

`GetSnapshot` · `SaveSettings{section,values}` · `GetLogs{after}` · `ClearLogs` ·
`TestSite{site_id}` · `TestConnections{proxy}` · `StartRun{mode,sites,profile}` · `StopRun` ·
`LaunchLogin{site_id,profile}` · `AutoRegisterAccounts{site_id,count}` ·
`GetSiteDriver{site_id}` · `LoadFolder` · `OpenDataFolder`

Same names over three transports: Wails bindings (step 5), `POST /api/<Method>` (web mode),
and the in-page mock. The frontend never knows which one answered.

## Design rules carried over from the kit

- `styles.css` is the kit's stylesheet plus one clearly-bannered block of additions.
- Two modes, five sections each — no third mode, no modal flows.
- Every action reports back: status line, log row, or toast.
- One field = one HTML input + one read line + one `setValue` line.
- Drafts survive sub-panel switches and 3 s polling (`formStore` + `dirtyFields`).
- A broken site folder can never break the console: it is skipped and logged as `error`.

## Adding a site

```bash
cp -r sites/template_new_site sites/newsite     # edit id/name/family/start_url/selectors
go run ./cmd/console                            # registry picks it up automatically
```
