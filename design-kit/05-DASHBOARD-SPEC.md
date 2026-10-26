# 05 · DASHBOARD SPEC (information architecture)

The original dashboard has **two modes**, each with **five numbered sections**. Every
section is a sub-panel: only one is visible at a time, all of them stay in the DOM.
This document is the full inventory, original → generic, so you can map your site onto it.

```
SIDEBAR
├── brand                     initials + product name + one-line subtitle
├── nav (2 modes)             Studio · Ops
├── subnav (5 per mode)       numbered in the card titles, not in the buttons
└── footer                    Workers pill (Stopped/Running) · backend pill
MAIN
├── panel header              h1 + mode badge + one sentence + 1–3 actions
├── (stats row)               only on modes that own a running job
└── 5 sub-panels              one card (or card-grid) each
```

---

## 1. Original inventory (what the ChitChat build actually had)

### Mode 1 · Show Mode — "visible browser does the work"

| # | Section | Content | Backend |
|---|---|---|---|
| 1 | Setting | First-message policy (`bot`/`user`/`random`), bot-first chance %, wait strategy (`unlimited`/`skip`), first timeout min/max, reply delay min/max, auto-skip checkbox, no-reply timeout min/max, next-chat wait min/max | `SaveShowModeSettings` |
| 2 | Proxy | enable checkbox, default scheme (http/socks5), proxy lines textarea, **Test Proxy**, per-line results | `SaveProxySettings`, `TestProxyEntries` |
| 3 | Message | chat-mode radio cards (Single / Folder / EVAChat Flow / API Chat), sequence textarea, Load Folder + Clean Loaded + counter, Open data folder, Snap/TG usernames textarea, Save Message Mode | `SaveMessageSettings`, `LoadMessageFolder`, `Open*DataFolder` |
| 4 | Run Browser | profile radio (default / session), big **▶ Run Browser**, status line, hint | `Save*` then `RunBrowser` |
| 5 | Live Logs | filter (All/SMS/Rules/Errors), Clear, feed | `GetShowLiveLog(after)` |

### Mode 2 · Background Bot — "headless workers"

| # | Section | Content | Backend |
|---|---|---|---|
| 1 | Login Capture | profile name + datalist of saved profiles, browser mode (Rod/ChromeDP), headless checkbox, **Launch for Login**, status | `ListProfiles`, `RunBrowser` |
| 2 | Proxy | identical to Show Mode §2 | shared handler |
| 3 | Message | identical to Show Mode §3 (no EVAChat-only bits) | shared handler |
| 4 | Bot Settings | first-message policy again, poll min/max, action min/max, skip timeout min/max, chat-closed wait min/max, worker reply delay min/max | `SaveGeneralSettings` |
| 5 | Live Logs | same component, different feed | `GetBotLiveLog(after)` |

Header for Mode 2 additionally carries **Save & Start Bot**, **Stop**, **Refresh** and a
stats row: Running · Profiles · Messages sent · Traffic users · snap/tg sent · Dead accounts.
Plus a **studio window** (`apichat.exe`) that shares the same stores: three tabs
(Chat Test & Logs · Persona Prompt · Settings) with a banner confirming the sync.

### Why the duplication (Proxy/Message appear twice)

Both modes use the same settings stores; the second mode is a peer, not a child. The
duplication is deliberate: a user configuring the headless worker never has to go back to
the other mode. The **code** is shared via `fillProxy(prefix, …)` / `fillMessage(prefix, …)`
with a `sm` / `bg` id prefix — that is the pattern to copy, not the markup.

---

## 2. Generic spec to implement (matches `starter/`)

### Mode A · Studio — "configure, run once, watch"

| # | Section | Required content | Methods |
|---|---|---|---|
| 1 | **Setup** | 2–4 identity fields (name, mode select, concurrency, timeout), one boolean, one free-text notes field, Save + status | `SaveSettings("setup", …)` |
| 2 | **Connections** | enable toggle, default scheme, one-line-per-entry textarea, **Test** + status + per-line results; plus a credential card (base URL, API key, model, temperature, **Test API**) | `SaveSettings("connections"/"credentials")`, `TestConnection`, `TestAPI` |
| 3 | **Actions** | strategy radio cards with one revealed block per strategy (sequence / folder / model), Load Folder + Clean + counter, targets textarea, Save; below it a **Recent items** table | `SaveSettings("actions")`, `LoadFolder`, `OpenDataFolder` |
| 4 | **Run** | profile radio (default / saved session), **▶ Run**, status line, one hint; badge in the header mirrors state | `Save*` ×4 → `StartRun` |
| 5 | **Live Logs** | kind filter, Clear, feed, empty hint | `GetLogs(after)`, `ClearLogs` |

### Mode B · Ops — "always-on worker"

| # | Section | Required content |
|---|---|---|
| 1 | **Account** | profile text + datalist, browser-mode select, headless toggle, **Launch for Login**, status |
| 2 | **Connections** | same component as Studio §2 (entry test only) |
| 3 | **Actions** | sequence textarea, targets textarea, Save |
| 4 | **Settings** | four min/max timing pairs + Save + status |
| 5 | **Live Logs** | same component, own feed |

Header: **Save & Start**, **Stop**, **Refresh**, plus a one-line `ops-summary` and an
`ops-status` line directly under the header.

---

## 3. Section rules (apply to every derived dashboard)

1. **Number the cards, not the buttons.** Card titles read `1 · Setup`; subnav buttons read
   `Setup`. The number is the user's progress map.
2. **Each section saves itself** (own button + own status line) *and* the Run button saves
   everything first. Never rely on one global Save for a five-step flow.
3. **The Run section is last but one.** Logs are always section 5, and always exist even
   before anything has run (empty hint, not a blank box).
4. **Every mode owns its own job**, its own logs and its own Run/Stop. Shared settings are
   shared by *store*, not by *mode*.
5. **Stats belong to the mode that runs a job.** A mode with nothing running shows no caps.
6. **Header actions are state machines**: Refresh always enabled; Start disabled while
   running or busy; Stop disabled unless running.

## 4. Content inventory per card (minimum viable)

| Card | Minimum fields | Buttons | Feedback |
|---|---|---|---|
| Setup | 3 inputs + 1 select | Save | status line |
| Connections | 1 textarea + 1 toggle | Test | status + per-line list |
| Credentials | 3 inputs + 1 number | Test API | status |
| Actions | 1 radio group + 1 textarea | Save, Load, Open | status + counter |
| Run | 1 radio group | Run (primary, full width) | status + badge |
| Logs | 1 select filter | Clear | feed + empty hint |
| Account | 1 input + 1 select + 1 toggle | Launch | status |
| Settings | 4 × min/max pairs | Save | status |

Anything beyond this list needs a justification in your `NOTES.md` — the design is meant
to stay boring.

## 5. Empty / loading / error states (mandatory)

| Surface | Empty | Loading | Error |
|---|---|---|---|
| Stats | `0` in every capsule | keep last values, never blank the row | `danger` capsule for failures |
| Log feed | `No activity yet. Press Run.` (hint under the list) | keep previous entries, don't flash | `error` rows inline |
| Table | single row `No items yet.` | keep previous rows | badge `err` in the state column |
| Form | seeded defaults from the backend | inputs disabled only while saving | status line red, values preserved |
| Bridge | `demo` pill + banner | `…` pill for <1 s | red status line with the exact message |

## 6. Keyboard / focus map

`Tab` walks the visible sub-panel only (hidden panels are `display:none`, so they are
skipped automatically — that is why sub-panels toggle a class instead of using `hidden`).
`Enter` submits nothing by default (no `<form>` element) — actions are explicit buttons.
Document this in your project README so testers do not expect Enter-to-save.

## 7. Screen-size behaviour

| Width | Behaviour |
|---|---|
| ≥ 1280 | Sidebar 238 px, `subpage` centred at 1100 px, stats on one row |
| 1000–1280 | Same layout, `card-grid` drops to one column |
| 900–1000 | Subnav becomes a wrapping horizontal row under the nav |
| < 900 | Log card unclamps its height; `.form-grid.two-column` stays two columns (it is a desktop app), so test down to 900 px only |
