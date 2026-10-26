// =============================================================================
//  app.js — Multi-Site Console dashboard (5-site engine)
// -----------------------------------------------------------------------------
//  Built from the console design kit. The 70% (routing, form store, bridge,
//  snapshot apply, log feed, statuses, themes) is unchanged; the 30% is:
//    - a site registry with 5 sites (site picker + multi-select + state table)
//    - per-site probe (Test Site) and DOM-driver viewer
//    - auto-register accounts + manual login per site
//    - Show mode = one visible browser · Worker mode = headless multi-site
//
//  Contract (one object argument per call — see NOTES.md §4):
//    GetSnapshot()                                  -> Snapshot
//    SaveSettings({section, values})                -> Snapshot
//    GetLogs({after})                               -> {version, entries, cleared, running, stats}
//    ClearLogs()                                    -> {ok}
//    TestSite({site_id})                            -> {entry, ok, latency_ms, http, message}
//    TestConnections(proxySettings)                 -> {results:[…]}
//    StartRun({mode, sites, profile})               -> {ok}
//    StopRun()                                      -> {ok}
//    LaunchLogin({site_id, profile})                -> {ok}
//    AutoRegisterAccounts({site_id, count})         -> {accounts:[…], count}
//    GetSiteDriver({site_id})                       -> {dom_driver_js, site_json, protection_note}
//    LoadFolder()                                   -> {count, names, sequences:[{name,lines}], total}
//    OpenDataFolder()                               -> {path}
//  Step-3 additions:
//    GetSiteDiagnostics({site_id})                  -> SiteDiagnostics
//    GetSessions({site_id})                         -> {site_id, sessions:[…], count}
//    SaveSession({site_id, profile, values})        -> SiteSession
//    DeleteSession({site_id, profile})              -> {ok}
//  Step-4 additions:
//    GetBrowserStatus()                             -> {found, path, name, source, version, note}
//    TestSiteBrowser({site_id, profile})            -> {ok, probe, phase, has_input, protection, screenshot}
// =============================================================================

const CONFIG = {
  brand: { mark: "MS", title: "Multi-Site Console", subtitle: "5-site engine · Go + WebView" },
  backend: "auto",       // auto | wails | http | mock
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
  tab: "show",
  activeSub: { show: "show-site", worker: "worker-sites" },
  running: false,
  runMode: "",
  busy: { show: false, worker: false },
  logs: {
    show: { after: 0, entries: [], pending: false },
    worker: { after: 0, entries: [], pending: false },
  },
  selectedSite: "",
  selectedSites: [],
  accounts: [],
  bridgeKind: "unknown",
  diagnostics: null,
  diagSiteID: "",
  sessions: [],
};

// Every input/select/textarea with an id inside a panel is mirrored here so
// switching sub-panels never loses typed values.
const formStore = { show: {}, worker: {} };
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

function badgeClass(status) {
  if (status === "ready") return "badge ok";
  if (status === "needs-setup" || status === "selected") return "badge warn";
  if (status === "disabled" || status === "no-driver") return "badge err";
  return "badge";
}

// ---------------------------------------------------------------------------
// Form store (draft protection across sub-panel switches + polling)
// ---------------------------------------------------------------------------
function fieldValue(el) { return el.type === "checkbox" ? el.checked : el.value; }

function captureGlobalState() {
  for (const [slice, sel] of [["show", '[data-panel="show"]'], ["worker", '[data-panel="worker"]']]) {
    document.querySelectorAll(`${sel} input, ${sel} select, ${sel} textarea`).forEach((el) => {
      if (el.id) formStore[slice][el.id] = fieldValue(el);
    });
  }
}

function restoreGlobalState(slice) {
  const sel = slice === "show" ? '[data-panel="show"]' : '[data-panel="worker"]';
  document.querySelectorAll(`${sel} input, ${sel} select, ${sel} textarea`).forEach((el) => {
    if (!el.id || !(el.id in formStore[slice]) || el === document.activeElement) return;
    if (el.type === "checkbox") el.checked = !!formStore[slice][el.id];
    else el.value = formStore[slice][el.id];
  });
}

function bindGlobalState() {
  for (const [slice, sel] of [["show", '[data-panel="show"]'], ["worker", '[data-panel="worker"]']]) {
    document.querySelectorAll(`${sel} input, ${sel} select, ${sel} textarea`).forEach((el) => {
      if (!el.id) return;
      formStore[slice][el.id] = fieldValue(el);
      const save = () => { formStore[slice][el.id] = fieldValue(el); dirtyFields.add(el.id); };
      el.addEventListener("input", save);
      el.addEventListener("change", save);
    });
  }
}

// ---------------------------------------------------------------------------
// Bridge: Wails binding OR HTTP /api OR in-page mock
// ---------------------------------------------------------------------------
const bridge = {
  kind: "unknown",
  api: null,
  async invoke(method, payload) {
    try {
      if (this.kind === "wails" || this.kind === "mock") {
        if (!this.api || typeof this.api[method] !== "function") {
          throw new Error(`Backend method missing: ${method}`);
        }
        return await this.api[method](payload);
      }
      return await httpInvoke(method, payload);
    } catch (error) {
      reportFrontendError(`Backend ${method} failed`, error);
      throw error;
    }
  },
};

async function httpInvoke(method, payload) {
  const url = `${CONFIG.apiPrefix}/${method}`;
  const options = method === "GetSnapshot"
    ? { method: "GET" }
    : {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload ?? {}),
      };
  const res = await fetch(url, options);
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
  return data;
}

async function resolveBridge() {
  const wailsApi = window.go && window.go.main && window.go.main.App;
  if (CONFIG.backend !== "http" && CONFIG.backend !== "mock" && wailsApi) {
    bridge.kind = "wails"; bridge.api = wailsApi; return;
  }
  if (CONFIG.backend !== "mock") {
    try {
      const controller = new AbortController();
      const timer = setTimeout(() => controller.abort(), 1200);
      const res = await fetch(`${CONFIG.apiPrefix}/GetSnapshot`, { signal: controller.signal });
      clearTimeout(timer);
      if (res.ok) { bridge.kind = "http"; return; }
    } catch (_) { /* no HTTP backend — fall through */ }
  }
  const mockApi = window.__consoleMock;
  if (mockApi) { bridge.kind = "mock"; bridge.api = mockApi; return; }
  bridge.kind = "http";
}

function reportFrontendError(context, error) {
  try {
    const runtime = window.runtime || {};
    if (typeof runtime.LogError === "function") {
      runtime.LogError(`Frontend | ${context} | ${error && error.stack ? error.stack : String(error)}`);
    }
  } catch (_) { /* logging bridge unavailable */ }
}

function subscribeEvents() {
  const runtime = window.runtime || {};
  if (typeof runtime.EventsOn !== "function") return;
  runtime.EventsOn("ui:log", () => refreshLogs(state.tab));
  runtime.EventsOn("ui:running", (running) => {
    state.running = !!running;
    renderRunning();
    refreshLogs(state.tab);
  });
}

// ---------------------------------------------------------------------------
// Routing
// ---------------------------------------------------------------------------
function activateSubpanel(panel) {
  if (!panel) return;
  const mode = panel.startsWith("worker-") ? "worker" : "show";
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
  byId("subnav-show").hidden = tab !== "show";
  byId("subnav-worker").hidden = tab !== "worker";
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
  const pill = byId("running-pill");
  pill.textContent = state.running ? "Running" : "Stopped";
  pill.className = "pill " + (state.running ? "running" : "stopped");

  const badge = byId("show-run-badge");
  badge.textContent = state.running ? "Running" : "Idle";
  badge.classList.toggle("live", state.running);

  const workerBadge = byId("worker-badge");
  workerBadge.textContent = state.running ? "Running" : "API worker";
  workerBadge.classList.toggle("live", state.running);

  byId("run-start").disabled = state.running || state.busy.show;
  byId("worker-start").disabled = state.running || state.busy.worker;
  byId("worker-stop").disabled = !state.running;
  byId("stat-running").textContent = state.running ? "1" : "0";
}

function setBusy(which, busy) {
  state.busy[which] = busy;
  renderRunning();
}

function renderBackendPill() {
  const pill = byId("backend-pill");
  pill.textContent = { wails: "wails", http: "http", mock: "demo", unknown: "…" }[bridge.kind] || bridge.kind;
  pill.className = "pill " + (bridge.kind === "mock" ? "stopped" : "running");
  byId("demo-banner").hidden = bridge.kind !== "mock";
}

// ---------------------------------------------------------------------------
// Reads (DOM -> payload). One function per settings section.
// ---------------------------------------------------------------------------
function readShowSetting() {
  return {
    site_id: state.selectedSite || currentSnapshotSiteID(),
    profile: text("sf-profile") || "default",
    first: {
      mode: text("sf-mode") || "bot",
      chance: num("sf-chance"),
      wait_mode: text("sf-wait") || "unlimited",
      timeout_min: num("sf-timeout-min"),
      timeout_max: num("sf-timeout-max"),
      reply_min: num("sf-reply-min"),
      reply_max: num("sf-reply-max"),
      no_reply_min: num("sf-noreply-min"),
      no_reply_max: num("sf-noreply-max"),
      auto_skip: checked("sf-autoskip"),
    },
  };
}

function readMessage(prefix) {
  const radio = document.querySelector(`input[name="${prefix}-strategy"]:checked`);
  return {
    strategy: radio ? radio.value : "single",
    sequence: text(`${prefix}-sequence`),
    targets: text(`${prefix}-targets`),
    max_lines: num(`${prefix}-max-lines`),
  };
}

function readBrowserSettings() {
  return {
    path: text("wk-browser-path"),
    headless: checked("wk-browser-headless"),
    page_timeout_ms: num("wk-page-timeout"),
    keep_profile: checked("wk-keep-profile"),
  };
}

function readWorkerSettings() {
  return {
    sites: state.selectedSites.slice(),
    profile: text("wk-profile") || "default",
    headless: checked("wk-headless"),
    concurrency: num("wk-concurrency"),
    poll_min: num("wk-poll-min"),
    poll_max: num("wk-poll-max"),
    action_min: num("wk-action-min"),
    action_max: num("wk-action-max"),
    timeout_min: num("wk-timeout-min"),
    timeout_max: num("wk-timeout-max"),
    retry_min: num("wk-retry-min"),
    retry_max: num("wk-retry-max"),
  };
}

function readProxy() {
  return { enabled: checked("wk-proxy-enabled"), scheme: text("wk-proxy-scheme") || "http", entries: text("wk-proxy-entries") };
}

function readAccountSettings() {
  return {
    count: num("acc-count"),
    gender: text("acc-gender") || "female",
    age: num("acc-age"),
    nickname_pool: text("acc-pool"),
  };
}

function currentSnapshotSiteID() {
  return (state.snapshot && state.snapshot.settings && state.snapshot.settings.show && state.snapshot.settings.show.site_id) || "";
}

// ---------------------------------------------------------------------------
// Snapshot -> DOM
// ---------------------------------------------------------------------------
function applySnapshot(snapshot) {
  if (!snapshot) return;
  state.snapshot = snapshot;
  const s = snapshot.settings || {};

  // Show · Setting
  const show = s.show || {};
  const first = show.first || {};
  setValue("sf-mode", first.mode || "bot");
  setValue("sf-chance", first.chance ?? 70);
  setValue("sf-wait", first.wait_mode || "unlimited");
  setValue("sf-timeout-min", first.timeout_min ?? 30);
  setValue("sf-timeout-max", first.timeout_max ?? 60);
  setValue("sf-reply-min", first.reply_min ?? 4);
  setValue("sf-reply-max", first.reply_max ?? 9);
  setValue("sf-noreply-min", first.no_reply_min ?? 60);
  setValue("sf-noreply-max", first.no_reply_max ?? 90);
  setValue("sf-autoskip", first.auto_skip);
  setValue("sf-profile", show.profile || "default");

  // Message (both modes share the section)
  const message = s.message || {};
  setMessageStrategy("msg", message.strategy || "single");
  setMessageStrategy("wmsg", message.strategy || "single");
  setValue("msg-sequence", message.sequence);
  setValue("wmsg-sequence", message.sequence);
  setValue("msg-targets", message.targets);
  setValue("wmsg-targets", message.targets);
  setValue("msg-max-lines", message.max_lines ?? 3);
  setValue("wmsg-max-lines", message.max_lines ?? 3);

  // Worker · Settings
  const worker = s.worker || {};
  setValue("wk-poll-min", worker.poll_min ?? 5);
  setValue("wk-poll-max", worker.poll_max ?? 12);
  setValue("wk-action-min", worker.action_min ?? 3);
  setValue("wk-action-max", worker.action_max ?? 8);
  setValue("wk-timeout-min", worker.timeout_min ?? 60);
  setValue("wk-timeout-max", worker.timeout_max ?? 90);
  setValue("wk-retry-min", worker.retry_min ?? 10);
  setValue("wk-retry-max", worker.retry_max ?? 30);
  setValue("wk-concurrency", worker.concurrency ?? 1);
  setValue("wk-profile", worker.profile || "default");
  setValue("wk-headless", worker.headless);

  // Browser driver (step 4)
  const browser = s.browser || {};
  setValue("wk-browser-path", browser.path || "");
  setValue("wk-browser-headless", browser.headless !== false, true);
  setValue("wk-page-timeout", browser.page_timeout_ms || 45000);
  setValue("wk-keep-profile", browser.keep_profile !== false, true);

  // Proxy
  const proxy = s.proxy || {};
  setValue("wk-proxy-enabled", proxy.enabled);
  setValue("wk-proxy-scheme", proxy.scheme || "http");
  setValue("wk-proxy-entries", proxy.entries);

  // Accounts
  const accounts = s.accounts || {};
  setValue("acc-count", accounts.count ?? 2);
  setValue("acc-gender", accounts.gender || "female");
  setValue("acc-age", accounts.age ?? 21);
  setValue("acc-pool", accounts.nickname_pool);

  // Sites
  state.selectedSite = show.site_id || (snapshot.sites && snapshot.sites[0] && snapshot.sites[0].id) || "";
  state.selectedSites = (worker.sites || []).slice();
  renderSitePicker(snapshot.sites || []);
  renderSiteFacts();
  renderSiteCheckList(snapshot.sites || []);
  renderSiteTable(snapshot.sites || []);
  renderAccountSiteSelect(snapshot.sites || []);
  fillSiteSelect("show-log-site", snapshot.sites || []);
  fillSiteSelect("worker-log-site", snapshot.sites || []);
  fillSessionSiteSelect(snapshot.sites || []);

  // Stats + runtime
  renderStats(snapshot.stats || {});
  state.running = !!(snapshot.runtime && snapshot.runtime.running);
  state.runMode = (snapshot.runtime && snapshot.runtime.mode) || "";
  renderRunning();

  const selectedCount = (worker.sites || []).length;
  byId("worker-summary").textContent =
    `${(snapshot.sites || []).length} sites · ${selectedCount} selected · concurrency ${worker.concurrency ?? 1} · backend ${bridge.kind}` +
    (snapshot.runtime && snapshot.runtime.version ? ` · v${snapshot.runtime.version}` : "");
  byId("wsel-count").textContent = `${selectedCount} sites selected`;

  captureGlobalState();
}

function renderStats(stats) {
  byId("stat-sites").textContent = stats.sites ?? 0;
  byId("stat-enabled").textContent = stats.enabled ?? 0;
  byId("stat-accounts").textContent = stats.accounts ?? 0;
  byId("stat-sessions").textContent = stats.sessions ?? 0;
  byId("stat-failures").textContent = stats.failed ?? 0;
}

function setMessageStrategy(prefix, strategy) {
  const radio = document.querySelector(`input[name="${prefix}-strategy"][value="${strategy}"]`);
  if (radio) radio.checked = true;
  document.querySelectorAll(".mode-option").forEach((option) => {
    const input = option.querySelector(`input[name="${prefix}-strategy"]`);
    if (input) option.classList.toggle("active", input.checked);
  });
  if (prefix === "msg") {
    byId("msg-section-single").style.display = strategy === "single" ? "" : "none";
    byId("msg-section-folder").style.display = strategy === "folder" ? "" : "none";
    byId("msg-section-model").style.display = strategy === "model" ? "" : "none";
  }
}

// --- step 3: site filters, sessions, diagnostics ----------------------------
function fillSiteSelect(selectId, sites, keepValue) {
  const el = byId(selectId);
  if (!el) return;
  const previous = keepValue !== undefined ? keepValue : el.value;
  const fragment = document.createDocumentFragment();
  const all = document.createElement("option");
  all.value = "all";
  all.textContent = "All sites";
  fragment.appendChild(all);
  sites.forEach((site) => {
    const option = document.createElement("option");
    option.value = site.id;
    option.textContent = site.name || site.id;
    fragment.appendChild(option);
  });
  el.replaceChildren(fragment);
  el.value = previous && Array.from(el.options).some((o) => o.value === previous) ? previous : "all";
}

function fillSessionSiteSelect(sites) {
  const el = byId("sess-site");
  if (!el) return;
  const previous = el.value || state.selectedSite;
  const fragment = document.createDocumentFragment();
  sites.forEach((site) => {
    const option = document.createElement("option");
    option.value = site.id;
    option.textContent = site.name || site.id;
    fragment.appendChild(option);
  });
  el.replaceChildren(fragment);
  if (previous && sites.some((site) => site.id === previous)) el.value = previous;
}

function renderDiagnostics(diag) {
  const list = byId("site-diag-checks");
  const badge = byId("site-diag-badge");
  if (!list || !badge) return;
  if (!diag) {
    list.replaceChildren();
    const li = document.createElement("li");
    li.className = "hint-text";
    li.textContent = "Pick a site to load its diagnostics.";
    list.appendChild(li);
    badge.textContent = "not loaded";
    badge.className = "status-pill";
    return;
  }
  state.diagnostics = diag;
  state.diagSiteID = diag.site_id || "";
  badge.textContent = diag.ready ? "ready" : "attention";
  badge.className = "status-pill " + (diag.ready ? "ok" : "warn");

  const fragment = document.createDocumentFragment();
  (diag.checks || []).forEach((check) => {
    const li = document.createElement("li");
    li.className = "diag-item " + (check.ok ? "ok" : "bad");
    li.dataset.check = check.name;
    const mark = document.createElement("span");
    mark.className = "diag-mark";
    mark.textContent = check.ok ? "✓" : "✕";
    const name = document.createElement("span");
    name.className = "diag-name";
    name.textContent = check.name;
    const detail = document.createElement("span");
    detail.className = "diag-detail";
    detail.textContent = check.detail || "";
    li.append(mark, name, detail);
    fragment.appendChild(li);
  });
  list.replaceChildren(fragment);

  byId("site-diag-coverage").textContent = `selectors ${diag.selector_filled}/${diag.selector_total}`;
  const history = diag.latency_history || [];
  byId("site-diag-latency").textContent = history.length
    ? `latency ${history[history.length - 1]} ms · ${history.length} probe(s)`
    : "latency —";
  byId("site-diag-sequences").textContent = `sequences ${(diag.sequence_files || []).length}`;
}

async function loadDiagnostics(siteID, silent) {
  const id = siteID || state.selectedSite;
  if (!id) return;
  try {
    const diag = await bridge.invoke("GetSiteDiagnostics", { site_id: id });
    renderDiagnostics(diag);
    if (!silent) setStatus("site-test-status", `Diagnostics loaded for ${diag.name || id}.`, "ok");
  } catch (error) {
    renderDiagnostics(null);
    if (!silent) setStatus("site-test-status", error.message || String(error), "error");
  }
}

function renderSessions(payload) {
  const body = byId("sess-table-body");
  if (!body) return;
  const sessions = (payload && payload.sessions) || [];
  state.sessions = sessions;
  body.replaceChildren();
  if (!sessions.length) {
    const row = document.createElement("tr");
    const cell = document.createElement("td");
    cell.colSpan = 6;
    cell.textContent = "No profiles yet for this site.";
    row.appendChild(cell);
    body.appendChild(row);
    return;
  }
  sessions.forEach((session) => {
    const row = document.createElement("tr");
    row.dataset.profile = session.profile;
    const cells = [
      session.profile || "default",
      String(session.cookies ?? 0),
      String(session.local_storage ?? 0),
      session.updated_at ? new Date(session.updated_at).toLocaleString() : "—",
      session.note || "—",
    ];
    cells.forEach((value) => {
      const cell = document.createElement("td");
      cell.textContent = value;
      row.appendChild(cell);
    });
    const action = document.createElement("td");
    const button = document.createElement("button");
    button.type = "button";
    button.className = "secondary-button small";
    button.textContent = "Delete";
    button.addEventListener("click", () => deleteSession(session.profile));
    action.appendChild(button);
    row.appendChild(action);
    body.appendChild(row);
  });
}

async function loadSessions(silent) {
  const el = byId("sess-site");
  const siteID = el ? el.value : state.selectedSite;
  if (!siteID) return;
  try {
    const payload = await bridge.invoke("GetSessions", { site_id: siteID });
    renderSessions(payload);
    if (!silent) setStatus("sess-status", `${(payload && payload.count) || 0} session(s).`, "ok");
  } catch (error) {
    if (!silent) setStatus("sess-status", error.message || String(error), "error");
  }
}

async function deleteSession(profile) {
  const siteID = byId("sess-site").value;
  if (!siteID || !profile) return;
  try {
    await bridge.invoke("DeleteSession", { site_id: siteID, profile });
    toast(`Session ${profile} deleted.`, "ok");
    await loadSessions(true);
    await refreshSnapshot(true, true);
  } catch (error) {
    setStatus("sess-status", error.message || String(error), "error");
  }
}

// --- sites -----------------------------------------------------------------
function renderSitePicker(sites) {
  const wrap = byId("site-picker");
  wrap.replaceChildren();
  sites.forEach((site) => {
    const card = document.createElement("label");
    card.className = "site-card" + (site.id === state.selectedSite ? " active" : "");
    card.dataset.site = site.id;

    const input = document.createElement("input");
    input.type = "radio";
    input.name = "site-choice";
    input.value = site.id;
    input.checked = site.id === state.selectedSite;
    input.addEventListener("change", () => {
      state.selectedSite = site.id;
      document.querySelectorAll(".site-card").forEach((el) => {
        el.classList.toggle("active", el.dataset.site === site.id);
      });
      renderSiteFacts();
      byId("site-driver-code").hidden = true;
      setStatus("site-test-status", "Not tested.");
      loadDiagnostics(site.id, true);
    });

    const top = document.createElement("div");
    top.className = "site-card-top";
    const name = document.createElement("span");
    name.className = "site-card-name";
    name.textContent = site.name || site.id;
    const badge = document.createElement("span");
    badge.className = badgeClass(site.status);
    badge.textContent = site.status || "—";
    top.append(name, badge);

    const meta = document.createElement("div");
    meta.className = "site-card-meta";
    meta.textContent = site.start_url || "—";

    const facts = document.createElement("div");
    facts.className = "site-card-facts";
    facts.textContent = `${site.family} · ${site.account_mode} · accounts ${site.accounts ?? 0}`;

    card.append(input, top, meta, facts);
    wrap.appendChild(card);
  });
}

function siteByID(id) {
  const sites = (state.snapshot && state.snapshot.sites) || [];
  return sites.find((site) => site.id === id) || null;
}

function renderSiteFacts() {
  const site = siteByID(state.selectedSite);
  byId("site-fact-url").textContent = site ? site.start_url || "—" : "—";
  byId("site-fact-family").textContent = site ? `${site.family} · ${site.auth_mode || "dom"}` : "—";
  byId("site-fact-account-mode").textContent = site ? site.account_mode || "—" : "—";
  byId("site-fact-entry-flow").textContent = site ? site.entry_flow || "—" : "—";
  const protection = site && site.protection_summary ? site.protection_summary : "—";
  byId("site-fact-protection").textContent = protection;
  byId("run-site-summary").textContent = site
    ? `${site.name} · ${site.family} · accounts ${site.accounts ?? 0}${site.has_session ? " · session saved" : ""}`
    : "Select a site in step 1.";
}

function renderSiteCheckList(sites) {
  const wrap = byId("site-check-list");
  wrap.replaceChildren();
  sites.forEach((site) => {
    const row = document.createElement("label");
    row.className = "site-check-row";

    const input = document.createElement("input");
    input.type = "checkbox";
    input.value = site.id;
    input.checked = state.selectedSites.includes(site.id);
    input.addEventListener("change", () => {
      if (input.checked) {
        if (!state.selectedSites.includes(site.id)) state.selectedSites.push(site.id);
      } else {
        state.selectedSites = state.selectedSites.filter((id) => id !== site.id);
      }
      byId("wsel-count").textContent = `${state.selectedSites.length} sites selected`;
      dirtyFields.add("wsel");
    });

    const body = document.createElement("div");
    const name = document.createElement("div");
    name.className = "site-check-name";
    name.textContent = site.name || site.id;
    const sub = document.createElement("div");
    sub.className = "site-check-sub";
    sub.textContent = `${site.family} · ${site.account_mode} · accounts ${site.accounts ?? 0} · driver ${site.driver_bytes ?? 0} B`;
    body.append(name, sub);

    const badge = document.createElement("span");
    badge.className = badgeClass(site.status);
    badge.textContent = site.status || "—";

    row.append(input, body, badge);
    wrap.appendChild(row);
  });
}

function renderSiteTable(sites) {
  const body = byId("sites-table-body");
  if (!sites.length) {
    body.innerHTML = '<tr><td colspan="7">No sites loaded.</td></tr>';
    return;
  }
  body.replaceChildren();
  sites.forEach((site) => {
    const tr = document.createElement("tr");
    const cells = [
      site.name || site.id,
      site.family || "—",
      site.account_mode || "—",
      String(site.accounts ?? 0),
      site.has_session ? "yes" : "no",
      `${site.driver_bytes ?? 0} B`,
    ];
    cells.forEach((value) => {
      const td = document.createElement("td");
      td.textContent = value;
      tr.appendChild(td);
    });
    const statusTd = document.createElement("td");
    const badge = document.createElement("span");
    badge.className = badgeClass(site.status);
    badge.textContent = site.status || "—";
    statusTd.appendChild(badge);
    tr.appendChild(statusTd);
    body.appendChild(tr);
  });
}

function renderAccountSiteSelect(sites) {
  const select = byId("acc-site");
  const previous = select.value;
  select.replaceChildren();
  sites.forEach((site) => {
    const option = document.createElement("option");
    option.value = site.id;
    option.textContent = `${site.name || site.id} (${site.accounts ?? 0} accounts)`;
    select.appendChild(option);
  });
  if (previous && sites.some((site) => site.id === previous)) select.value = previous;
  else if (state.selectedSite) select.value = state.selectedSite;
}

function renderAccountsTable() {
  const body = byId("acc-table-body");
  if (!state.accounts.length) {
    body.innerHTML = '<tr><td colspan="6">Nothing registered yet.</td></tr>';
    return;
  }
  body.replaceChildren();
  state.accounts.forEach((account, index) => {
    const tr = document.createElement("tr");
    [String(index + 1), account.nickname, account.site_id, account.gender, String(account.age)].forEach((value) => {
      const td = document.createElement("td");
      td.textContent = value;
      tr.appendChild(td);
    });
    const statusTd = document.createElement("td");
    const badge = document.createElement("span");
    badge.className = "badge ok";
    badge.textContent = account.status || "registered";
    statusTd.appendChild(badge);
    tr.appendChild(statusTd);
    body.appendChild(tr);
  });
}

// ---------------------------------------------------------------------------
// Actions
// ---------------------------------------------------------------------------
function renderBrowserStatus(status) {
  const line = byId("wk-browser-status");
  if (!line || !status) return;
  if (status.found) {
    line.textContent = `${status.name} · ${status.version || "version unknown"} · ${status.source}`;
    line.className = "status-line ok";
    if (byId("wk-browser-path") && !byId("wk-browser-path").value) {
      byId("wk-browser-path").placeholder = status.path;
    }
  } else {
    line.textContent = status.note || "No browser found.";
    line.className = "status-line error";
  }
}

async function detectBrowser(showToast) {
  try {
    const status = await bridge.invoke("GetBrowserStatus");
    renderBrowserStatus(status);
    if (showToast) toast(status.found ? `${status.name} detected` : "No browser found", status.found ? "ok" : "error");
    return status;
  } catch (error) {
    const line = byId("wk-browser-status");
    if (line) { line.textContent = error.message || String(error); line.className = "status-line error"; }
    return null;
  }
}

async function saveBrowserSettings() {
  await saveSection("browser", readBrowserSettings(), "wk-browser-status");
  await detectBrowser(false);
}

async function browserCheck() {
  const siteID = state.selectedSite;
  if (!siteID) { setStatus("site-browser-status", "Select a site first.", "error"); return; }
  const line = byId("site-browser-status");
  line.textContent = "Opening a real browser… (can take ~20s)";
  line.className = "status-line";
  try {
    const out = await bridge.invoke("TestSiteBrowser", { site_id: siteID, profile: "default" });
    const verdict = out.ok ? "PASS" : "NO COMPOSER";
    line.textContent = `${verdict} · phase=${out.phase} · messages=${out.messages} · ${out.probe || ""}` +
      (out.screenshot ? ` · shot: ${out.screenshot}` : "");
    line.className = "status-line " + (out.ok ? "ok" : "skip");
    await refreshSnapshot(true, true);
  } catch (error) {
    line.textContent = error.message || String(error);
    line.className = "status-line error";
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

async function saveSection(section, values, statusId) {
  try {
    setStatus(statusId, "Saving…");
    const snapshot = await bridge.invoke("SaveSettings", { section, values });
    dirtyFields.clear();
    applySnapshot(snapshot);
    setStatus(statusId, "Saved.", "ok");
    toast(`${section} saved.`);
    return true;
  } catch (error) {
    setStatus(statusId, error.message || String(error), "error");
    return false;
  }
}

async function testSelectedSite() {
  const siteID = state.selectedSite;
  if (!siteID) { setStatus("site-test-status", "Select a site first.", "error"); return; }
  try {
    setStatus("site-test-status", "Probing…");
    const result = await bridge.invoke("TestSite", { site_id: siteID });
    const verdict = result.ok ? "PASS" : "FAIL";
    setStatus("site-test-status", `${verdict} · ${result.message || "no message"} · ${result.latency_ms ?? 0} ms`,
      result.ok ? "ok" : "error");
    await refreshSnapshot(true);
    await loadDiagnostics(siteID, true);
  } catch (error) {
    setStatus("site-test-status", error.message || String(error), "error");
  }
}

async function viewDriver() {
  const siteID = state.selectedSite;
  if (!siteID) { setStatus("site-test-status", "Select a site first.", "error"); return; }
  try {
    const payload = await bridge.invoke("GetSiteDriver", { site_id: siteID });
    const pre = byId("site-driver-code");
    pre.textContent = `// site.json\n${payload.site_json || ""}\n\n// protection\n${payload.protection_note || "(none)"}\n\n// dom_driver.js\n${payload.dom_driver_js || ""}`;
    pre.hidden = false;
    pre.scrollIntoView({ block: "nearest" });
  } catch (error) {
    setStatus("site-test-status", error.message || String(error), "error");
  }
}

async function saveShowSetting() {
  await saveSection("show", readShowSetting(), "sf-status");
  renderSitePicker((state.snapshot && state.snapshot.sites) || []);
  renderSiteFacts();
}

async function saveSelection() {
  await saveSection("worker", readWorkerSettings(), "wsel-status");
}

async function registerAccounts() {
  try {
    const siteID = byId("acc-site").value;
    if (!siteID) { setStatus("acc-status", "No site available.", "error"); return; }
    setStatus("acc-status", "Registering…");
    // persist the account policy first so the backend uses the same numbers
    await bridge.invoke("SaveSettings", { section: "accounts", values: readAccountSettings() });
    const result = await bridge.invoke("AutoRegisterAccounts", { site_id: siteID, count: num("acc-count") });
    const accounts = (result && result.accounts) || [];
    state.accounts = accounts.concat(state.accounts).slice(0, 40);
    renderAccountsTable();
    setStatus("acc-status", `${accounts.length} account(s) created for ${siteID}.`, "ok");
    await refreshSnapshot(true);
  } catch (error) {
    setStatus("acc-status", error.message || String(error), "error");
  }
}

async function launchLogin() {
  try {
    const siteID = byId("acc-site").value;
    setStatus("acc-login-status", "Preparing profile…");
    const out = await bridge.invoke("LaunchLogin", { site_id: siteID, profile: text("acc-profile") || "default" });
    setStatus("acc-login-status", (out && out.message) || "Login queued.", "ok");
    await refreshSnapshot(true);
  } catch (error) {
    setStatus("acc-login-status", error.message || String(error), "error");
  }
}

async function testProxy() {
  try {
    setStatus("wk-proxy-status", "Dialling…");
    const out = await bridge.invoke("TestConnections", readProxy());
    const results = (out && out.results) || [];
    const box = byId("wk-proxy-results");
    box.replaceChildren();
    results.forEach((result) => {
      const p = document.createElement("p");
      p.className = result.ok ? "proxy-test-pass" : "proxy-test-fail";
      p.textContent = `${result.entry} — ${result.ok ? "PASS" : "FAIL"} (${result.latency_ms ?? 0} ms) ${result.message || ""}`.trim();
      box.appendChild(p);
    });
    const passed = results.filter((r) => r.ok).length;
    setStatus("wk-proxy-status", results.length ? `${passed}/${results.length} passed.` : "No entries to test.",
      passed === results.length && results.length ? "ok" : "error");
  } catch (error) {
    setStatus("wk-proxy-status", error.message || String(error), "error");
  }
}

async function loadFolder() {
  try {
    setStatus("msg-folder-status", "Loading…");
    const out = await bridge.invoke("LoadFolder");
    setStatus("msg-folder-status", `${(out && out.count) || 0} sequences loaded.`, "ok");
  } catch (error) {
    setStatus("msg-folder-status", error.message || String(error), "error");
  }
}

async function startShowRun() {
  if (state.running || state.busy.show) return;
  setBusy("show", true);
  try {
    if (!state.selectedSite) throw new Error("Select a site in step 1.");
    setStatus("run-status", "Saving Setting + Message…");
    await bridge.invoke("SaveSettings", { section: "show", values: readShowSetting() });
    await bridge.invoke("SaveSettings", { section: "message", values: readMessage("msg") });

    state.logs.show.entries = []; state.logs.show.after = 0; renderLogs("show");
    setStatus("run-status", "Starting…");
    const profile = document.querySelector('input[name="run-profile"]:checked');
    await bridge.invoke("StartRun", {
      mode: "show",
      sites: [state.selectedSite],
      profile: profile ? profile.value : "default",
    });
    state.running = true; renderRunning();
    setStatus("run-status", "Running. Live logs appear under step 5.", "ok");
    await refreshSnapshot(true);
  } catch (error) {
    setStatus("run-status", error.message || String(error), "error");
  } finally {
    setBusy("show", false);
  }
}

async function startWorker() {
  if (state.running || state.busy.worker) return;
  setBusy("worker", true);
  try {
    if (!state.selectedSites.length) throw new Error("Tick at least one site in step 1.");
    setStatus("worker-status", "Saving Sites + Message + Settings + Proxy…");
    await bridge.invoke("SaveSettings", { section: "worker", values: readWorkerSettings() });
    await bridge.invoke("SaveSettings", { section: "message", values: readMessage("wmsg") });
    await bridge.invoke("SaveSettings", { section: "proxy", values: readProxy() });

    state.logs.worker.entries = []; state.logs.worker.after = 0; renderLogs("worker");
    setStatus("worker-status", "Starting workers…");
    await bridge.invoke("StartRun", {
      mode: "worker",
      sites: state.selectedSites.slice(),
      profile: text("wk-profile") || "default",
    });
    state.running = true; renderRunning();
    setStatus("worker-status", "Started. One worker runs per selected site.", "ok");
    await refreshSnapshot(true);
  } catch (error) {
    setStatus("worker-status", error.message || String(error), "error");
  } finally {
    setBusy("worker", false);
  }
}

async function stopRun() {
  try {
    await bridge.invoke("StopRun");
    state.running = false;
    renderRunning();
    setStatus("worker-status", "Stopped.", "ok");
    setStatus("run-status", "Stopped.", "ok");
    await refreshSnapshot(true);
  } catch (error) {
    setStatus("worker-status", error.message || String(error), "error");
  }
}

// ---------------------------------------------------------------------------
// Live logs (one backend ring, rendered per mode)
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

function logSiteMatch(entry, siteFilter) {
  if (!siteFilter || siteFilter === "all") return true;
  return (entry.site || "") === siteFilter;
}

function renderLogs(prefix) {
  const list = byId(`${prefix}-log-list`);
  const filterEl = byId(`${prefix}-log-filter`);
  const emptyEl = byId(`${prefix}-log-empty`);
  if (!list) return;
  const filter = filterEl ? filterEl.value : "all";
  const siteFilterEl = byId(`${prefix}-log-site`);
  const siteFilter = siteFilterEl ? siteFilterEl.value : "all";
  const nearBottom = list.scrollHeight - list.scrollTop - list.clientHeight < 60;
  const fragment = document.createDocumentFragment();
  let shown = 0;

  for (const entry of state.logs[prefix].entries) {
    if (!logFilterMatch(entry, filter)) continue;
    if (!logSiteMatch(entry, siteFilter)) continue;
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

    const metaParts = [];
    if (entry.site) metaParts.push(entry.site);
    if (entry.meta) metaParts.push(entry.meta);
    if (metaParts.length) {
      const meta = document.createElement("p");
      meta.className = "log-meta";
      meta.textContent = metaParts.join(" · ");
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
    const page = await bridge.invoke("GetLogs", { after: bucket.after });
    if (page && Array.isArray(page.entries) && page.entries.length) {
      bucket.entries.push(...page.entries);
      bucket.after = page.version ?? bucket.entries.length;
      renderLogs(prefix);
    }
    if (page && page.cleared) {
      bucket.entries = [];
      bucket.after = page.version || 0;
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
    state.logs.show.entries = []; state.logs.show.after = 0;
    state.logs.worker.entries = []; state.logs.worker.after = 0;
    renderLogs("show");
    renderLogs("worker");
  } catch (error) {
    toast(error.message || String(error), "error");
  }
}

// ---------------------------------------------------------------------------
// Wiring
// ---------------------------------------------------------------------------
async function hydrateStep3() {
  await loadDiagnostics(state.selectedSite, true);
  await loadSessions(true);
  await detectBrowser(false);
}
function bindForms() {
  byId("theme-select").value = localStorage.getItem("kit-theme") || CONFIG.theme;
  document.documentElement.dataset.theme = byId("theme-select").value;

  ["msg", "wmsg"].forEach((prefix) => {
    document.querySelectorAll(`input[name="${prefix}-strategy"]`).forEach((input) => {
      input.addEventListener("change", () => {
        const picked = document.querySelector(`input[name="${prefix}-strategy"]:checked`);
        setMessageStrategy(prefix, picked ? picked.value : "single");
      });
    });
  });
}

function bindActions() {
  byId("theme-select").addEventListener("change", (event) => {
    document.documentElement.dataset.theme = event.target.value;
    localStorage.setItem("kit-theme", event.target.value);
  });
  byId("demo-hide").addEventListener("click", () => { byId("demo-banner").hidden = true; });

  // Show mode
  byId("site-test").addEventListener("click", testSelectedSite);
  byId("site-diag-refresh").addEventListener("click", () => loadDiagnostics(state.selectedSite));
  byId("site-browser-check").addEventListener("click", browserCheck);
  byId("show-log-site").addEventListener("change", () => renderLogs("show"));
  byId("worker-log-site").addEventListener("change", () => renderLogs("worker"));
  byId("sess-refresh").addEventListener("click", () => loadSessions(false));
  byId("sess-site").addEventListener("change", () => loadSessions(false));
  byId("site-driver-load").addEventListener("click", viewDriver);
  byId("sf-save").addEventListener("click", saveShowSetting);
  byId("msg-save").addEventListener("click", () => saveSection("message", readMessage("msg"), "msg-status"));
  byId("msg-load-folder").addEventListener("click", loadFolder);
  byId("run-start").addEventListener("click", startShowRun);
  byId("show-refresh").addEventListener("click", () => refreshSnapshot(false, true).then(() => renderSitePicker((state.snapshot && state.snapshot.sites) || [])).catch(() => {}));
  byId("show-log-filter").addEventListener("change", () => renderLogs("show"));
  byId("show-log-clear").addEventListener("click", () => clearLogs("show"));

  // Worker mode
  byId("wsel-save").addEventListener("click", saveSelection);
  byId("acc-register").addEventListener("click", registerAccounts);
  byId("acc-login").addEventListener("click", launchLogin);
  byId("wmsg-save").addEventListener("click", () => saveSection("message", readMessage("wmsg"), "wmsg-status"));
  byId("wk-save").addEventListener("click", () => saveSection("worker", readWorkerSettings(), "wk-status"));
  byId("wk-proxy-save").addEventListener("click", () => saveSection("proxy", readProxy(), "wk-proxy-status"));
  byId("wk-proxy-test").addEventListener("click", testProxy);
  byId("wk-browser-save").addEventListener("click", saveBrowserSettings);
  byId("wk-browser-detect").addEventListener("click", () => detectBrowser(true));
  byId("worker-start").addEventListener("click", startWorker);
  byId("worker-stop").addEventListener("click", stopRun);
  byId("worker-refresh").addEventListener("click", () => refreshSnapshot(false, true).catch(() => {}));
  byId("worker-log-filter").addEventListener("change", () => renderLogs("worker"));
  byId("worker-log-clear").addEventListener("click", () => clearLogs("worker"));
}

async function bootstrap() {
  bindTabs();
  bindForms();
  bindGlobalState();
  bindActions();
  renderAccountsTable();
  activateTab("show");

  await resolveBridge();
  renderBackendPill();
  subscribeEvents();

  for (let attempt = 0; attempt < 25; attempt++) {
    try {
      await refreshSnapshot(true);
      break;
    } catch (_) {
      await new Promise((resolve) => setTimeout(resolve, 200));
    }
  }

  hydrateStep3().catch(() => {});

  setInterval(() => refreshLogs(state.tab), CONFIG.pollLogsMs);
  setInterval(() => { refreshSnapshot(true).catch(() => {}); }, CONFIG.pollStatsMs);
}

window.addEventListener("error", (event) => reportFrontendError("Uncaught error", event.error || event.message));
window.addEventListener("unhandledrejection", (event) => reportFrontendError("Unhandled rejection", event.reason));
window.addEventListener("DOMContentLoaded", () => {
  bootstrap().catch((error) => {
    reportFrontendError("Bootstrap failed", error);
    setStatus("run-status", error.message || String(error), "error");
  });
});
