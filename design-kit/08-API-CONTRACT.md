# 08 · API CONTRACT (Go ⇄ JS ⇄ HTTP ⇄ mock)

One contract, four implementations must agree: the Wails-bound Go `App`, the HTTP handler,
the in-page mock, and any future backend in another language. If a method is missing in one
of them, the UI must degrade — never crash.

## 1. Conventions

| Rule | Value |
|---|---|
| Naming | `PascalCase` methods, matching Go exported names exactly |
| Args | 0 or 1 argument (a struct/object). Multiple values go inside one object |
| Returns | JSON-serialisable value, or `(value, error)` in Go → thrown `Error` in JS |
| Mutations | Always return the **fresh snapshot** so the UI can re-apply state |
| Errors | Go `error` text is shown verbatim in a status line; HTTP uses `{"error":"…"}` + 4xx/5xx |
| Time | `"HH:MM:SS"` local, rendered as-is (tabular nums in CSS) |
| IDs | Integers, monotonically increasing per run (`log.id`, `log.version`) |
| Secrets | Never returned in a snapshot in plain text unless the field is a password input target |

## 2. Starter contract (implement these first)

| Method | Arg | Returns | Purpose |
|---|---|---|---|
| `GetSnapshot()` | — | `Snapshot` | Everything the UI renders with |
| `SaveSettings(section, values)` | `section: string`, `values: object` | `Snapshot` | One save path for all sections |
| `TestConnection(conn)` | `{enabled, scheme, entries}` | `{results:[Result]}` | Per-line connectivity verdicts |
| `TestAPI(credentials)` | `{base_url, api_key, model, temperature}` | `string` | Human-readable "200 OK · 12 tokens" |
| `StartRun(payload)` | `{profile: string}` | `{ok, message}` | Begin the job |
| `StopRun()` | — | `{ok}` | Cancel the job, idempotent |
| `LaunchLogin(payload)` | `{profile, backend, headless}` | `{ok, message}` | Open a browser for a manual login |
| `GetLogs(after)` | `after: int` | `{version, entries:[LogEntry], cleared?, running?}` | Versioned log poll |
| `ClearLogs()` | — | `{ok}` | Truncate the feed |
| `OpenDataFolder()` | — | `string` | Absolute path (and open it in the OS) |
| `LoadFolder()` | — | `{count, names:[string]}` | Read sequences/prompts from the data folder |

### Shapes

```jsonc
// Snapshot
{
  "settings": {
    "setup":       { "name": "…", "mode": "balanced", "concurrency": 2, "timeout_seconds": 60, "headless": false, "notes": "…" },
    "connections": { "enabled": false, "scheme": "http", "entries": "host:port\n…" },
    "credentials": { "base_url": "https://api.example.com/v1", "api_key": "", "model": "…", "temperature": 0.8 },
    "actions":     { "strategy": "single|folder|model", "sequence": "line\nline", "targets": "@a\n@b" },
    "timings":     { "poll_min": 5, "poll_max": 12, "action_min": 3, "action_max": 8,
                     "timeout_min": 60, "timeout_max": 90, "retry_min": 10, "retry_max": 30 },
    "profile":     { "name": "default", "backend": "rod", "headless": false }
  },
  "strategy": "single",
  "stats":    { "profiles": 2, "completed": 1, "queued": 0, "failed": 0 },
  "items":    [ { "name": "alpha", "profile": "default", "state": "ok", "updated": "10:02:11" } ],
  "runtime":  { "running": false, "kind": "wails|http|mock", "data_dir": "/abs/path", "version": "1.0" }
}

// Result (TestConnection)
{ "entry": "host:port", "ok": true, "latency_ms": 41, "message": "ok" }

// LogEntry
{ "id": 42, "time": "10:02:11", "kind": "in|out|skip|system|error",
  "text": "the line that was sent or received", "meta": "rule: first · detail: 2s" }
```

`kind` drives the CSS class (`log-out`), the badge label (`OUT`) and the filters. Adding a
new kind means: CSS badge rule + `logKindLabel()` + `logFilterMatch()` + docs. Prefer
reusing the five.

## 3. Mapping to the original ChitChat methods

When a derived app needs more sections than the starter, extend by **same shape**, not by
new patterns:

| Starter concept | Original method(s) |
|---|---|
| `SaveSettings("setup", …)` | `SaveShowModeSettings`, `SaveGeneralSettings`, `SaveUnifiedSettings` |
| `SaveSettings("connections", …)` | `SaveProxySettings`, `SaveBrowserSettings` |
| `SaveSettings("actions", …)` | `SaveMessageSettings` |
| `TestConnection` | `TestProxyEntries(raw)` |
| `TestAPI` | `TestAPIChat(settings)` |
| `StartRun` / `StopRun` | `StartBot` / `StopBot`, `RunBrowser`, `RunFreshBrowsers` |
| `GetLogs(after)` | `GetShowLiveLog(after)`, `GetBotLiveLog(after)` |
| `ClearLogs` | `ClearLogs` |
| `OpenDataFolder` | `OpenEVAChatDataFolder`, `OpenAPIChatDataFolder` |
| `LoadFolder` | `LoadMessageFolder`, `CleanProfiles`, `RemoveProfileData` |
| `GetSnapshot` | `GetInitialState`, `ListProfiles`, `GetLocalTestChatState` |

Do **not** port the original's per-purpose save methods into a new project; one
`SaveSettings(section, values)` with a typed payload is easier to keep consistent. If your
sections diverge a lot, use one method per section but keep the "return the fresh snapshot"
rule.

## 4. HTTP mapping (browser mode)

```
GET  /api/GetSnapshot
POST /api/SaveSettings          body: {"section":"setup","values":{...}}
POST /api/TestConnection
POST /api/TestAPI
POST /api/StartRun
POST /api/StopRun
GET  /api/GetLogs?after=0       (or POST {"after":0})
POST /api/ClearLogs
POST /api/OpenDataFolder
POST /api/LoadFolder
```

Notes:
- `SaveSettings` takes two values, so wrap them: `{"section": "...", "values": {...}}`.
  (In Wails the binding is called as `SaveSettings("setup", {...})` — the bridge passes
  `args[0]`, so for HTTP either send the wrapped object or give the endpoint a dedicated
  handler that unpacks it. Pick one and document it in your project README.)
- Serve the frontend from the same origin (`/` → `index.html`), so `fetch("/api/…")` never
  needs CORS.
- Errors: `{"error":"message"}` + status 400 (validation) / 500 (unexpected).
- Add `Cache-Control: no-store` on API responses.

## 5. Events (Wails only, optional everywhere)

| Event | Payload | Consumer |
|---|---|---|
| `ui:log` | `LogEntry` or `{level, message}` | triggers an immediate `refreshLogs()` |
| `ui:running` | `bool` | pill + badge + button disabled states |
| `ui:showlog` / `ui:botlog` | — | "new lines available" ping |

The UI must work when **zero** events arrive (polling covers it). Never put state that only
exists in an event payload.

## 6. Contract tests to write (cheap, catch real bugs)

```js
// tests/contract.test.cjs — run with `node --test`
test("every bridge method exists on the mock", () => {
  const methods = ["GetSnapshot","SaveSettings","TestConnection","TestAPI","StartRun",
                   "StopRun","LaunchLogin","GetLogs","ClearLogs","OpenDataFolder","LoadFolder"];
  for (const m of methods) assert.equal(typeof window.__consoleMock[m], "function", m);
});
```

Go side: one table-driven test that round-trips each `Save*` → `Load*` pair and asserts the
normalisation (invalid enum becomes the default), plus a test that `GetLogs(after)` never
repeats an entry.

## 7. Versioning

Put a `version` string in `runtime` (`"design-kit-starter"`, `"chitchat-1.0"`, …).
The UI shows it in the sidebar pill tooltip. Clients must tolerate **unknown** fields
(`?.` everywhere) — additive changes never bump the contract.

## 8. Forbidden in the contract

- Returning HTML fragments or pre-formatted log strings with markup (the UI escapes).
- Passing file paths the UI then fetches directly (the backend owns file access).
- Long-running synchronous calls (>2 s) without emitting progress — either stream a log
  line per step or return a handle and poll `GetLogs`.
- Sending the entire log history on every poll.
- Mutating methods that return `null` instead of the snapshot.
