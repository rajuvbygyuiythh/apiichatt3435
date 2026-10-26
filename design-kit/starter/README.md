# starter/ — the runnable 70%

A generic, dependency-free dashboard that **is** the design: same shell, same components,
same behaviour as the original console, with every domain string removed.

```bash
python3 -m http.server 8080     # open http://localhost:8080
```

You should see: sidebar (brand, 2 modes, 5 sections each, Workers/Backend pills), a panel
header with a badge, five stat capsules, and a yellow "Demo backend active" banner. Press
**Run** — log rows start appearing every ~1.1 s. Switch to **Ops** — its own subnav, forms
and log feed.

## Files

| File | Lines | Role |
|---|---|---|
| `index.html` | ~380 | Structure: 2 modes × 5 sub-panels, every component used twice |
| `styles.css` | ~1320 | The original stylesheet (unchanged) + a marked additions block (themes, toast, table, empty state, badges, step head) |
| `app.js` | ~660 | Shell logic: routing, form store, bridge, snapshot apply, actions, log feed, boot |
| `js/mock-backend.js` | ~230 | Fake backend with the exact method names of the Go `App` |

## The three transports

`app.js` decides once at boot:

```
window.go.main.App present?          → "wails"   (inside the desktop shell)
fetch /api/GetSnapshot answers?      → "http"    (served by any backend)
window.__consoleMock present?        → "mock"    (this folder, no backend)
```

The sidebar pill shows which one is active. Set `CONFIG.backend` in `app.js` to force one
(`"wails" | "http" | "mock"`), which is handy for testing the HTTP path.

## What to change first

1. `CONFIG.brand` (and the same three strings in `index.html`) — name, initials, subtitle.
2. `CONFIG.apiPrefix` if your API is not at `/api`.
3. The `read<Section>()` functions in `app.js` + the matching form fields.
4. The five section names (sidebar button, card `h2`, `data-subpanel` attribute).
5. `js/mock-backend.js` — keep it while designing, delete it when the real backend exists
   (also delete its `<script>` tag and the `demo-banner` markup, or gate it behind `?demo`).

Everything else (routing, store, bridge, log rendering, statuses, themes) is meant to stay.

## Conventions baked in

- `data-tab` on `.nav-button` = mode key; `data-panel` on `.tab-panel` = same key.
- `data-subpanel` on `.subnav-button` and on `.subpanel` = the section id.
- Section ids are prefixed by mode (`studio-…`, `ops-…`) so shared helpers can take a prefix.
- Every input has an `id`; those are the only handles the JS uses.
- Two placeholders conventions in the copy: `%username%` (single value) and `{hello|hi}`
  (variant pick) — purely illustrative, replace with your own.

## Verify it still works

```bash
node --check app.js && node --check js/mock-backend.js
node --test ../extras/contract.test.cjs        # method parity + id parity + no leaked secrets
```

The contract test checks that every `bridge.invoke("X")` has a counterpart in the mock, in
the Go skeleton and in the HTML ids — the cheapest way to catch a rename.

## Do not

- Do not add a bundler, framework, or CSS library.
- Do not edit rules above the `STARTER ADDITIONS` banner.
- Do not call `fetch` outside `httpInvoke()`.
- Do not render user or log text with `innerHTML`.
