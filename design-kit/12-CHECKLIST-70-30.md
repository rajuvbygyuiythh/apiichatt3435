# 12 · CHECKLIST — 70% parity / 30% done

Copy this into your project as `CHECKLIST.md` and tick it honestly. Mark anything you did
not run as `NOT RUN`, never as done.

---

## A · The 70% (must be identical to the kit)

### Structure
- [ ] `styles.css` copied **verbatim**; new rules only under a banner at the end
- [ ] Sidebar: brand mark (2 chars) + title + subtitle, exactly 2 nav buttons
- [ ] Exactly 5 subnav buttons per mode; inactive subnav uses the `hidden` attribute
- [ ] Sidebar footer: running pill (+ optional backend/health pill)
- [ ] Panel header: `h1` + mode badge + one-sentence description + 1–3 actions
- [ ] Stats row only on modes that run a job, 5–6 capsules, ≤1 `danger`
- [ ] Every section = one card (or `card-grid` of siblings), numbered step head 1–5
- [ ] No card nested inside a card
- [ ] `subpage` max 1100 px, `subpage narrow` 760 px for single-column forms

### Components
- [ ] `form-grid` with a visible `<span>` label for every control
- [ ] Units in labels (`Timeout (s)`), min/max pairs side by side
- [ ] Strategy switch = `mode-option` radio cards, one revealed block each
- [ ] One-line-per-entry textarea → Test button → per-line PASS/FAIL/WARN rows
- [ ] Secrets in `type="password"`, never echoed to logs/status/toasts
- [ ] Run card: profile radio group + full-width primary + status line + hint
- [ ] Live log card: filter select, Clear, fixed-height scroll list, empty hint
- [ ] Log row = `time | kind badge | text + optional meta`; five kinds supported
- [ ] Status line per card (max one), `.status-ok` / `.status-error`
- [ ] Toast only as a bonus confirmation, auto-dismiss 4.2 s
- [ ] Table only for item lists; badges only in the state column
- [ ] Empty / loading / error states for stats, logs, tables, forms

### Behaviour
- [ ] `bridge.invoke()` is the only backend call site
- [ ] Transport resolution: Wails → HTTP probe → mock, with a visible `wails/http/demo` pill
- [ ] `formStore` restores drafts when switching sub-panels
- [ ] `dirtyFields` protects typed values from 3 s polling
- [ ] Save returns and re-applies the fresh snapshot
- [ ] Run saves every section before starting
- [ ] `setBusy` disables the primary button and is always released in `finally`
- [ ] Log poll uses a version cursor, a `pending` guard, and pins scroll only near the bottom
- [ ] Frontend exceptions reach `logs.txt` (`window.onerror` + `unhandledrejection`)
- [ ] Boot retries the bindings (~5 s) instead of showing a blank window

### Quality
- [ ] Works at 1280×820 and 900 px wide
- [ ] All three themes (`forest`, `ocean`, `light`) complete and correct
- [ ] Keyboard: Tab reaches every control in the visible panel only; focus visible
- [ ] Contrast ≥ 4.5:1 for text, ≥ 3:1 for borders/UI
- [ ] No console errors on a full click-through of both modes
- [ ] `node --check` clean on every JS file; `go vet ./...` clean

### Housekeeping
- [ ] `data/`, `build/`, `dist/`, `*.exe` gitignored
- [ ] `reference/` not shipped in the derived repo
- [ ] `AGENTS.md` (repo rules) present and accurate
- [ ] `docs/` has one page per subsystem you actually built

---

## B · The 30% (site-specific, must be *decided*, not inherited)

- [ ] Brand: name, initials, subtitle, window title, exe name — no original strings left
  (`grep -ri "chitchat\|persona\|snap\|evachat"`)
- [ ] Mode names chosen and justified in `NOTES.md`
- [ ] Five section names chosen; step numbers match the subnav order
- [ ] Settings structs defined with JSON tags + `Default*()` constructors
- [ ] Storage keys added (`settings/<section>`) and load/save round-trip tested
- [ ] `read<Section>()` + matching `setValue()` lines for every field (1:1:1)
- [ ] "Test" actually tests something real and returns per-entry verdicts
- [ ] "Run" starts a real job and returns in <2 s; progress arrives as log rows
- [ ] "Stop" cancels work, is idempotent, and always clears the running state
- [ ] Log vocabulary reused (in/out/skip/system/error) with domain-appropriate labels
- [ ] Data folder seeded on first run; an Open Folder button exists
- [ ] Table columns match between `thead` and `renderItems()`
- [ ] Credentials read from storage, never from the DOM after load, never logged
- [ ] No original domain logic, scripts, selectors or prompt text was copied

---

## C · Two-sided contract

- [ ] Every JS method exists in Go **and** in the mock (or the mock is deleted and the
  demo path removed)
- [ ] Error shape identical on both paths: Go `error` → thrown `Error` → red status line
- [ ] HTTP mode: `GET` for no-arg, `POST` + JSON body otherwise, `{"error":…}` on failure
- [ ] Events are an optimisation only; the UI works with zero events
- [ ] Method signatures documented in `README.md` of the derived project

---

## D · Sign-off

```
Date        :
Builder     :
70% verified: __ / 40 boxes
30% verified: __ / 14 boxes
NOT RUN     : ................................................
Known gaps  : ................................................
Screenshot  : starter running, both modes, log feed with 3+ kinds
```
