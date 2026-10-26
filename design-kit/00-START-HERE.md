# 00 · START HERE

## What you are holding

A design-and-architecture kit extracted from a working desktop dashboard
(Go + Wails v2 + vanilla HTML/CSS/JS + BadgerDB + live event log). It is enough for an
agent to rebuild the same **look, structure and wiring** for a completely different site,
without inheriting the original product's domain logic.

Two things you can do right now:

| I want to… | Go to |
|---|---|
| See the design in a browser, click around | `starter/` → `python3 -m http.server 8080` |
| Read the exact original code that produced it | `reference/frontend/` |
| Hand the job to an AI agent | `01-AGENT-BRIEF.md` + `13-AGENT-PROMPTS.md` |
| Understand the whole system in one page | `02-ARCHITECTURE.md` |
| Change it for my own site | `10-REBRAND-FOR-NEW-SITE.md` |

## The core idea: 70 / 30

```
┌──────────────────────────────────────────────────────────────────────────┐
│  70%  REUSE — identical for every site                                    │
│  shell · sidebar · 2 modes × 5 sections · cards · form-grid · statuses    │
│  radio-card strategies · min/max pairs · entry-test-result blocks ·       │
│  run card · live log feed + filters · stats · toasts · draft store ·      │
│  invoke() bridge · snapshot/apply · themes · docs                         │
│                                            copy from starter/ verbatim    │
├──────────────────────────────────────────────────────────────────────────┤
│  30%  INVENT — different for every site                                   │
│  brand · mode names · section names · settings structs · what Test/Run    │
│  actually do · log vocabulary · data folder · extra list columns          │
└──────────────────────────────────────────────────────────────────────────┘
```

Rule of thumb while working: **if you are writing CSS, you are doing the 70% wrong** —
the stylesheet is already complete. If you are writing a Go handler or a new settings
field, you are in the 30% — go ahead, but add it in the same shape as the existing ones.

## Decision tree before you start

1. **Does the target need a desktop shell?**
   - Yes → Wails v2 (as in `reference/go/chitchatui-main.go`). Same frontend, same JS.
   - No, web is fine → keep `starter/` as-is and serve it from any backend; the bridge's
     HTTP mode (`CONFIG.backend = "http"`) is already implemented.
2. **Does the target need a local DB?**
   - Settings + logs only → BadgerDB exactly as in `reference/go/uistore-store.go`.
   - Only a handful of values → JSON files in `data/` are enough; keep the same keys.
3. **Does the target need a browser automation step?**
   - Yes → Rod / chromedp (see `03-TECH-STACK.md` § "Browser automation").
   - No → delete the "Account / Login capture" section and keep four sections.
4. **Does the target need an LLM call?**
   - Yes → reuse the OpenAI-compatible `/chat/completions` client pattern
     (`reference/go/openai-client.go`) behind a `TestAPI` handler.
   - No → drop the credential card.

## The five-minute tour of the starter

```
starter/
├── index.html        sidebar (brand, nav, subnav, footer pills)
│                     ├─ Mode "Studio" → Setup · Connections · Actions · Run · Live Logs
│                     └─ Mode "Ops"    → Account · Connections · Actions · Settings · Logs
├── styles.css        the ORIGINAL dashboard stylesheet (1323 lines, untouched design)
│                     + a marked "STARTER ADDITIONS" block (themes, toast, table, empty state)
├── app.js            ~640 lines of generic shell logic (routing, store, bridge, logs)
├── js/
│   └── mock-backend.js   same method names as the Go App; runs with no toolchain
```

Every element has a job and is reused at least twice across the two modes. If you add a
new visual idea, add it as a component in `04-UI-DESIGN-SYSTEM.md` first, then implement
it once and reuse it.

## What is deliberately NOT here

- Chat/persona logic, "director" scoring, message sequences, share-turn behaviour
- Snapchat/Telegram selectors, DOM automation scripts, anti-bot handling
- API keys, usernames, tokens, proxy credentials (the reference files show *field names*
  only; values are empty or placeholders)
- The verification/test harness of the original (it tested bot behaviour) — the design
  kit's equivalent is the mock backend + the jsdom smoke test described in §
  `11-BUILD-RUNBOOK.md`

## Definition of "I understood the kit"

You can answer these without opening a file:

1. How many top-level modes, and how many sub-sections per mode? → 2 and 5.
2. Where does the frontend get its data? → one `invoke()` call site, three possible
   transports (Wails binding / HTTP `/api` / in-page mock).
3. What happens to unsaved input when the user switches sub-panels? → kept in
   `formStore`, never lost, never overwritten by polling (`dirtyFields`).
4. Where do settings live on disk? → `data/ui/badger` (keys `settings/*`), with a JSON
   mirror for cross-process sync.
5. How does a log row get rendered? → poll `GetLogs(afterVersion)` → append → render
   filtered list, keep scroll pinned near the bottom.
