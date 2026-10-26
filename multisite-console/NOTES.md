# NOTES.md — the 30% decisions (step 2, frozen for review)

Filled from `design-kit/10-REBRAND-FOR-NEW-SITE.md` → step 0. Everything in this file is
the **project-specific 30%**. The other 70% is the console design kit, unchanged.

## 0 · Identity

| Slot | Value |
|---|---|
| Product name | **Multi-Site Console** |
| Initials | **MS** |
| Subtitle | `5-site engine · Go + WebView` |
| Window title / exe | `Multi-Site Console` / `multisite-console.exe` |
| Theme | `forest` (ocean + light selectable in the header) |
| Platform | **Wails v2 desktop** (Windows), with a web mode for development/testing |

## 1 · Scope — 5 sites, one per engine family

Source: `rajuvbygyuiythh/fghfghgfhgf` release `v2.0.0-multisite-final` (102 site folders).
ChitChat is **excluded**. One representative per family was taken so the console has to
handle every shape the engine knows about (full analysis: `SITE-ANALYSIS.md`).

| # | Site ID | Name | Family | Account mode |
|---|---|---|---|---|
| 1 | `omezy` | Omezy.io (Text Roulette) | `family_a` | auto_register |
| 2 | `camgo` | Camgo.com (Text & Video) | `family_b` | auto_register |
| 3 | `viby` | Viby.chat (Guest 1-on-1 Text) | `family_c` | auto_register |
| 4 | `chat42` | Chat42.com (Instant Name → 1v1) | `family_d` | auto_register |
| 5 | `chatusa` | ChatUSA.club (Fast Stranger Match) | `family_e` | auto_register |

Each site stays an **isolated folder** (`sites/<id>/site.json`, `dom_driver.js`,
`*_protection.txt`, `accounts/`, `profiles/`). Adding a 6th site = copy a folder; no Go or
UI change. This is the same isolation contract as the source repo.

## 2 · Modes and sections (2 × 5, per the design rules)

### Mode A · **Show Mode** — one visible browser, one site

| # | Section | Content | Backend |
|---|---|---|---|
| 1 | **Site** | site picker cards (5) with live status badge, start URL, family, account mode, entry flow, protection note, **Test Site**, **View DOM Driver** | `TestSite`, `GetSiteDriver` |
| 2 | **Setting** | first-message policy, bot-first %, wait mode, first timeout min/max, reply delay min/max, no-reply min/max, auto-skip, profile | `SaveSettings("show", …)` |
| 3 | **Message** | strategy radio (single / folder / model), sequence, targets, max lines, **Load Folder** | `SaveSettings("message", …)`, `LoadFolder` |
| 4 | **Run** | selected-site summary, session radio (fresh login / saved session), **▶ Run Browser**, status | `SaveSettings` ×2 → `StartRun` |
| 5 | **Live Logs** | filter (All / IN-OUT / Rules / Errors), Clear, feed with site tags | `GetLogs`, `ClearLogs` |

### Mode B · **Worker** — headless, several sites at once

| # | Section | Content |
|---|---|---|
| 1 | **Sites** | multi-select checklist (5) + **Save Selection** + site-state table (family, mode, accounts, session, driver bytes, status) |
| 2 | **Accounts** | auto-register (site, count, gender, age, nickname pool) → writes `sites/<id>/accounts/*.json`; manual login (profile) → prepares `sites/<id>/profiles/<profile>`; table of accounts created |
| 3 | **Message** | same store as Show · Message (second copy of the component, prefixed ids) |
| 4 | **Settings** | poll/action/timeout/retry min-max pairs, concurrency, profile, headless, **Proxy** block with Test |
| 5 | **Live Logs** | same component, same ring |

Header: **Save & Start**, **Stop**, **Refresh** + `worker-summary` line. Stats capsules:
Sites · Enabled · Accounts · Sessions · Running · Failures.

## 3 · Settings structs (Go `internal/core/model.go`)

```go
Settings{
  Show    { site_id, profile, first{ mode, chance, wait_mode, timeout_min, timeout_max,
                                     reply_min, reply_max, no_reply_min, no_reply_max, auto_skip } }
  Worker  { sites[], profile, headless, concurrency, poll/action/timeout/retry min+max }
  Message { strategy, sequence, targets, loaded_sequences[], max_lines }   // shared by both modes
  Proxy   { enabled, scheme, entries }                                     // shared by both modes
  Accounts{ count, gender, age, nickname_pool }
}
```

Persisted as one JSON document: `data/settings.json` (atomic temp+rename, corrupt file is
moved to `.corrupt` and defaults are seeded — never a crash).

## 4 · Method contract (one object argument each)

| Method | Arg | Returns |
|---|---|---|
| `GetSnapshot` | — | `{settings, strategy, sites[], stats, runtime}` |
| `SaveSettings` | `{section, values}` | fresh `Snapshot` |
| `GetLogs` | `{after}` | `{version, entries[], cleared, running, stats}` |
| `ClearLogs` | — | `{ok}` |
| `TestSite` | `{site_id}` | `{entry, ok, latency_ms, http, message}` — real HTTP probe |
| `TestConnections` | `ProxySettings` | `{results:[{entry, ok, latency_ms, message}]}` — real TCP dial |
| `StartRun` | `{mode: "show"\|"worker", sites[], profile}` | `{ok}` |
| `StopRun` | — | `{ok}` (idempotent) |
| `LaunchLogin` | `{site_id, profile}` | `{ok, message, profile_dir}` |
| `AutoRegisterAccounts` | `{site_id, count}` | `{accounts[], count}` |
| `GetSiteDriver` | `{site_id}` | `{dom_driver_js, site_json, protection_note}` |
| `LoadFolder` | — | `{count, names[]}` |
| `OpenDataFolder` | — | `{path}` |

Transport: Wails bindings in the desktop build, `POST /api/<Method>` (GET for
`GetSnapshot`) in web mode, and `js/mock-backend.js` for pure UI work. Same names, same JSON.

## 5 · Log vocabulary (reused from the kit)

| kind | Meaning here |
|---|---|
| `in` | partner message observed in the page |
| `out` | our message sent / an account registered |
| `skip` | deliberately not acted on (disabled site, no-reply timeout, closing and rotating) |
| `system` | lifecycle: run start/stop, onboarding, probe result, settings saved, storage path |
| `error` | probe failure, site skipped from the registry |

Every entry may carry `site` (id) and `meta` (rule / detail), rendered as the second line.

## 6 · Data folder

```
data/                     ← -data flag (default ./data)
├── settings.json         atoms: all sections, one document
├── sequences/*.txt       folder-strategy scripts (Load Folder)
└── logs.txt              (step 3: file log alongside the ring)
sites/<id>/
├── site.json  dom_driver.js  <id>_protection.txt
├── accounts/*.json       auto-registered identities
└── profiles/<profile>/   manual-login sessions
```

## 7 · What step 2 delivers vs what is still simulated

| Area | Step 2 (now) | Later step |
|---|---|---|
| Registry of 5 sites, per-site isolation | **real** (files parsed from `sites/`) | — |
| Settings persistence, validation, normalise | **real** (Go tests) | — |
| Test Site (HTTP probe), Test Proxy (TCP dial) | **real** | richer probes (WS/DOM readiness) in step 3 |
| Auto-register accounts | **real files** written to `sites/<id>/accounts/` | network-side registration in step 4 |
| Start / Stop job | **real** goroutine, pacing from settings, cancel + log | step 4: real Rod browser driver per site |
| Launch login | prepares the profile folder + logs | step 4: opens the browser |
| Message strategies: single / folder | **real** (lines sent by the runner) | step 4: model strategy |
| Wails desktop shell | not yet (web mode + `desktop/` in step 5) | step 5 |

## 8 · Deliberate design decisions

1. **Site = folder, never a flag inside Go.** Adding a site is a copy-paste of a folder;
   a broken `site.json` is skipped with a logged `error` entry, not a crash.
2. **One settings document.** The console's state is small; a JSON file is inspectable and
   trivially testable. (The source project used BadgerDB — same key concept, heavier tool.)
3. **Stdlib-only core.** `internal/core` has zero third-party imports, so `go vet` /
   `go test` / web mode run anywhere, and the Wails shell is a thin extra layer.
4. **Two log views, one ring.** Simpler than two rings; entries carry `site`, so a future
   per-mode ring only needs a filter, not a new store.
5. **Status is computed from disk** (`accounts/`, `profiles/`, `dom_driver.js`), not from a
   saved string, so the badge can never lie after a manual folder edit.
