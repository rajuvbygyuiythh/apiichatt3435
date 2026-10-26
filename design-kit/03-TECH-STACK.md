# 03 · TECH STACK + METHODS

Everything below is what the original actually shipped. Versions are from its `go.mod` /
`package.json`; use them as-is unless you have a reason not to.

## 1. Stack at a glance

| Concern | Choice | Version | Why |
|---|---|---|---|
| Language (backend) | Go | 1.24 | Single static binary, easy concurrency, `embed` for assets |
| Desktop shell | Wails | v2.12.0 | WebView2/WebKit shell with Go bindings + events, no Node at runtime |
| Frontend | Vanilla HTML + CSS + JS | — | Zero build, full control of fidelity, jsdom-testable |
| Local DB | BadgerDB | v4.9.1 | Embedded KV, no server, JSON docs per key |
| Browser automation | Rod | v0.116.2 | Login capture + DOM chat in a visible browser |
| Browser automation (alt) | chromedp | v0.14.2 | CDP-only path, second backend option in the same UI |
| HTTP client (API calls) | `net/http` (stdlib) | — | One POST to `/chat/completions`, no SDK needed |
| Proxy support | `utls` + custom dialer | v1.8.2 | TLS fingerprint + SOCKS5/HTTP relay |
| Logging | custom `applog` | — | File-first logging, credential redaction, capture of stdout |
| Verification | `node --test` + jsdom | 26.1.0 | Browser-free UI tests, contract tests |
| Packaging | `fflate` (zip) | 0.8.2 | Source package with inventory |

Platform: Windows is the primary target (`.bat` scripts, WebView2 loader, WMI usage).
macOS/Linux work with Wails defaults; drop the four Windows-only deps
(`yusufpapurcu/wmi`, `go-winloader`, `go-webview2`, `go-toast`).

## 2. Methods used, by layer

### 2.1 Go / Wails (application)

| Method | Signature (Go) | JSON shape | Purpose |
|---|---|---|---|
| `startup(ctx)` | `func(context.Context)` | — | Store the Wails ctx, seed defaults, emit initial state |
| `shutdown(ctx)` | `func(context.Context)` | — | Close stores, cancel workers |
| `GetInitialState()` | `() (model.UISnapshot, error)` | object | The single snapshot the UI loads with |
| `SaveBrowserSettings(s)` | `(model.BrowserSettings) error` | `{}` | Persist + normalise |
| `SaveMessageSettings(s)` | `(model.MessageSettings) error` | `{}` | Persist + mirror sync |
| `SaveProxySettings(s)` | `(model.ProxySettings) error` | `{}` | Persist |
| `SaveGeneralSettings(s)` | `(model.GeneralSettings) error` | `{}` | Persist |
| `SaveUnifiedSettings(p)` | `(UnifiedSettingsPayload) error` | `{}` | One round-trip for "Save all" |
| `TestProxyEntries(raw)` | `(string) ([]proxy.TestResult, error)` | array | Backend-side connectivity test |
| `TestAPIChat(s)` | `(model.MessageSettings) (string, error)` | string | One model call, returns human summary |
| `RunBrowser(s, n)` / `RunFreshBrowsers(n)` | | `{}` | Launch the visible browser |
| `StartBot()` / `StopBot()` | `() error` | `{}` | Worker lifecycle |
| `StartLocalTestChat(mode)` / `SendLocalTestChatMessage(text)` / `GetLocalTestChatState()` | | objects | Chat playground inside the dashboard |
| `ListProfiles()` | `() ([]ProfileOption, error)` | array | Fill the profile picker |
| `LoadMessageFolder()` | `() ([]string, error)` | array | Read sequences from the data folder |
| `OpenEVAChatDataFolder()` / `OpenAPIChatDataFolder()` | `() (string, error)` | string | Open Explorer at the data dir |
| `GetShowLiveLog(after)` / `GetBotLiveLog(after)` | `(int) (LogPage, error)` | object | Versioned log polling |
| `ClearLogs()` | `() error` | `{}` | Truncate the log ring |
| `GetLiveChatSnapshot(id)` | `(string) snapshot` | object | Live chat feed |
| `emit(name, data)` | unexported | — | `runtime.EventsEmit` wrapper |
| `log(level, msg)` | unexported | — | Disk log + DB append + `ui:log` event |

Rules visible in this table and worth keeping: every method returns `(value, error)` and
takes/returns **plain structs with JSON tags**; unexported helpers do the work; nothing
returns HTML.

### 2.2 Frontend JS (the important functions)

| Function | Job |
|---|---|
| `bridge.invoke(method, ...args)` | Only call site for backend methods (Wails / HTTP / mock) |
| `resolveBridge()` | Decide transport once at boot: Wails → HTTP probe → mock |
| `activateTab(tab)` / `activateSubpanel(panel)` | Two-level routing with `active` classes |
| `capture/restore/bindGlobalState()` | Draft-safe form store per mode |
| `applySnapshot(snapshot)` | Server → DOM, skipping dirty fields |
| `read<Section>()` | DOM → payload objects, one function per settings section |
| `saveSection(section, values, statusId)` | Save + clear dirty + re-apply + toast |
| `renderLogs(prefix)` | Filter + append rows, pin scroll if near bottom |
| `refreshLogs(prefix)` | Poll `GetLogs(afterVersion)`, guard with `pending` |
| `setStatus(id, msg, kind)` | The only way to write a status line (`error`/`ok`) |
| `toast(msg, kind)` | Transient confirmation, auto-dismiss 4.2 s |
| `setBusy(which, bool)` | Disable Run while working, re-render running state |
| `reportFrontendError(ctx, err)` | `window.runtime.LogError` → `logs.txt` |

### 2.3 CSS method

- One stylesheet, ordered: tokens → base → buttons → shell → sidebar → panels →
  cards/forms → lists/logs → media queries → (starter) additions.
- Tokens only (`--bg`, `--panel`, `--line`, `--accent`, …). No literal colours outside
  `:root` blocks.
- Class names are semantic, kebab-case, **not** utility-first: `.stat-capsule`,
  `.mode-option`, `.log-row`, `.proxy-test-pass`.
- Layout: flex shell (`sidebar` fixed 238px + `main` fluid), `grid` for forms, CSS grid
  `auto-fit` for card groups. No CSS framework, no CSS-in-JS.
- State = class toggles only: `.active`, `.running`, `.stopped`, `.status-ok`,
  `.status-error`, `.success`, `.live`.
- Breakpoint: one, at 900 px — subnav becomes a wrapping row, log card unclamps.

### 2.4 Testing method

- `node --test` for settings/contract/UI tests, jsdom for DOM behaviour, no browser.
- Backend: table-driven Go tests next to the code (`*_test.go`), PASS/FAIL/BLOCKED/NOT_RUN
  kept distinct in reports.
- Manual desktop verification is a separate, explicitly-authorised step — never assumed.

## 3. Minimal dependency set for a derived app

```
go.mod
  github.com/wailsapp/wails/v2        // shell, bindings, events
  github.com/dgraph-io/badger/v4      // settings + logs
  github.com/go-rod/rod               // only if you automate a browser
  github.com/chromedp/chromedp        // only if you want a 2nd browser backend
  github.com/refraction-networking/utls // only if you need proxy/TLS fingerprints
package.json (dev only)
  jsdom                               // UI tests
```

Everything else in the original `go.mod` is an indirect dependency.

## 4. Alternatives that fit the same design

| If you need… | Swap to | Keep |
|---|---|---|
| Smaller binary, no Badger | `encoding/json` files under `data/` | key names, mirror pattern |
| A web deployment instead of desktop | plain `net/http` + `embed` (the "HTTP mode" is already in the bridge) | entire frontend |
| A different shell | Tauri (Rust) or Electron | frontend + contract |
| Real auth on the API | add middleware around `/api/*` | method names |
| SQL reporting | keep Badger for settings, add SQLite read-model for logs | UI unchanged |
