# Design Kit — ChitChat Console UI (reusable for any site)

A complete, documented extraction of the **visual + architectural design** of the ChitChat
dashboard: a dark, sidebar-first desktop console built with **Go + Wails v2 + plain
HTML/CSS/JS**, storing settings in **BadgerDB**, streaming a **live log feed** over Wails
events, and shipping a **browser-runnable** frontend.

The goal of this kit: any agent (or human) can read it and rebuild **~70% of a new
dashboard** immediately — layout, tokens, components, routing, form persistence, bridge
layer, settings storage, live logs — then spend the remaining **30%** on what is genuinely
new for the target site.

> ⚠️ This kit is **design + architecture only**. It contains **none** of the original
> product's behaviour: no chat bot logic, no persona prompts, no target site selectors,
> no automation scripts, no credentials. Domain names/"chitchat" appear only as historical
> labels inside `reference/` (read-only source of truth for the design).

---

## 60-second start

```bash
# 1. Look at the design immediately — no toolchain needed
cd starter
python3 -m http.server 8080
# open http://localhost:8080        (runs on an in-page mock backend)

# 2. Read the docs in order (see "Reading order" below)

# 3. Build the real thing: copy starter/ into a Wails project and follow 07 + 08
```

The starter is **fully clickable**: two modes, five sub-sections each, working forms,
connection tester with per-line results, run/stop, a live log feed that filters by kind,
stat capsules, toasts, and three themes (forest / ocean / light).

---

## What's in the box

| Path | What it is |
|---|---|
| `README.md` | This file — index + orientation |
| `AGENTS.md` | Drop-in instruction file for coding agents working on a derived project |
| `00-START-HERE.md` | Reading order, 70/30 rule, decision tree |
| `01-AGENT-BRIEF.md` | One-page brief an agent must obey (hard rules + definition of done) |
| `02-ARCHITECTURE.md` | Layers, data flow, data directory, why it is built this way |
| `03-TECH-STACK.md` | Exact language/tool versions, dependencies and what each is for |
| `04-UI-DESIGN-SYSTEM.md` | Tokens, type scale, spacing, every component with markup |
| `05-DASHBOARD-SPEC.md` | Panel-by-panel spec: nav, forms, stats, run card, logs |
| `06-FRONTEND-PATTERNS.md` | The 10 JS patterns (routing, formStore, bridge, logs, …) with code |
| `07-BACKEND-PATTERNS.md` | Go `App` struct, mutex, emits, storage keys, HTTP fallback |
| `08-API-CONTRACT.md` | Method-by-method contract (Go ↔ JS ↔ mock ↔ HTTP) |
| `09-FILE-MAP.md` | Every file/dir of the original, with keep/replace guidance |
| `10-REBRAND-FOR-NEW-SITE.md` | The 30% playbook: what to change for a different site |
| `11-BUILD-RUNBOOK.md` | Install, build, test, package, troubleshoot |
| `12-CHECKLIST-70-30.md` | Tick-list to verify parity before shipping |
| `13-AGENT-PROMPTS.md` | Copy-paste prompts for build / adapt / review |
| `design-tokens.json` | Machine-readable tokens (3 themes) for codegen |
| `extras/app-skeleton.go` | Go starting point: `App` struct + all 11 contract methods stubbed |
| `extras/contract.test.cjs` | `node --test` parity check (JS ↔ mock ↔ Go ↔ HTML ids) |
| `starter/` | **Runnable generic dashboard** — the 70% base to copy |
| `reference/` | Original source files (`frontend/dashboard-*`, `frontend/studio-*`, `go/*`), read-only truth — see `reference/README.md` |

---

## Reading order

1. `00-START-HERE.md` — 5 min
2. `01-AGENT-BRIEF.md` — the rules you must not break
3. `04-UI-DESIGN-SYSTEM.md` + open `starter/index.html` side by side
4. `05-DASHBOARD-SPEC.md` — the information architecture
5. `06-FRONTEND-PATTERNS.md` + `08-API-CONTRACT.md`
6. `07-BACKEND-PATTERNS.md`
7. `10-REBRAND-FOR-NEW-SITE.md` — only once the 70% runs
8. `12-CHECKLIST-70-30.md` — before you call it done

---

## The 70 / 30 rule (short version)

**70% = copy from `starter/`**
shell layout · sidebar + nav + subnav + subpanels · panel header with badge + actions ·
stat capsules · card + numbered step head · `form-grid` · radio-card strategy switch ·
min/max timing pairs · one-line-per-entry + Test button + result list · secret field ·
big Run button + status line · live log feed + filter + clear · toasts · draft-safe form
store · single `invoke()` bridge (Wails / HTTP / mock) · settings snapshot + apply ·
data folder button · three themes.

**30% = write for the new site**
brand name/initials · the two mode names · the five sub-section names · the actual
settings structs · what "Test" tests · what "Run" runs · the log entry vocabulary ·
the data folder contents · any site-specific table columns.

---

## Non-negotiable design rules

1. **Sidebar-first, dark, calm.** Muted green accent on near-black; no gradients, no glow,
   no rounded-3xl cards. Radius 8–12px only.
2. **Two modes maximum** at the top level. Each mode has exactly five sub-sections:
   Setup · Connections · Actions · Run · Live Logs (names may change, count should not).
3. **One card = one job**, with a numbered step head (`1 · Setup`).
4. **Never nest cards.** Wide content uses `card-grid`, not a card inside a card.
5. **Every action reports back.** A click either changes a status line, appends a log row,
   or fires a toast — never silence.
6. **Every input is a real, labelled control** in a `form-grid`; no placeholder-only inputs.
7. **Secrets are `type="password"`,** never logged, never rendered raw.
8. **Logs are append-only and kind-tagged** (`in`, `out`, `skip`, `system`, `error`);
   never re-render the world on every event — poll/replace the list only.
9. **Drafts survive.** Switching sub-panels and background polling must not wipe typed
   values (see `formStore` + `dirtyFields`).
10. **The UI must run without Go.** Mock backend + `demo-banner` so design review,
    screenshots and unit tests never need a toolchain.

---

## License / provenance

These files are a re-organisation of source you supplied (`ChitChat-App-clean-fixed-2026-10-20`).
The design and structure are yours to reuse; the original business logic was deliberately
excluded. Keep this note with the kit.
