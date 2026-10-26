// =============================================================================
//  app.js — generic dashboard shell for the console design kit
// -----------------------------------------------------------------------------
//  WHAT THIS FILE IS
//  The reusable 70%: routing (tab -> subnav -> subpanel), a global form store,
//  a single invoke() bridge, settings read/write, live log feed, status lines,
//  toasts, stats and run/stop wiring. No site-specific copy, no bot logic.
//
//  WHAT YOU CHANGE (the 30%)
//  1. CONFIG.brand ......... name, initials, subtitle
//  2. CONFIG.backend ....... "auto" | "wails" | "http" | "mock"
//  3. the FIELD MAPS ....... add/remove fields for your own settings structs
//  4. the ACTION functions .. what Run / Start / Test actually call
//  5. backend method names .. keep them or rename in BOTH Go and here
//
//  BACKEND CONTRACT (see 08-API-CONTRACT.md)
//    GetSnapshot()                  -> Snapshot      (settings + runtime + stats)
//    SaveSettings(section, values)  -> Snapshot
//    TestConnection(entries)        -> {results:[...]}
//    TestAPI(credentials)           -> string
//    StartRun(payload)              -> {ok, message}
//    StopRun()                      -> {ok}
//    LaunchLogin(profile)           -> {ok, message}
//    GetLogs(afterVersion)          -> {version, entries:[...]}
//    ClearLogs()                    -> {ok}
//    OpenDataFolder()               -> string (absolute path)
//    LoadFolder()                   -> {count, names}
// =============================================================================

const CONFIG = {
  brand: {
    mark: "AC",
    title: "App Console",
    subtitle: "Design kit · Go + WebView",
  },
  // "auto" = Wails binding if present, else HTTP /api, else in-page mock.
  backend: "auto",
  apiPrefix: "/api",
  pollLogsMs: 1000,
  pollStatsMs: 3000,
  theme: "forest",
};

// ---------------------------------------------------------------------------
// State
// ---------------------------------------------------------------------------
const state = {
  snapshot: null,
  tab: "studio",
  running: { studio: false, ops: false },
  busy: { studio: false, ops: false },
  activeSub: { studio: "studio-setup", ops: "ops-account" },
  logs: {
    studio: { after: 0, entries: [], pending: false },
    ops: { after: 0, entries: [], pending: false },
  },
  bridgeKind: "unknown",
  loadedSequences: 0,
  items: [],
};

// Every input/select/textarea with an id inside a panel is stored here, so
// switching sub-panels never loses typed values (the DOM hides, the store keeps).
const formStore = { studio: {}, ops: {} };

// Fields the user typed into but has not saved yet. Server snapshots must never
// stomp a draft; polling only refreshes untouched fields.
const dirtyFields = new Set();

// ---------------------------------------------------------------------------
// Tiny helpers
// ---------------------------------------------------------------------------
const byId = (id) => document.getElementById(id);
const text = (id) => { const el = byId(id); return el ? String(el.value || "").trim() : ""; };
const num = (id) => { const el = byId(id); return el ? Number(el.value || 0) : 0; };
const checked = (id) => { const el = byId(id); return !!(el && el.checked); };

function setValue(id, val, force) {
  const el = byId(id);
  if (!el || el === document.activeElement) return;
  if (!force && dirtyFields.has(id)) return;
  if (el.type === "checkbox") el.checked = !!val;
  else el.value = val ?? "";
}

function setStatus(id, message, kind) {
  const el = byId(id);
  if (!el) return;
  el.textContent = message || "";
  el.classList.toggle("status-error", kind === "error");
  el.classList.toggle("status-ok", kind === "ok");
}

function toast(message, kind) {
  const wrap = byId("toast-wrap");
  if (!wrap) return;
  const el = document.createElement("div");
  el.className = "toast" + (kind ? " " + kind : "");
  el.textContent = message;
  wrap.appendChild(el);
  setTimeout(() => el.remove(), 4200);
}

function nowTime() {
  return new Date().toLocaleTimeString([], { hour12: false });
}

// ---------------------------------------------------------------------------
// Form store (draft protection across sub-panel switches)
// ---------------------------------------------------------------------------
function fieldValue(el) {
  return el.type === "checkbox" ? el.checked : el.value;
}

function captureGlobalState() {
  for (const [slice, sel] of [["studio", '[data-panel="studio"]'], ["ops", '[data-panel="ops"]']]) {
    document.querySelectorAll(`${sel} input, ${sel} select, ${sel} textarea`).forEach((el) => {
      if (el.id) formStore[slice][el.id] = fieldValue(el);
    });
  }
}

function restoreGlobalState(slice) {
  const sel = slice === "studio" ? '[data-panel="studio"]' : '[data-panel="ops"]';
  document.querySelectorAll(`${sel} input, ${sel} select, ${sel} textarea`).forEach((el) => {
    if (!el.id || !(el.id in formStore[slice]) || el === document.activeElement) return;
    if (el.type === "checkbox") el.checked = !!formStore[slice][el.id];
    else el.value = formStore[slice][el.id];
  });
}

function bindGlobalState() {
  for (const [slice, sel] of [["studio", '[data-panel="studio"]'], ["ops", '[data-panel="ops"]']]) {
    document.querySelectorAll(`${sel} input, ${sel} select, ${sel} textarea`).forEach((el) => {
      if (!el.id) return;
      formStore[slice][el.id] = fieldValue(el);
      const save = () => {
        formStore[slice][el.id] = fieldValue(el);
        dirtyFields.add(el.id);
      };
      el.addEventListener("input", save);
      el.addEventListener("change", save);
    });
  }
}

// ---------------------------------------------------------------------------
// Bridge: Wails binding OR HTTP /api OR in-page mock — one call site.
// ---------------------------------------------------------------------------
const bridge = {
  kind: "unknown",
  api: null,
  async invoke(method, ...args) {
    try {
      if (this.kind === "wails" || this.kind === "mock") {
        if (!this.api || typeof this.api[method] !== "function") {
          throw new Error(`Backend method missing: ${method}`);
        }
        return await this.api[method](...args);
      }
      return await httpInvoke(method, ...args);
    } catch (error) {
      reportFrontendError(`Backend ${method} failed`, error);
      throw error;
    }
  },
};

async function httpInvoke(method, ...args) {
  const payload = args.length === 0 ? undefined : args[0];
  const res = await fetch(`${CONFIG.apiPrefix}/${method}`, {
    method: payload === undefined ? "GET" : "POST",
    headers: { "Content-Type": "application/json" },
    body: payload === undefined ? undefined : JSON.stringify(payload),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
  return data;
}

async function resolveBridge() {
  const wailsApi = window.go && window.go.main && window.go.main.App;
  if (CONFIG.backend !== "http" && CONFIG.backend !== "mock" && wailsApi) {
    bridge.kind = "wails";
    bridge.api = wailsApi;
    return;
  }

  if (CONFIG.backend !== "mock") {
    try {
      const controller = new AbortController();
      const timer = setTimeout(() => controller.abort(), 900);
      const res = await fetch(`${CONFIG.apiPrefix}/GetSnapshot`, { signal: controller.signal });
      clearTimeout(timer);
      if (res.ok) { bridge.kind = "http"; return; }
    } catch (_) { /* no HTTP backend listening — fall through */ }
  }

  const mockApi = window.__consoleMock;
  if (mockApi) { bridge.kind = "mock"; bridge.api = mockApi; return; }

  bridge.kind = "http"; // last resort: show real errors from the HTTP path
}

function reportFrontendError(context, error) {
  try {
    const runtime = window.runtime || {};
    if (typeof runtime.LogError === "function") {
      const detail = error && error.stack ? error.stack : String(error);
      runtime.LogError(`Frontend | ${context} | ${detail}`);
    }
  } catch (_) { /* logging bridge unavailable */ }
}

function subscribeEvents() {
  const runtime = window.runtime || {};
  if (typeof runtime.EventsOn !== "function") return;
  runtime.EventsOn("ui:log", () => { refreshLogs(state.tab); });
  runtime.EventsOn("ui:running", (running) => {
    state.running.ops = !!running;
    state.running.studio = !!running;
    renderRunning();
  });
}

// ---------------------------------------------------------------------------
// Routing: mode tab -> subnav -> subpanel
// ---------------------------------------------------------------------------
function activateSubpanel(panel) {
  if (!panel) return;
  const mode = panel.startsWith("ops-") ? "ops" : "studio";
  state.activeSub[mode] = panel;
  document.querySelectorAll(".subpanel").forEach((el) => {
    el.classList.toggle("active", el.dataset.subpanel === panel);
  });
  document.querySelectorAll(".subnav-button").forEach((btn) => {
    btn.classList.toggle("active", btn.dataset.subpanel === panel);
  });
  restoreGlobalState(mode);
  if (panel.endsWith("-logs")) refreshLogs(mode);
}

function activateTab(tab) {
  state.tab = tab;
  document.querySelectorAll(".nav-button").forEach((btn) => {
    btn.classList.toggle("active", btn.dataset.tab === tab);
  });
  document.querySelectorAll(".tab-panel").forEach((panel) => {
    panel.classList.toggle("active", panel.dataset.panel === tab);
  });
  byId("subnav-studio").hidden = tab !== "studio";
  byId("subnav-ops").hidden = tab !== "ops";
  activateSubpanel(state.activeSub[tab]);
}

function bindTabs() {
  document.querySelectorAll(".nav-button").forEach((btn) => {
    btn.addEventListener("click", () => activateTab(btn.dataset.tab));
  });
  document.querySelectorAll(".subnav-button").forEach((btn) => {
    btn.addEventListener("click", () => activateSubpanel(btn.dataset.subpanel));
  });
}

function renderRunning() {
  const anyRunning = state.running.studio || state.running.ops;
  const pill = byId("running-pill");
  pill.textContent = anyRunning ? "Running" : "Stopped";
  pill.className = "pill " + (anyRunning ? "running" : "stopped");

  const badge = byId("studio-run-badge");
  badge.textContent = state.running.studio ? "Running" : "Idle";
  badge.classList.toggle("live", state.running.studio);

  byId("studio-run").disabled = state.running.studio || state.busy.studio;
  byId("ops-start").disabled = state.running.ops || state.busy.ops;
  byId("ops-stop").disabled = !state.running.ops;
  byId("stat-running").textContent = anyRunning ? "1" : "0";
}

function setBusy(which, busy) {
  state.busy[which] = busy;
  const runBtn = byId(which === "studio" ? "studio-run" : "ops-start");
  if (runBtn) runBtn.disabled = busy || state.running[which];
  renderRunning();
}

function backendLabel() {
  return { wails: "wails", http: "http", mock: "demo", unknown: "…" }[bridge.kind] || bridge.kind;
}

function renderBackendPill() {
  const pill = byId("backend-pill");
  pill.textContent = backendLabel();
  pill.className = "pill " + (bridge.kind === "mock" ? "stopped" : "running");
  byId("demo-banner").hidden = bridge.kind !== "mock";
}

// ---------------------------------------------------------------------------
// Read / write settings (the 30% layer: adjust ids to your own structs)
// ---------------------------------------------------------------------------
function readSetup() {
  return {
    name: text("setup-name"),
    mode: text("setup-mode"),
    concurrency: num("setup-concurrency"),
    timeout_seconds: num("setup-timeout"),
    headless: checked("setup-headless"),
    notes: text("setup-notes"),
  };
}

function readConnections(prefix) {
  return {
    enabled: checked(`${prefix}-conn-enabled`) || checked("conn-enabled"),
    scheme: text(`${prefix}-conn-scheme`) || "http",
    entries: text(`${prefix}-conn-entries`),
  };
}

function readCredentials() {
  return {
    base_url: text("cred-base-url"),
    api_key: text("cred-api-key"),
    model: text("cred-model"),
    temperature: Number(text("cred-temperature")) || 0.8,
  };
}

function readStrategy() {
  const picked = document.querySelector('input[name="act-strategy"]:checked');
  return picked ? picked.value : "single";
}

function readActions(prefix) {
  return {
    strategy: prefix === "studio" ? readStrategy() : "single",
    sequence: text(prefix === "studio" ? "act-sequence" : "ops-sequence"),
    targets: text(prefix === "studio" ? "act-targets" : "ops-targets"),
  };
}

function readTimings() {
  return {
    poll_min: num("ops-poll-min"),
    poll_max: num("ops-poll-max"),
    action_min: num("ops-action-min"),
    action_max: num("ops-action-max"),
    timeout_min: num("ops-timeout-min"),
    timeout_max: num("ops-timeout-max"),
    retry_min: num("ops-retry-min"),
    retry_max: num("ops-retry-max"),
  };
}

function applySnapshot(snapshot) {
  if (!snapshot) return;
  state.snapshot = snapshot;
  const s = snapshot.settings || {};

  setValue("setup-name", s.setup?.name);
  setValue("setup-mode", s.setup?.mode || "balanced");
  setValue("setup-concurrency", s.setup?.concurrency ?? 2);
  setValue("setup-timeout", s.setup?.timeout_seconds ?? 60);
  setValue("setup-headless", s.setup?.headless);
  setValue("setup-notes", s.setup?.notes);

  setValue("studio-conn-enabled", s.connections?.enabled);
  setValue("studio-conn-scheme", s.connections?.scheme || "http");
  setValue("studio-conn-entries", s.connections?.entries);
  setValue("ops-conn-enabled", s.connections?.enabled);
  setValue("ops-conn-entries", s.connections?.entries);

  setValue("cred-base-url", s.credentials?.base_url);
  setValue("cred-api-key", s.credentials?.api_key);
  setValue("cred-model", s.credentials?.model);
  setValue("cred-temperature", s.credentials?.temperature ?? 0.8);

  setValue("act-sequence", s.actions?.sequence);
  setValue("act-targets", s.actions?.targets);
  setValue("ops-sequence", s.actions?.sequence);
  setValue("ops-targets", s.actions?.targets);

  const t = s.timings || {};
  setValue("ops-poll-min", t.poll_min ?? 5);
  setValue("ops-poll-max", t.poll_max ?? 12);
  setValue("ops-action-min", t.action_min ?? 3);
  setValue("ops-action-max", t.action_max ?? 8);
  setValue("ops-timeout-min", t.timeout_min ?? 60);
  setValue("ops-timeout-max", t.timeout_max ?? 90);
  setValue("ops-retry-min", t.retry_min ?? 10);
  setValue("ops-retry-max", t.retry_max ?? 30);

  setValue("ops-profile", s.profile?.name || "default");
  setValue("ops-backend", s.profile?.backend || "rod");

  if (snapshot.strategy) setStrategy(snapshot.strategy);
  renderStats(snapshot.stats || {});
  state.running.studio = !!(snapshot.runtime && snapshot.runtime.running);
  state.running.ops = !!(snapshot.runtime && snapshot.runtime.running);
  renderRunning();

  const steps = String(s.actions?.sequence || "").split(/\r?\n/).filter((l) => l.trim()).length;
  byId("ops-summary").textContent =
    `Browser: ${s.profile?.backend || "rod"} · Strategy: ${snapshot.strategy || readStrategy()} · Steps: ${steps}`;

  renderItems(snapshot.items || state.items || []);
  captureGlobalState();
}

function renderStats(stats) {
  byId("stat-profiles").textContent = stats.profiles ?? 0;
  byId("stat-done").textContent = stats.completed ?? 0;
  byId("stat-queue").textContent = stats.queued ?? 0;
  byId("stat-failed").textContent = stats.failed ?? 0;
}

function renderItems(items) {
  state.items = items;
  const body = byId("items-body");
  if (!items.length) {
    body.innerHTML = '<tr><td colspan="5">No items yet.</td></tr>';
    return;
  }
  body.replaceChildren();
  items.forEach((item, index) => {
    const tr = document.createElement("tr");
    const cells = [
      String(index + 1),
      item.name || "—",
      item.profile || "default",
      item.state || "pending",
      item.updated || nowTime(),
    ];
    cells.forEach((cellText, cellIndex) => {
      const td = document.createElement("td");
      if (cellIndex === 3) {
        const badge = document.createElement("span");
        badge.className = "badge " + (cellText === "ok" ? "ok" : cellText === "failed" ? "err" : "");
        badge.textContent = cellText;
        td.appendChild(badge);
      } else {
        td.textContent = cellText;
      }
      tr.appendChild(td);
    });
    body.appendChild(tr);
  });
}

// ---------------------------------------------------------------------------
// Actions
// ---------------------------------------------------------------------------
function setStrategy(strategy) {
  const radio = document.querySelector(`input[name="act-strategy"][value="${strategy}"]`);
  if (radio) radio.checked = true;
  document.querySelectorAll(".mode-option").forEach((option) => {
    const input = option.querySelector('input[name="act-strategy"]');
    if (input) option.classList.toggle("active", input.checked);
  });
  byId("act-section-single").style.display = strategy === "single" ? "" : "none";
  byId("act-section-folder").style.display = strategy === "folder" ? "" : "none";
  byId("act-section-model").style.display = strategy === "model" ? "" : "none";
}

async function saveSection(section, values, statusId) {
  try {
    setStatus(statusId, "Saving…");
    const snapshot = await bridge.invoke("SaveSettings", section, values);
    dirtyFields.clear();
    if (snapshot) applySnapshot(snapshot);
    setStatus(statusId, "Saved.", "ok");
    toast(`${section} saved.`);
  } catch (error) {
    setStatus(statusId, error.message || String(error), "error");
  }
}

async function testConnections(prefix) {
  const statusId = `${prefix}-conn-status`;
  const resultsId = `${prefix}-conn-results`;
  const box = byId(resultsId);
  try {
    setStatus(statusId, "Testing…");
    box.replaceChildren();
    const res = await bridge.invoke("TestConnection", readConnections(prefix));
    const results = (res && res.results) || [];
    if (!results.length) { setStatus(statusId, "No entries to test."); return; }
    results.forEach((r) => {
      const p = document.createElement("p");
      p.className = r.ok ? "proxy-test-pass" : "proxy-test-fail";
      p.textContent = `${r.entry} — ${r.ok ? "PASS" : "FAIL"} (${r.latency_ms ?? 0} ms) ${r.message || ""}`.trim();
      box.appendChild(p);
    });
    const passed = results.filter((r) => r.ok).length;
    setStatus(statusId, `${passed}/${results.length} passed.`, passed === results.length ? "ok" : "error");
  } catch (error) {
    setStatus(statusId, error.message || String(error), "error");
  }
}

async function testApi() {
  try {
    setStatus("cred-test-status", "Calling the API…");
    const message = await bridge.invoke("TestAPI", readCredentials());
    setStatus("cred-test-status", String(message || "OK"), "ok");
  } catch (error) {
    setStatus("cred-test-status", error.message || String(error), "error");
  }
}

async function runJob() {
  if (state.running.studio || state.busy.studio) return;
  setBusy("studio", true);
  try {
    setStatus("studio-run-status", "Saving Setup, Connections and Actions…");
    await bridge.invoke("SaveSettings", "setup", readSetup());
    await bridge.invoke("SaveSettings", "connections", readConnections("studio"));
    await bridge.invoke("SaveSettings", "credentials", readCredentials());
    await bridge.invoke("SaveSettings", "actions", readActions("studio"));

    state.logs.studio.entries = [];
    state.logs.studio.after = 0;
    renderLogs("studio");

    setStatus("studio-run-status", "Starting…");
    const payload = { profile: document.querySelector('input[name="run-profile"]:checked').value };
    await bridge.invoke("StartRun", payload);
    state.running.studio = true;
    renderRunning();
    setStatus("studio-run-status", "Running. Live logs will appear here.", "ok");
  } catch (error) {
    setStatus("studio-run-status", error.message || String(error), "error");
  } finally {
    setBusy("studio", false);
  }
}

async function stopJob(prefix) {
  try {
    await bridge.invoke("StopRun");
    state.running[prefix] = false;
    renderRunning();
    setStatus(prefix === "studio" ? "studio-run-status" : "ops-status", "Stopped.", "ok");
  } catch (error) {
    setStatus(prefix === "studio" ? "studio-run-status" : "ops-status", error.message || String(error), "error");
  }
}

async function startOps() {
  if (state.running.ops || state.busy.ops) return;
  setBusy("ops", true);
  try {
    setStatus("ops-status", "Saving profile, actions and settings…");
    await bridge.invoke("SaveSettings", "profile", {
      name: text("ops-profile") || "default",
      backend: text("ops-backend") || "rod",
      headless: checked("ops-headless"),
    });
    await bridge.invoke("SaveSettings", "actions", readActions("ops"));
    await bridge.invoke("SaveSettings", "timings", readTimings());

    state.logs.ops.entries = [];
    state.logs.ops.after = 0;
    renderLogs("ops");

    setStatus("ops-status", "Starting workers…");
    await bridge.invoke("StartRun", { profile: text("ops-profile") || "default" });
    state.running.ops = true;
    renderRunning();
    setStatus("ops-status", "Started. One worker runs per saved profile.", "ok");
  } catch (error) {
    setStatus("ops-status", error.message || String(error), "error");
  } finally {
    setBusy("ops", false);
  }
}

async function launchLogin() {
  try {
    setStatus("ops-login-status", "Launching browser for manual login…");
    await bridge.invoke("LaunchLogin", {
      profile: text("ops-profile") || "default",
      backend: text("ops-backend") || "rod",
      headless: false,
    });
    setStatus("ops-login-status", "Launch requested. Finish the login in the browser window.", "ok");
  } catch (error) {
    setStatus("ops-login-status", error.message || String(error), "error");
  }
}

async function loadFolder() {
  try {
    setStatus("act-folder-status", "Loading…");
    const res = await bridge.invoke("LoadFolder");
    state.loadedSequences = (res && res.count) || 0;
    setStatus("act-folder-status", `${state.loadedSequences} sequences loaded.`, "ok");
  } catch (error) {
    setStatus("act-folder-status", error.message || String(error), "error");
  }
}

async function openDataFolder() {
  try {
    const path = await bridge.invoke("OpenDataFolder");
    setStatus("act-data-status", String(path || "Opened."), "ok");
  } catch (error) {
    setStatus("act-data-status", error.message || String(error), "error");
  }
}

// ---------------------------------------------------------------------------
// Live logs
// ---------------------------------------------------------------------------
function logKindLabel(kind) {
  return { in: "IN", out: "OUT", skip: "SKIP", system: "SYS", error: "ERR" }[kind] || String(kind || "").toUpperCase();
}

function logFilterMatch(entry, filter) {
  if (filter === "io") return entry.kind === "in" || entry.kind === "out";
  if (filter === "rules") return entry.kind === "out" || entry.kind === "skip" || entry.kind === "system";
  if (filter === "errors") return entry.kind === "error";
  return true;
}

function renderLogs(prefix) {
  const list = byId(`${prefix}-log-list`);
  const filterEl = byId(`${prefix}-log-filter`);
  const emptyEl = byId(`${prefix}-log-empty`);
  if (!list) return;
  const filter = filterEl ? filterEl.value : "all";
  const nearBottom = list.scrollHeight - list.scrollTop - list.clientHeight < 60;
  const fragment = document.createDocumentFragment();
  let shown = 0;

  for (const entry of state.logs[prefix].entries) {
    if (!logFilterMatch(entry, filter)) continue;
    shown += 1;
    const row = document.createElement("div");
    row.className = `log-row log-${entry.kind}`;

    const time = document.createElement("span");
    time.className = "log-time";
    time.textContent = entry.time || "";

    const badge = document.createElement("span");
    badge.className = "log-badge";
    badge.textContent = logKindLabel(entry.kind);

    const body = document.createElement("div");
    body.className = "log-body";
    const line = document.createElement("p");
    line.className = "log-text";
    line.textContent = entry.kind === "in" || entry.kind === "out" ? `"${entry.text || ""}"` : (entry.text || "");
    body.appendChild(line);

    if (entry.meta) {
      const meta = document.createElement("p");
      meta.className = "log-meta";
      meta.textContent = entry.meta;
      body.appendChild(meta);
    }

    row.append(time, badge, body);
    fragment.appendChild(row);
  }

  list.replaceChildren(fragment);
  emptyEl.hidden = shown > 0;
  if (nearBottom) list.scrollTop = list.scrollHeight;
}

async function refreshLogs(prefix) {
  const bucket = state.logs[prefix];
  if (!bucket || bucket.pending) return;
  bucket.pending = true;
  try {
    const res = await bridge.invoke("GetLogs", bucket.after);
    if (res && Array.isArray(res.entries) && res.entries.length) {
      bucket.entries.push(...res.entries);
      bucket.after = res.version ?? bucket.entries.length;
      renderLogs(prefix);
    }
    if (res && res.cleared) {
      bucket.entries = [];
      bucket.after = res.version || 0;
      renderLogs(prefix);
    }
  } catch (_) {
    /* transient — the next poll retries */
  } finally {
    bucket.pending = false;
  }
}

async function clearLogs(prefix) {
  try {
    await bridge.invoke("ClearLogs");
    state.logs[prefix].entries = [];
    state.logs[prefix].after = 0;
    renderLogs(prefix);
  } catch (error) {
    toast(error.message || String(error), "error");
  }
}

async function refreshSnapshot(silent, force) {
  try {
    const snapshot = await bridge.invoke("GetSnapshot");
    if (force) dirtyFields.clear();
    applySnapshot(snapshot);
    return snapshot;
  } catch (error) {
    if (!silent) toast(error.message || String(error), "error");
    throw error;
  }
}

// ---------------------------------------------------------------------------
// Wiring
// ---------------------------------------------------------------------------
function bindForms() {
  document.querySelectorAll('input[name="act-strategy"]').forEach((input) => {
    input.addEventListener("change", () => setStrategy(readStrategy()));
  });
  setStrategy(readStrategy());
  byId("theme-select").value = localStorage.getItem("kit-theme") || CONFIG.theme;
  document.documentElement.dataset.theme = byId("theme-select").value;
}

function bindActions() {
  byId("theme-select").addEventListener("change", (event) => {
    document.documentElement.dataset.theme = event.target.value;
    localStorage.setItem("kit-theme", event.target.value);
  });
  byId("demo-hide").addEventListener("click", () => { byId("demo-banner").hidden = true; });

  byId("setup-save").addEventListener("click", () => saveSection("setup", readSetup(), "setup-status"));
  byId("conn-test").addEventListener("click", () => testConnections("studio"));
  byId("cred-test").addEventListener("click", testApi);
  byId("act-save").addEventListener("click", () => saveSection("actions", readActions("studio"), "act-save-status"));
  byId("act-load-folder").addEventListener("click", loadFolder);
  byId("act-open-data").addEventListener("click", openDataFolder);
  byId("act-clean-folder").addEventListener("click", () => setStatus("act-folder-status", "Loaded sequences cleaned."));

  byId("studio-refresh").addEventListener("click", () => refreshSnapshot(false, true));
  byId("studio-run").addEventListener("click", runJob);
  byId("studio-log-filter").addEventListener("change", () => renderLogs("studio"));
  byId("studio-log-clear").addEventListener("click", () => clearLogs("studio"));

  byId("ops-refresh").addEventListener("click", () => refreshSnapshot(false, true));
  byId("ops-start").addEventListener("click", startOps);
  byId("ops-stop").addEventListener("click", () => stopJob("ops"));
  byId("ops-launch-login").addEventListener("click", launchLogin);
  byId("ops-conn-test").addEventListener("click", () => testConnections("ops"));
  byId("ops-save-actions").addEventListener("click", () => saveSection("actions", readActions("ops"), "ops-actions-status"));
  byId("ops-save-settings").addEventListener("click", () => saveSection("timings", readTimings(), "ops-settings-status"));
  byId("ops-log-filter").addEventListener("change", () => renderLogs("ops"));
  byId("ops-log-clear").addEventListener("click", () => clearLogs("ops"));
}

async function bootstrap() {
  bindTabs();
  bindForms();
  bindGlobalState();
  bindActions();
  activateTab("studio");

  await resolveBridge();
  renderBackendPill();
  subscribeEvents();

  let attempts = 0;
  while (attempts < 25) {
    attempts += 1;
    try {
      await refreshSnapshot(true);
      break;
    } catch (_) {
      await new Promise((resolve) => setTimeout(resolve, 200));
    }
  }

  setInterval(() => {
    if (state.tab === "studio") refreshLogs("studio");
    else refreshLogs("ops");
  }, CONFIG.pollLogsMs);

  setInterval(() => { refreshSnapshot(true).catch(() => {}); }, CONFIG.pollStatsMs);
}

window.addEventListener("error", (event) => {
  reportFrontendError("Uncaught error", event.error || event.message);
});
window.addEventListener("unhandledrejection", (event) => {
  reportFrontendError("Unhandled promise rejection", event.reason);
});
window.addEventListener("DOMContentLoaded", () => {
  bootstrap().catch((error) => {
    reportFrontendError("Bootstrap failed", error);
    setStatus("studio-run-status", error.message || String(error), "error");
  });
});
