# Desktop shell (Wails v2)

This module turns the console into a native window. It is deliberately **separate** from the
repository's main module so that `go build ./...` and `go test ./...` at the project root never
need the Wails dependency, and so CI (or a machine with no WebView) stays green.

```
desktop/
├── go.mod            own module; replace multisiteconsole => ../  (uses the real core)
├── main.go           wails.Run + window options + data/sites resolution
├── wails.json        app name, output file name, product info
└── frontend/dist/    the dashboard assets embedded into the binary
```

## Build (Windows, one time)

```bat
scripts\install.bat        REM checks Go, downloads modules, installs the Wails CLI
scripts\build-desktop.bat  REM syncs the UI, runs `wails build`, copies sites next to the exe
```

Output: `build\bin\Multi-Site-Console.exe` (+ `sites\` beside it).

## Where the desktop build keeps its files

| What | Path |
|---|---|
| settings + logs + sequences | `%APPDATA%\MultiSiteConsole\data\` |
| site profiles / sessions | `%APPDATA%\MultiSiteConsole\sites\` if it exists, else `sites\` beside the exe |
| override | `set MULTISITE_HOME=D:\my\folder` or `set MULTISITE_SITES=...` |

The UI folder is embedded (`//go:embed all:frontend/dist`), so the exe is self-contained apart from
the `sites/` data you may keep beside it. Re-run `scripts\sync-ui.bat` after editing
`cmd/console/ui/*` (the batch/`.sh` variant does the same copy).

## Why not embed the UI straight from `cmd/console/ui`?

Wails requires the asset folder inside its own module directory. Copying keeps a single source of
truth for the dashboard (`cmd/console/ui`) while satisfying that requirement; the sync step is
idempotent and takes a second.

## What the window binds

`Bind: []interface{}{app}` exposes every exported method of `core.App` — the same 17 methods the
HTTP server exposes at `/api/<Method>`. `app.js` detects `window.go.main.App` first, so the exact
same dashboard runs in the browser (HTTP) and in the desktop window (native binding).
