# 11 · BUILD & RUN RUNBOOK

## 1. Just want to see the design (no toolchain)

```bash
cd starter
python3 -m http.server 8080        # or: npx serve .   /  php -S localhost:8080
# open http://localhost:8080
```

Runs on the in-page mock (`js/mock-backend.js`). The sidebar shows a `demo` pill and a
yellow banner — that is expected. Press **Run** and watch the log feed fill.

To open it as a local file instead (`file://`), remove the two `<script>` tags at the bottom
and inline the mock — served over HTTP is simpler.

## 2. Frontend checks (Node, no browser)

```bash
node --check starter/app.js
node --check starter/js/mock-backend.js
node -e "JSON.parse(require('fs').readFileSync('design-tokens.json'));console.log('tokens ok')"
```

jsdom smoke test (the one used to validate this kit — needs `npm i jsdom`):

```bash
node -e '
const {JSDOM}=require("jsdom"),fs=require("fs");
const h=fs.readFileSync("starter/index.html","utf8").replace(/<script[^>]*><\/script>/g,"");
const d=new JSDOM(h,{url:"http://localhost/",runScripts:"outside-only",pretendToBeVisual:true});
const w=d.window;
w.eval(fs.readFileSync("starter/js/mock-backend.js","utf8"));
w.eval(fs.readFileSync("starter/app.js","utf8"));
w.document.dispatchEvent(new w.Event("DOMContentLoaded"));
setTimeout(()=>{
  w.document.getElementById("studio-run").click();
  setTimeout(()=>{
    const rows=w.document.getElementById("studio-log-list").children.length;
    console.log(rows>0?"PASS: log feed rendered "+rows+" rows":"FAIL: no log rows");
    w.close(); process.exit(rows>0?0:1);
  },2600);
},900);
'
```

## 3. Desktop build (Go + Wails, Windows)

```bat
install.bat            :: install Go/Wails/WebView2, build, offer to run
install.bat build      :: build only
install.bat run        :: build if missing, then start
install.bat bundle     :: portable bundle (WebView2 fixed runtime + Chromium)
run-dashboard.bat      :: build-if-missing + start
test.bat               :: console tester (project-specific)
verify.bat             :: verification gate
```

Manual equivalent:

```bash
go mod download
wails build -clean -skipbindings -m -s -webview2 error -o myapp.exe
# fallback if the Wails CLI is unavailable:
go build -tags desktop,production -ldflags "-w -s -H windowsgui" -o build/bin/myapp.exe .
```

Notes that cost the original project real time:

| Symptom | Cause / fix |
|---|---|
| `0x800700AA` on second launch | WebView2 profile shared in `%APPDATA%` → set `WebviewUserDataPath` to `data/webview` |
| Build fails: exe locked | The app is running and locks `build/bin/*.exe` — close it first (install.bat checks this) |
| Blank white window | `AssetServer.Assets` points at the wrong `fs.Sub` root, or `dist/` missing → check `go:embed all:<dir>` |
| Window flashes white on open | `BackgroundColour` ≠ CSS `--bg` |
| `wails: command not found` | `go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0`, then add `$(go env GOPATH)/bin` to PATH |
| UI shows `demo` pill in the exe | Bindings not generated/attached → `wails dev` once, check `Bind: []interface{}{app}` |

### Linux / macOS

Wails works, but drop the Windows-only parts: `windows.Options` block, `runtimeassets`
(WebView2), `go-winloader`, `wmi`, `go-toast`. WebKitGTK (`libwebkit2gtk-4.1`) must be
installed for Wails v2.

## 4. Dev loop

```bash
wails dev          # hot reload of the frontend, Go rebuild on save
```

`wails dev` regenerates `frontend/wailsjs/**` bindings — that folder is generated, do not
hand-edit it, and gitignore or commit per your team's rule (original committed it).

If you have no Wails CLI yet, develop purely in the browser (`§1`) and only run the desktop
build when the UI is settled. The mock backend makes this a first-class workflow.

## 5. Runtime data

```
data/
├── ui/badger/      settings + logs (Badger)
├── webview/        WebView2 user-data dir
├── logs.txt        append-only diagnostics
└── <domain>/       sequences / prompts / captures / exports
```

Reset everything: close the app, delete `data/`. Reset only settings: delete
`data/ui/badger/`. **Never** delete `data/` while the app is running (Badger lock).

## 6. Verification gate (what "done" means)

```
1  node --check                       every JS file
2  node --test tests/                 contract + UI tests (jsdom)
3  go vet ./...                       static checks
4  go test ./...                      unit tests (use t.TempDir())
5  manual desktop run                 launch, save, run, stop, restart, values persisted
6  checklist                          12-CHECKLIST-70-30.md
```

Keep PASS / FAIL / BLOCKED / NOT_RUN distinct in any report you write. "Skipped" is not
"passed". Do not claim a desktop run happened if you only ran the browser version.

## 7. Packaging

- **Source package**: only the verified inventory (`scripts/package-source.cjs` pattern —
  explicit file list, deterministic zip, refuses to include stale/blocked reports).
- **Windows bundle**: `scripts/bundle-windows.ps1` downloads the WebView2 fixed runtime +
  Chromium so the app runs on a machine with nothing installed.
- Ship `data/` empty (or not at all). Ship `logs.txt` never.

## 8. Environment

| Tool | Version | Check |
|---|---|---|
| Go | ≥ 1.24 | `go version` |
| Wails CLI | v2.12.0 | `wails version` |
| Node (dev only) | ≥ 18 | `node -v` |
| WebView2 runtime | any current | Windows 11 has it; else install from Microsoft |
| Windows | 10/11 x64 | primary target |

## 9. Troubleshooting the UI itself

| Symptom | Look at |
|---|---|
| Nothing renders, console error at boot | `resolveBridge()` — mock file missing? script order? |
| `Backend method missing: X` | bridge kind is `wails` and the Go method is unexported / not bound |
| Fields revert after a few seconds | `applySnapshot` writing a field the user owns → check `dirtyFields` |
| Log rows don't scroll | `.log-list` has no fixed height (a parent `display:grid` collapsed it) |
| Logs duplicate | cursor not advanced (`bucket.after = res.version`) or `pending` guard missing |
| Tabs stop working after an error | an exception before `setBusy(...)=false` — check `finally` |
| Theme looks half-applied | a theme missing tokens → all twelve must be defined |
