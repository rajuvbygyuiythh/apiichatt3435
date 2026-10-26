# 10 · REBRAND PLAYBOOK (the 30%)

You have `starter/` running. Now make it *your* site. Do these in order; each step ends with
something you can see in the browser, so you never break a working thing.

---

## Step 0 · Write the 30% decisions down first (10 minutes)

Fill this in before touching code. If you cannot fill a row, you are not ready.

```
PRODUCT NAME        : ..............................
INITIALS (2 chars)  : ..............................
SUBTITLE (1 line)   : ..............................
MODE A (name)       : ..............................   job: ...............
MODE B (name)       : ..............................   job: ...............
SECTION NAMES       :  A: 1 ..... 2 ..... 3 ..... 4 ..... 5 .....
                       B: 1 ..... 2 ..... 3 ..... 4 ..... 5 .....
SETTINGS STRUCTS    :  setup{...} connections{...} actions{...} timings{...}
CREDENTIALS         :  base_url / api_key / model / other: ............
TEST verb           :  "Test Connections" tests ........................
RUN verb            :  "Run" starts ....................................
LOG KINDS           :  in / out / skip / system / error  (reuse all five?)
DATA FOLDER         :  data/<name>/  contains ..........................
LIST TABLE          :  columns: ........................................
TARGET PLATFORM     :  desktop (Wails)  |  web  |  both
```

## Step 1 · Brand (5 files, 10 lines)

| Where | Change |
|---|---|
| `index.html` | `<title>`, `#brand-mark` (2 letters), `#brand-title`, `#brand-subtitle` |
| `app.js` | `CONFIG.brand` (same three strings — single source if you render from JS) |
| `main.go` | `wails.Run` `Title:`; `BackgroundColour` = your `--bg` |
| `wails.json` | `"name"`, `"outputfilename"` (exe name) |
| `install.bat` | exe name in the build commands and the final echo |

Then: `grep -ri "chitchat\|app console\|demo-workspace" .` must return nothing but docs.

## Step 2 · Theme (optional but cheap)

Keep `forest`, or switch the default by setting `data-theme="ocean"` / `"light"` on `<html>`
and `CONFIG.theme`. For a brand colour, edit **only** `--accent` / `--accent-strong` in the
chosen theme block and re-check contrast ≥ 4.5:1 against `--panel`. Never add a second hue.

## Step 3 · Mode and section names

```html
<!-- sidebar -->
<button class="nav-button active" data-tab="studio">Monitor</button>
<button class="nav-button" data-tab="ops">Publish</button>

<button class="subnav-button active" data-subpanel="studio-setup">Workspace</button>
<button class="subnav-button" data-subpanel="studio-connections">Sources</button>
<button class="subnav-button" data-subpanel="studio-actions">Rules</button>
<button class="subnav-button" data-subpanel="studio-run">Run</button>
<button class="subnav-button" data-subpanel="studio-logs">Live Logs</button>
```

If a name changes, change it in **three** places: the sidebar button, the card
`<h2>`, and the sub-panel `data-subpanel` (+ the routing prefix check in app.js if you
rename the mode). Keep the prefixes (`studio-` / `ops-`) — they are what makes the prefixed
helpers (`fillProxy(prefix)`) work.

Section count stays five. If you truly need six, renumber the step heads (1–6) and the
subnav together, and update `05-DASHBOARD-SPEC.md` in your repo.

## Step 4 · Settings structs (Go + JS, mirrored)

1. Add the struct in `internal/model/` with `json` tags and a `Default*()` function.
2. Add a `settings/<section>` key + `Load/Save` pair in `internal/uistore`.
3. Add `read<Section>()` in app.js + matching `setValue(...)` lines in `applySnapshot()`.
4. Add the fields in HTML inside a `form-grid`.
5. Make `SaveSettings` (or a per-section method) return the fresh snapshot.

Golden rule: **a field appears once in the HTML, once in `read…`, once in `setValue…`.**
Duplicated fields are the #1 source of "my save didn't stick" bugs.

## Step 5 · Verb rewiring (Test / Run / Stop)

| Starter call | Replace with |
|---|---|
| `TestConnection` → `{results:[{entry,ok,latency_ms,message}]}` | Keep the shape, change what is tested (URL, key, DB, webhook…). If results are not per-line, still return `entry` = the thing tested |
| `TestAPI` → string | Return a short human sentence: `"200 OK · 1 model · 42 ms"` |
| `StartRun({profile})` | Your job. Must return fast (<2 s); stream progress via log rows |
| `StopRun()` | Cancel + idempotent. Always emit `ui:running=false` |
| `LaunchLogin({profile,…})` | Only if a manual login exists; otherwise delete the Account section and use four sections |

## Step 6 · Log vocabulary

Reuse the five kinds (`in`, `out`, `skip`, `system`, `error`). Rules of thumb:

- **in** = something arrived (message, webhook, API response, request)
- **out** = something you did (send, write, publish, commit)
- **skip** = deliberately not acted on, with the reason in `meta`
- **system** = lifecycle (start, cleanup, rotate, retry scheduled)
- **error** = failure + the next step (retry in Ns, will skip)

Change the **labels** in `logKindLabel()` if your domain reads better ("REQ"/"RES"),
not the CSS class names.

## Step 7 · Data folder

```
data/
├── ui/badger/          keep
├── logs.txt            keep
├── webview/            keep (desktop)
└── <your-folder>/      prompts/ · sequences/ · captures/ · exports/
```

Add an `OpenDataFolder()` button in the Actions card so users can find it, and seed the
folder on first run (the original did this with `seedDefaults()`), never at build time.

## Step 8 · Table columns

`renderItems()` builds five `<td>`s. Change `cells[...]` to your columns **and** update the
`<thead>` in the same commit. Badges belong only in the state column.

## Step 9 · Clean-up sweep

```bash
grep -rin "chitchat\|persona\|snap\|telegram\|evachat" . --exclude-dir=node_modules --exclude-dir=.git
grep -rin "sk-\|Bearer \|api_key.*=.*\"" . --exclude-dir=.git   # no real secrets
```

Delete: `js/mock-backend.js` + its `<script>` + `demo-banner` (or keep them behind a
`?demo` query for design review — recommended). Keep `reference/` out of the shipped repo.

---

## Four worked examples (how the 70% maps to a different site)

### A · Price tracker / monitoring console
| Slot | Content |
|---|---|
| Modes | **Monitor** (one-off scan) · **Watcher** (scheduled) |
| Setup | sources list, interval, concurrency, headless |
| Connections | proxy list + Test; alert webhook + Test API |
| Actions | rule radio (price drop / back in stock / keyword), sequence textarea → rules textarea, targets |
| Run | scan now, profile = browser session |
| Logs | in = price read, out = alert sent, skip = unchanged, error = fetch failed |

### B · Content publishing console
| Slot | Content |
|---|---|
| Modes | **Compose** · **Queue** |
| Setup | site select, account, timezone, dry-run toggle |
| Connections | site API endpoints + Test; CMS credentials + Test API |
| Actions | strategy radio (single post / batch folder / AI-written), body textarea, tags |
| Run | publish now; Stop cancels remaining queue |
| Logs | out = published, in = response, skip = duplicate, error = 4xx body |

### C · Data import / ETL console
| Slot | Content |
|---|---|
| Modes | **Import** · **Schedule** |
| Setup | source type, file/URL, delimiter, chunk size |
| Connections | DB DSN list + Test; object storage + Test API |
| Actions | mapping radio (auto / template / manual JSON), mapping textarea |
| Run | start import; progress in the log; stats: rows in/out/failed |
| Logs | system = chunk boundaries, out = rows written |

### D · Internal ops / release console
| Slot | Content |
|---|---|
| Modes | **Release** · **Agents** |
| Setup | repo, branch, version, environment |
| Connections | token list + Test; host list + Test |
| Actions | strategy radio (patch / minor / major), changelog textarea |
| Run | build + deploy; Stop = abort |
| Logs | system = step boundaries, error = failing command, meta = duration |

---

## What you must NOT rebrand away

- The 2×5 information architecture (it is the design).
- The status-line-first feedback model.
- The append-only, kind-tagged log feed.
- `formStore` + `dirtyFields` (draft protection).
- The single `invoke()` bridge and its three transports.
- The five visual rules (no gradients, one accent, borders not shadows, tight radii,
  text-only labels).
