# 06 · FRONTEND PATTERNS

Ten patterns are the whole frontend. `starter/app.js` implements all of them in ~640
commented lines; `reference/frontend/app.js` is the original (1024 lines) with the same
skeleton plus domain-specific players. Read this, then read the file.

---

## P1 · Two-level routing (tab → subnav → sub-panel)

State, not URLs. No history API, no router library.

```js
const state = { tab: "studio", activeSub: { studio: "studio-setup", ops: "ops-account" } };

function activateSubpanel(panel) {
  const mode = panel.startsWith("ops-") ? "ops" : "studio";
  state.activeSub[mode] = panel;
  document.querySelectorAll(".subpanel").forEach((el) =>
    el.classList.toggle("active", el.dataset.subpanel === panel));
  document.querySelectorAll(".subnav-button").forEach((btn) =>
    btn.classList.toggle("active", btn.dataset.subpanel === panel));
  restoreGlobalState(mode);                   // drafts come back with the panel
  if (panel.endsWith("-logs")) refreshLogs(mode);
}

function activateTab(tab) {
  state.tab = tab;
  document.querySelectorAll(".tab-panel").forEach((p) =>
    p.classList.toggle("active", p.dataset.panel === tab));
  byId("subnav-studio").hidden = tab !== "studio";
  byId("subnav-ops").hidden = tab !== "ops";
  activateSubpanel(state.activeSub[tab]);     // remember where the user was
}
```

Why class toggles: hidden panels stay focusable-skipped (`display:none` via `.active`) and
keep their DOM values. `hidden` on a subnav is the one exception (it has no form state).

## P2 · Draft-safe form store

Typing must survive both panel switches and background polling.

```js
const formStore = { studio: {}, ops: {} };
const dirtyFields = new Set();               // fields the user changed, unsaved

function bindGlobalState() {                 // once at boot
  for (const [slice, sel] of [["studio", '[data-panel="studio"]'], ["ops", '[data-panel="ops"]']]) {
    document.querySelectorAll(`${sel} input, ${sel} select, ${sel} textarea`).forEach((el) => {
      if (!el.id) return;
      formStore[slice][el.id] = fieldValue(el);
      const save = () => { formStore[slice][el.id] = fieldValue(el); dirtyFields.add(el.id); };
      el.addEventListener("input", save);
      el.addEventListener("change", save);
    });
  }
}

function setValue(id, val, force) {          // the ONLY writer from server → DOM
  const el = byId(id);
  if (!el || el === document.activeElement) return;
  if (!force && dirtyFields.has(id)) return; // never stomp a draft
  …
}
```

`dirtyFields.clear()` happens only after a successful save (or an explicit Refresh).
This is why the starter's polling every 3 s cannot eat what you are typing.

## P3 · One bridge, three transports

```js
const bridge = {
  kind: "unknown", api: null,
  async invoke(method, ...args) {
    if (this.kind === "wails" || this.kind === "mock") return this.api[method](...args);
    return httpInvoke(method, ...args);                       // POST /api/<Method>
  },
};

async function resolveBridge() {
  const wailsApi = window.go?.main?.App;
  if (wailsApi) return void ((bridge.kind = "wails"), (bridge.api = wailsApi));

  try {                                                        // probe, 900 ms budget
    const res = await fetch("/api/GetSnapshot", { signal: AbortSignal.timeout(900) });
    if (res.ok) return void (bridge.kind = "http");
  } catch (_) { /* nothing listening */ }

  if (window.__consoleMock) return void ((bridge.kind = "mock"), (bridge.api = window.__consoleMock));
  bridge.kind = "http";                                        // expose real errors
}
```

`renderBackendPill()` shows `wails` / `http` / `demo` in the sidebar and reveals the demo
banner for `mock` — the user always knows where the data came from. Never call `fetch`
outside this file section.

## P4 · Snapshot in, snapshot out

One object carries everything the UI needs:

```js
function applySnapshot(snapshot) {
  const s = snapshot.settings || {};
  setValue("setup-name", s.setup?.name);            // one line per field
  setValue("studio-conn-entries", s.connections?.entries);
  …
  renderStats(snapshot.stats || {});
  state.running.studio = !!snapshot.runtime?.running;
  renderRunning();
  captureGlobalState();                             // re-baseline the form store
}
```

Reads are the mirror image, grouped per section so saving is obvious:

```js
function readSetup() {
  return { name: text("setup-name"), mode: text("setup-mode"),
           concurrency: num("setup-concurrency"), timeout_seconds: num("setup-timeout"),
           headless: checked("setup-headless"), notes: text("setup-notes") };
}
```

Helpers `text/num/checked/setValue` keep every field access to one line. Optional chaining
(`s.setup?.name`) means an older backend that lacks a field cannot break the UI.

## P5 · Save → re-apply → report

```js
async function saveSection(section, values, statusId) {
  try {
    setStatus(statusId, "Saving…");
    const snapshot = await bridge.invoke("SaveSettings", section, values);
    dirtyFields.clear();
    if (snapshot) applySnapshot(snapshot);          // server is the source of truth
    setStatus(statusId, "Saved.", "ok");
    toast(`${section} saved.`);
  } catch (error) {
    setStatus(statusId, error.message || String(error), "error");
  }
}
```

Returning the **new snapshot** from a save kills a whole class of bugs (stale UI after the
backend normalises a value). Copy that: every mutating method returns the fresh snapshot.

## P6 · Action = save everything, then start

```js
async function runJob() {
  setBusy("studio", true);
  try {
    await bridge.invoke("SaveSettings", "setup", readSetup());
    await bridge.invoke("SaveSettings", "connections", readConnections("studio"));
    await bridge.invoke("SaveSettings", "credentials", readCredentials());
    await bridge.invoke("SaveSettings", "actions", readActions("studio"));

    state.logs.studio.entries = []; state.logs.studio.after = 0; renderLogs("studio");

    await bridge.invoke("StartRun", { profile: selectedProfile() });
    state.running.studio = true; renderRunning();
    setStatus("studio-run-status", "Running. Live logs will appear here.", "ok");
  } catch (err) {
    setStatus("studio-run-status", err.message, "error");       // never silent
  } finally {
    setBusy("studio", false);
  }
}
```

The user never has to remember Save before Run. Clearing the log buffer before start makes
the new run readable.

## P7 · Live log feed (append, filter, pin scroll)

```js
function renderLogs(prefix) {
  const list = byId(`${prefix}-log-list`);
  const filter = byId(`${prefix}-log-filter`).value;
  const nearBottom = list.scrollHeight - list.scrollTop - list.clientHeight < 60;

  const frag = document.createDocumentFragment();
  let shown = 0;
  for (const entry of state.logs[prefix].entries) {
    if (!logFilterMatch(entry, filter)) continue;
    shown++;
    const row = document.createElement("div");
    row.className = `log-row log-${entry.kind}`;
    // time span, kind badge, body (text + optional meta) — textContent only
    frag.appendChild(row);
  }
  list.replaceChildren(frag);
  byId(`${prefix}-log-empty`).hidden = shown > 0;
  if (nearBottom) list.scrollTop = list.scrollHeight;             // don't yank a reader
}

async function refreshLogs(prefix) {
  const bucket = state.logs[prefix];
  if (bucket.pending) return;                                    // no overlapping polls
  bucket.pending = true;
  try {
    const res = await bridge.invoke("GetLogs", bucket.after);     // versioned cursor
    if (res?.entries?.length) {
      bucket.entries.push(...res.entries);
      bucket.after = res.version;
      renderLogs(prefix);
    }
  } finally { bucket.pending = false; }
}
```

Never send the whole log array on every poll (the original streams hundreds of lines/min).
Cursor = the last entry id. `textContent` only — log text may contain HTML-ish payloads.

## P8 · Status, badge, busy — one vocabulary

| Need | Call | Lands |
|---|---|---|
| "working / done / failed" near a button | `setStatus(id, msg, "ok"\|"error")` | `.status-line` (red/green class) |
| Running state everywhere | `renderRunning()` | sidebar pill, `h1` badge, Run/Stop disabled |
| Transient confirmation | `toast(msg, "error"\|"warn")` | bottom-right, 4.2 s |
| Per-item verdict | `.proxy-test-pass/fail/warn` rows | right under the Test button |
| Frontend exception | `reportFrontendError(ctx, err)` | `runtime.LogError` → `logs.txt` |

`setBusy()` disables the primary button while its action runs; **always** released in a
`finally`, otherwise the UI locks.

## P9 · Section-specific helpers via a prefix (avoid copy-paste)

Both modes share Proxy/Message components; the code is written once with an id prefix:

```js
function fillProxy(prefix, proxy) {        // prefix = "sm" | "bg"  (starter: "studio" | "ops")
  byId(`${prefix}-conn-enabled`).checked = !!proxy.enabled;
  byId(`${prefix}-conn-entries`).value = proxy.entries || "";
}
async function testProxy(prefix) { … byId(`${prefix}-conn-status`) … }
```

Copy the pattern, not the markup: two nearly identical panels are fine, two nearly
identical functions are not.

## P10 · Boot with retries, never a blank screen

```js
async function bootstrap() {
  bindTabs(); bindForms(); bindGlobalState(); bindActions();
  activateTab("studio");
  await resolveBridge(); renderBackendPill(); subscribeEvents();

  for (let i = 0; i < 25; i++) {              // the Wails binding can attach late
    try { await refreshSnapshot(true); break; }
    catch (_) { await new Promise((r) => setTimeout(r, 200)); }
  }
  setInterval(() => refreshLogs(state.tab), 1000);
  setInterval(() => refreshSnapshot(true).catch(() => {}), 3000);
}
```

Plus global handlers:

```js
window.addEventListener("error", (e) => reportFrontendError("Uncaught error", e.error || e.message));
window.addEventListener("unhandledrejection", (e) => reportFrontendError("Uncaught rejection", e.reason));
```

A desktop app that shows nothing on a binding delay looks crashed. Retry, then report.

---

## API of app.js (what you may call from console during debugging)

| Call | Effect |
|---|---|
| `refreshSnapshot(false, true)` | Force re-read, clear drafts |
| `refreshLogs("studio")` | One poll now |
| `applySnapshot(window.__snapshot)` | Repaint from a captured object |
| `dirtyFields` | `Set` — inspect what would be saved |
| `state` | Everything the shell knows |
| `bridge.kind` | `"wails" \| "http" \| "mock"` |

## Style conventions (keep them)

- 2-space indent, double quotes, semicolons, `const`/`let` only.
- Named functions, no arrow-function soup; one responsibility per function.
- `const $ = byId` shorthands at the top; no jQuery, no templating, no `innerHTML` for
  user data (only for static table placeholders).
- Comments explain *why*, in English, with `// ---- section ----` banners.
- Every DOM id appears in exactly one read function and one `setValue` line.
