# 04 · UI DESIGN SYSTEM

The stylesheet in `starter/styles.css` is the design. This document explains **why** each
value exists so you can extend it without breaking the look. Rule: read this before adding
a single CSS declaration.

---

## 1. Character of the visual language

A **dark operations console**: dense but calm, green-tinted neutrals, one accent colour,
zero decoration. It looks like a tool that is already running, not a landing page.

| Principle | Consequence |
|---|---|
| Calm, low-chroma surfaces | Backgrounds are desaturated greens/greys (`#161716`, `#20221e`), never pure black |
| One accent, used sparingly | Muted green `#83cb73` only for: active nav, primary button, stats, OUT log rows, success |
| Borders do the separating | 1 px `--line` borders everywhere; shadows only on toasts |
| Tight radii | 8 px controls, 10–14 px cards, 999 px stat capsules/pills |
| No images, no icons font | Text-only labels; the only glyphs are **▶**, **✓ / ✗**, and the step numbers |
| Numbers are tabular | Log timestamps and stats use `font-variant-numeric: tabular-nums` |

Forbidden in derived projects: gradients, glassmorphism, drop shadows on cards, emoji in
nav, more than one accent hue, light-mode-only designs (light exists as a theme, not the base).

---

## 2. Tokens

All tokens live in `:root`. **Never** use a literal colour in a rule.

| Token | Forest (default) | Ocean | Light | Used for |
|---|---|---|---|---|
| `--bg` | `#161716` | `#0b1120` | `#f5f7f2` | App background |
| `--panel` | `#20221e` | `#111827` | `#ffffff` | Card surface |
| `--panel-alt` | `#282b25` | `#0f172a` | `#eff2ea` | Secondary button, table head |
| `--panel-strong` | `#30342d` | `#1b2537` | `#e3e8dc` | Inline `code`, log badge |
| `--line` | `#3e4439` | `#1e293b` | `#d4dac9` | Every border |
| `--text` | `#eef2e8` | `#f8fafc` | `#1d2118` | Body text |
| `--muted` | `#b5beb0` | `#94a3b8` | `#5c6455` | Labels, hints, timestamps |
| `--accent` | `#83cb73` | `#38bdf8` | `#4f8f42` | Primary action, active state |
| `--accent-strong` | `#6bb25b` | `#0284c7` | `#3d7533` | Hover on primary |
| `--warn` | `#d9b74d` | `#f59e0b` | `#9a6c10` | SKIP rows, demo banner |
| `--danger` | `#d36a6a` | `#ef4444` | `#b04545` | Errors, destructive, dead stats |

Sidebar background is its own value (`#191b17` forest, `#ecefe6` light) — slightly darker
than `--bg` in dark themes, so the shell reads as two planes.

Machine-readable: `design-tokens.json`.

### Type scale (px)

| Role | Size | Weight | Notes |
|---|---|---|---|
| Page title (`h1`) | 24 | 700 | In `.panel-header` |
| Card title (`h2`) | 16 | 600–700 | One per card |
| Body / control | 15 / 13.5 | 400–600 | Buttons are 13.5 at 600 |
| Label (`label > span`) | 12.5 | 400 | `--muted` |
| Hint (`.hint-text`) | 12.5 | 400 | `--muted`, `line-height: 1.5` |
| Stat number | 17 | 700 | `--accent` |
| Log text | 12.5 | 400 | `--text`, wraps anywhere |
| Log meta / time | 11.5 / 11 | 400 | `--muted`, tabular |
| Badge / mode label | 10–11.5 | 700 | UPPERCASE + `letter-spacing: 0.04em` |

Font stack: `"Segoe UI", Arial, sans-serif` (body) and
`ui-monospace, "Cascadia Mono", Consolas, monospace` (code, prompts, log detail).
No webfont is loaded — the app must render instantly offline.

### Spacing scale

`4 · 6 · 8 · 10 · 12 · 14 · 16 · 18 · 22 · 24` px.
Cards pad **18 px** and use `gap: 11px` between children. Form grids gap **12 px**.
Sub-pages cap at **1100 px** (wide) or **760 px** (narrow, single-column forms).

### Layout geometry

```
shell            flex, min-height 100vh
sidebar          fixed 238px, sticky, full height, 22px 16px padding
main             flex 1, 22px 24px padding, max-width 1480px centered
panel-header     flex row, space-between, wraps
home-stats       grid 6 × minmax(0,1fr), gap 10px
sidebar-footer   margin-top:auto → pills pinned to the bottom
```

Only breakpoint: **900 px** — subnav becomes a horizontal wrapping row, `.log-card`
loses its max-height. Design target is 1280×820 (Wails default window).

---

## 3. Component catalogue

Every component below exists in `styles.css`. Copy the markup from `starter/index.html`.

### 3.1 Sidebar

```html
<aside class="sidebar">
  <div class="brand">
    <div class="brand-mark">AC</div>            <!-- 40×40, accent bg, dark text -->
    <div>
      <div class="brand-title">App Console</div>
      <div class="brand-subtitle">Design kit · Go + WebView</div>
    </div>
  </div>
  <nav class="nav">                            <!-- top-level modes -->
    <button class="nav-button active" data-tab="studio">Studio</button>
    <button class="nav-button" data-tab="ops">Ops</button>
  </nav>
  <nav class="subnav" id="subnav-studio">      <!-- 5 sections of that mode -->
    <button class="subnav-button active" data-subpanel="studio-setup">Setup</button>
    …
  </nav>
  <div class="sidebar-footer">                 <!-- always visible at the bottom -->
    <div class="status-row"><span class="status-label">Workers</span>
      <span id="running-pill" class="pill stopped">Stopped</span></div>
  </div>
</aside>
```

Rules: exactly two `.nav-button`s; exactly five `.subnav-button`s per mode; the inactive
subnav gets the `hidden` attribute (not `display:none` in JS); the footer holds at most two
`status-row` pills (running state + backend/health).

### 3.2 Panel header

```html
<header class="panel-header">
  <div>
    <div class="title-row">
      <h1>Studio</h1>
      <span class="mode-badge">Idle</span>     <!-- "Running" adds .live -->
    </div>
    <p>What this mode does, in one sentence.</p>
  </div>
  <div class="actions">                        <!-- 1–3 buttons max -->
    <button class="secondary-button">Refresh</button>
  </div>
</header>
```

### 3.3 Stats row

```html
<div class="home-stats">
  <div class="stat-capsule"><span>Running</span><strong id="stat-running">0</strong></div>
  <div class="stat-capsule danger"><span>Failures</span><strong id="stat-failed">0</strong></div>
</div>
```

Five or six capsules, all the same height, at most one `danger`. Values are updated by
`renderStats()` — never by rebuilding the markup.

### 3.4 Card + numbered step head

```html
<section class="card">
  <div class="step-head"><span class="step-num">1</span><h2>Setup</h2></div>
  <p class="hint-text">One sentence of guidance. Not a paragraph.</p>
  …form…
</section>
```

The number matches the subnav position (1–5). `+` is allowed for a helper card that is not
a step. **Never** nest a `.card` inside a `.card`; use `.card-grid` for siblings.

### 3.5 Form grid

```html
<div class="form-grid two-column">
  <label><span>Workspace name</span><input type="text" id="setup-name"></label>
  <label><span>Mode</span><select id="setup-mode">…</select></label>
  <label><span>Concurrency</span><input type="number" min="1" max="64" step="1" value="2"></label>
</div>
<label class="checkbox-row"><input type="checkbox" id="setup-headless"><span>Run headless</span></label>
<label><span>Notes</span><textarea id="setup-notes" rows="4"></textarea></label>
```

Rules: every control has a visible `<span>` label (no placeholder-as-label); numeric inputs
state the unit in the label (`Timeout (s)`); paired min/max always sit side by side; one
`textarea` = one concept.

### 3.6 Strategy radio cards (`mode-option`)

```html
<div class="chat-mode-card">
  <span class="mode-label">Strategy</span>
  <div class="mode-options">
    <label class="mode-option active">
      <input type="radio" name="act-strategy" value="single" checked><span>Single sequence</span>
    </label>
    …
  </div>
</div>
```

Use for **mutually exclusive strategies** (2–4 options). Selecting one shows its own block
and hides the others — but the hidden blocks keep their values (form store).

### 3.7 One-line-per-entry + test + results

```html
<textarea id="studio-conn-entries" rows="5" placeholder="host:port&#10;host:port:user:pass"></textarea>
<button class="secondary-button" id="conn-test">Test Connections</button>
<p id="studio-conn-status" class="status-line">Not tested.</p>
<div id="studio-conn-results" class="proxy-results">
  <p class="proxy-test-pass">127.0.0.1:8080 — PASS (41 ms) ok</p>
  <p class="proxy-test-fail">10.0.0.9:1080 — FAIL (0 ms) timeout</p>
  <p class="proxy-test-warn">… — WARN …</p>
</div>
```

This triple (multi-line input → test button → per-line verdict list) is the house pattern
for anything batch-shaped: proxies, endpoints, webhooks, accounts, API keys.

### 3.8 Secret field

`<input type="password" autocomplete="off">` + hint `sk-... / rc_...` path only. Never print
the value into logs, the DOM as text, or a toast.

### 3.9 Run card

```html
<section class="card run-card">
  <div class="step-head"><span class="step-num">4</span><h2>Run</h2></div>
  <div class="profile-choice">
    <span class="mode-label">Select profile</span>
    <div class="mode-options">…radio cards…</div>
  </div>
  <button class="primary-button run-button" id="studio-run">▶ Run</button>
  <p id="studio-run-status" class="status-line">Idle.</p>
</section>
```

One full-width primary button, one status line under it, one hint below that. Start and
Stop are never the same button; Stop lives in the header of the mode that owns the job.

### 3.10 Live log feed

```html
<section class="card log-card">
  <div class="log-header">
    <div class="step-head" style="margin:0"><span class="step-num">5</span><h2>Live Logs</h2></div>
    <div class="log-tools">
      <select id="studio-log-filter"><option value="all">All</option>…</select>
      <button class="secondary-button" id="studio-log-clear">Clear</button>
    </div>
  </div>
  <div id="studio-log-list" class="log-list" aria-live="polite"></div>
  <p id="studio-log-empty" class="hint-text">No activity yet. Press Run.</p>
</section>
```

Row anatomy: `[60px time] [44px kind badge] [1fr body]`, body = one `.log-text` line plus an
optional `.log-meta` line (`rule · detail`). Kinds and colours:

| kind | badge | meaning |
|---|---|---|
| `in` | blue-grey | something received |
| `out` | green | something sent / an action taken |
| `skip` | amber | deliberately not acted on (with reason) |
| `system` | neutral | lifecycle (start, cleanup, rotation) |
| `error` | red | failure (always with the next-step detail) |

The feed is a fixed-height scroll area, **pinned to the bottom** only when the user is
already near the bottom (60 px tolerance). Filters: All · IN/OUT · Rules · Errors.

### 3.11 Status line, badge, pill, toast

| Element | Class | When |
|---|---|---|
| Status line | `.status-line` + `.status-ok` / `.status-error` | Under the button that caused it — the primary feedback channel |
| Mode badge | `.mode-badge` (+ `.live`) | Next to an `h1`, shows Running/Idle |
| Pill | `.pill.running` / `.pill.stopped` | Sidebar footer only |
| Generic badge | `.badge` / `.ok` / `.warn` / `.err` | Inside tables and lists |
| Toast | `.toast` (+ `.error` / `.warn`) | Confirmation for a save that has no status line nearby (4.2 s) |

Rule: **one status line per card maximum.** Don't stack toasts for routine saves if the
card already has a status line — the toast is a bonus, not the primary channel.

### 3.12 Table, empty state, kv list

Tables exist **only** for item lists (`.table` inside `.table-wrap`): uppercase muted
headers, 10/12 px cell padding, hover row tint, badges in the state column.
`.empty-state` is a dashed box with a bold line + a hint line. `.kv` is a two-column
definition list for read-only summaries.

---

## 4. Interaction states

| State | Visual |
|---|---|
| Hover (button) | background lightens (`--accent-strong`), secondary border turns accent |
| Hover (mode option) | border → accent, no fill change |
| Active/selected | accent border + 10–12% accent fill + accent text |
| Disabled | `opacity: .55`, `cursor: not-allowed` |
| Busy | primary button disabled while the action runs (`setBusy`) |
| Running | sidebar pill green "Running", `h1` badge `.live` |
| Error | status line red; never just a red border |

Transitions are 0.15 s on background/border/colour only. No transforms, no scale, no
bounce animations. The single keyframe in the kit is the toast slide-in (0.16 s).

## 5. Accessibility & keyboard

- Native controls only (`button`, `label`, `select`, `input`, `textarea`) — screen readers
  and keyboard navigation come for free.
- Every icon-ish text (`▶`, `✓`) is decorative; the accessible name is the label text.
- Log containers use `aria-live="polite"`; status lines use `role="status"` + `aria-live`.
- Visible focus comes from the browser default — do not remove outlines.
- Contrast: text `#eef2e8` on `#20221e` ≈ 13:1; muted `#b5beb0` on `#20221e` ≈ 7:1;
  accent `#83cb73` on `#20221e` ≈ 8.5:1. Keep new colours in that band.

## 6. Theming contract

Themes only override the token block, never rules:

```html
<html data-theme="forest">   <!-- default; ocean; light -->
```

`styles.css` ships `:root[data-theme="ocean"]` and `:root[data-theme="light"]` overrides;
`starter/app.js` persists the choice in `localStorage["kit-theme"]`. Any new theme must
define **all twelve** tokens or it will inherit half of another theme and look broken.

## 7. Extending the system (the only allowed way)

1. Prefer an existing class. 80% of "new" needs are a `card` + `form-grid` + status line.
2. If a genuinely new component is needed, add it at the end of `styles.css` under a banner:
   ```css
   /* ---------- <project> additions ---------- */
   .my-thing { … uses tokens only … }
   ```
3. Add it to this document with its markup, states and rules.
4. Reuse it at least twice before inventing a third variant.
