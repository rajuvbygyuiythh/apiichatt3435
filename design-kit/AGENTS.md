# AGENTS.md — working rules for this repository

> Drop-in instruction file. If you are an AI agent or a new contributor, read this before
> your first edit. It is deliberately short and prescriptive.

## 0 · What this project is

A desktop dashboard built with **Go + Wails v2** and a **vanilla HTML/CSS/JS** frontend.
Its design comes from the console design kit (`design-kit/` in the source drop, integrated
into `cmd/<app>/ui/` here). The visual language is **fixed**; the domain is ours.

## 1 · Read first, in this order

1. `docs/ARCHITECTURE.md` — layers + data flow (or `design-kit/02-ARCHITECTURE.md`)
2. `docs/UI-DESIGN-SYSTEM.md` — tokens, components, states
3. `docs/DASHBOARD-SPEC.md` — the 2 modes × 5 sections
4. `docs/API-CONTRACT.md` — every method the UI calls
5. `CHECKLIST.md` — what "done" means

## 2 · Hard rules

1. **`cmd/<app>/ui/styles.css` is frozen.** Add new rules only at the end, under a
   `/* ---- <feature> additions ---- */` banner, using existing tokens. No hex literals
   outside `:root`, no new framework, no utility classes.
2. **Two modes, five sections each.** New functionality becomes a field, a card, or a
   sub-section — never a third mode, never a modal-heavy flow.
3. **One bridge.** All backend calls go through `bridge.invoke("Method", arg)` in
   `app.js`. `fetch` and `window.go` may only appear inside `resolveBridge()`/`httpInvoke()`.
4. **One source of truth per setting.** A field appears exactly once in HTML, once in
   `read<Section>()`, once in `applySnapshot()`. Duplication = bug.
5. **Mutations return the snapshot.** Any save/start/stop returns the fresh state; the UI
   re-applies it instead of guessing.
6. **Never swallow errors.** `setStatus(id, err.message, "error")` for the user,
   `reportFrontendError()` for the log file. Empty `catch {}` blocks are rejected.
7. **Secrets** are `type="password"`, stored locally only, never logged, never echoed.
8. **Every generated artifact stays out of git**: `data/`, `build/`, `dist/`, `*.exe`,
   `verification-output/`, `node_modules/`.
9. **No domain logic in the UI.** The frontend renders state and sends intent. Rules live
   in `internal/`.
10. **Do not copy behaviour from the reference product** in `design-kit/reference/` —
    design and structure only. No prompts, scripts, selectors or targeting lists.

## 3 · Code standards

| Area | Standard |
|---|---|
| Go | gofmt; `fmt.Errorf("...: %w", err)`; exported methods = UI API; unexported helpers; `log.Fatal` only in `main`; file header comment stating the file's one job |
| JS | 2-space indent, double quotes, semicolons, `const`/`let`; named functions; `textContent` for user data, never `innerHTML`; comment every non-obvious *why* |
| CSS | tokens only; ordered sections; kebab-case semantic class names; one breakpoint (900 px) |
| HTML | native controls, visible labels, `role="status"` + `aria-live` on status/log surfaces |
| Docs | one file per subsystem in `docs/`; update the doc in the same commit as the change |
| Commits | one logical change; message = `area: what changed` (e.g. `ui: add retry timing pair`) |

## 4 · Verification (run before you claim done)

```bash
go vet ./...
go test ./...                       # tests use t.TempDir(), never data/
node --check cmd/<app>/ui/app.js
node --test tests/                  # contract + jsdom UI tests
node --test extras/contract.test.cjs   # method parity (JS ↔ mock ↔ Go ↔ HTML ids)
```

Then the manual pass, and only if authorised: launch the desktop app, save every section,
restart, confirm persistence, run + stop, confirm log streaming.

**Never** mark a check PASS if you did not execute it. Use `NOT RUN`. Keep
PASS / FAIL / BLOCKED / NOT_RUN distinct in reports.

## 5 · Where things live

```
main.go                 embed + wails.Run wiring (window size/colour, webview dir)
app.go                  App struct: exported methods = UI API, mutex, emit, log
internal/model/         settings structs, Default*(), Snapshot
internal/uistore/       Badger: settings/* + logs/*
internal/applog/        disk logger with redaction
internal/<domain>/      our actual logic
cmd/<app>/ui/           index.html · styles.css · app.js  (the design)
tests/                  node --test + jsdom
docs/                   one page per subsystem
data/                   runtime state (gitignored)
```

## 6 · Common pitfalls (learned the hard way)

- Second copy of the exe fails with `0x800700AA` → `WebviewUserDataPath` must live in
  `data/webview`, not `%APPDATA%`.
- White flash on launch → `BackgroundColour` must equal the CSS `--bg`.
- Build fails because the exe is locked → close the running app.
- Typed values revert after ~3 s → polling overwrote a dirty field; check `dirtyFields`.
- Duplicate log rows → the cursor (`bucket.after`) isn't advanced or `pending` guard missing.
- Buttons dead after an error → `setBusy` not released in `finally`.
- Badger locked on start → another process (or a second window) holds `data/ui/badger`.

## 7 · When requirements conflict

Prefer, in order: (1) the design kit, (2) the existing file's conventions, (3) your own
taste. If a requirement genuinely cannot fit the 2×5 structure, write the reason in
`NOTES.md` and ask before building a new pattern.
