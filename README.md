# apiichatt3435 — dashboard design kit

Ei repo-te apnar ChitChat dashboard-er **design + architecture** ekta reusable kit hisebe
achhe, jate onno kono site/product-er jonno ekhoni 70% UI+structure build kora jay.

| Path | What |
|---|---|
| **`chitchat-design-kit.zip`** | 📦 **The deliverable** — 42 files, self-contained (docs + runnable starter + reference source) |
| `design-kit/` | Same content, unpacked, so it can be read on GitHub |
| `design-kit/starter/` | Runnable generic dashboard (same design, no domain logic) |
| `ChitChat-App-clean-fixed-2026-10-20 (7).zip` | The original upload (untouched) |

## Quick look

```bash
cd design-kit/starter && python3 -m http.server 8080   # → http://localhost:8080
```

Runs on an in-page mock backend: two modes, five sections each, working forms, connection
tester, run/stop, live filtered log feed, three themes.

## Read in this order

1. `design-kit/README.md` — index + the 70/30 rule
2. `design-kit/00-START-HERE.md`
3. `design-kit/01-AGENT-BRIEF.md` — rules an AI agent must follow
4. `design-kit/04-UI-DESIGN-SYSTEM.md` + `05-DASHBOARD-SPEC.md`
5. `design-kit/06-FRONTEND-PATTERNS.md`, `07-BACKEND-PATTERNS.md`, `08-API-CONTRACT.md`
6. `design-kit/10-REBRAND-FOR-NEW-SITE.md` — the 30% for your new site
7. `design-kit/12-CHECKLIST-70-30.md` before shipping

## Stack the design was built on

Go 1.24 · Wails v2.12 (desktop shell, bindings, events) · BadgerDB v4 (settings + logs) ·
vanilla HTML/CSS/JS frontend (no framework, no bundler) · Rod/chromedp (browser automation,
optional) · `node --test` + jsdom (verification).

Nothing of the original product's behaviour (chat bot, persona, targeting) is included —
only the design, the wiring patterns and the contracts.
