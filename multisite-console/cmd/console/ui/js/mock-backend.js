// =============================================================================
//  mock-backend.js — in-page fake backend for the Multi-Site Console
// -----------------------------------------------------------------------------
//  Implements the exact method names the Go core exposes, so the dashboard can
//  be designed, screenshotted and UI-tested with no toolchain at all. The
//  bridge prefers a real backend when one answers; this file is inert then.
//  Delete this file (and its <script> tag) for a release build if you prefer.
// =============================================================================
(function () {
  const SITES = [
    {
      id: "camgo", name: "Camgo.com (Text & Video)", family: "family_b", account_mode: "auto_register",
      enabled: true, start_url: "https://camgo.com/", auth_mode: "dom_only",
      entry_flow: "Select Text Chat toggle -> Add interests -> Start",
      protection_summary: "Video-first platform; text panel only appears after Start. Wait for the composer before sending.",
      accounts: 2, has_session: true, driver_bytes: 14210, status: "ready",
      diag_ready: true, selector_filled: 8, selector_total: 11, sessions: 1, sequences: 3,
    },
    {
      id: "chat42", name: "Chat42.com (Instant Name -> 1v1)", family: "family_d", account_mode: "auto_register",
      enabled: true, start_url: "https://chat42.com/", auth_mode: "dom_only",
      entry_flow: "Pick a name -> 18+ tick -> Start chat",
      protection_summary: "Nickname form is required before the room opens; reuse the saved profile to skip it.",
      accounts: 1, has_session: false, driver_bytes: 13880, status: "ready",
      diag_ready: false, selector_filled: 7, selector_total: 11, sessions: 0, sequences: 3,
    },
    {
      id: "chatusa", name: "ChatUSA.club (Fast Stranger Match)", family: "family_e", account_mode: "auto_register",
      enabled: true, start_url: "https://www.chatusa.club/stranger-chat", auth_mode: "dom_only",
      entry_flow: "Open stranger-chat -> start matching",
      protection_summary: "Very fast match cycle; cap actions per minute and keep replies paced.",
      accounts: 0, has_session: false, driver_bytes: 13542, status: "needs-setup",
      diag_ready: false, selector_filled: 6, selector_total: 11, sessions: 0, sequences: 3,
    },
    {
      id: "omezy", name: "Omezy.io (Text Roulette)", family: "family_a", account_mode: "auto_register",
      enabled: true, start_url: "https://omezy.io/text/", auth_mode: "dom_only",
      entry_flow: "Chat anonymously -> 18+ Agree modal -> Start / Esc / Send",
      protection_summary: "18+ modal blocks the composer until accepted; Escape skips, Enter sends.",
      accounts: 3, has_session: true, driver_bytes: 14502, status: "ready",
      diag_ready: true, selector_filled: 9, selector_total: 11, sessions: 2, sequences: 3,
    },
    {
      id: "viby", name: "Viby.chat (Guest 1-on-1 Text)", family: "family_c", account_mode: "auto_register",
      enabled: true, start_url: "https://viby.chat/free-chat-rooms", auth_mode: "dom_only",
      entry_flow: "Guest entry -> Private 1-on-1 Text Chat",
      protection_summary: "Free guest mode allows up to 10 chats per day per browser session; clear profile storage to reset.",
      accounts: 0, has_session: false, driver_bytes: 14430, status: "needs-setup",
      diag_ready: false, selector_filled: 6, selector_total: 11, sessions: 0, sequences: 3,
    },
  ];

  const settings = {
    show: {
      site_id: "omezy",
      profile: "default",
      first: {
        mode: "bot", chance: 70, wait_mode: "unlimited",
        timeout_min: 30, timeout_max: 60, reply_min: 4, reply_max: 9,
        no_reply_min: 60, no_reply_max: 90, auto_skip: true,
      },
    },
    worker: {
      sites: ["omezy", "camgo"], profile: "default", headless: true, concurrency: 1,
      poll_min: 5, poll_max: 12, action_min: 3, action_max: 8,
      timeout_min: 60, timeout_max: 90, retry_min: 10, retry_max: 30,
    },
    message: { strategy: "single", sequence: "hey there\nwhat's up?", targets: "", max_lines: 3 },
    proxy: { enabled: false, scheme: "http", entries: "" },
    browser: { path: "", headless: true, page_timeout_ms: 45000, keep_profile: true },
    accounts: { count: 2, gender: "female", age: 21, nickname_pool: "Sophia,Emma,Olivia,Mia,Chloe,Ava" },
  };

  let entries = [];
  let version = 0;
  let running = false;
  let runMode = "";
  let timer = null;
  let stats = { sites: SITES.length, enabled: 5, accounts: 6, sessions: 2, running: 0, completed: 0, failed: 0 };
  let accountsCreated = [];
  const latencies = {};
  const lastProbes = {};
  const profiles = {
    omezy: [
      { site_id: "omezy", profile: "default", cookies: 12, local_storage: 4, http_status: 200, note: "18+ modal accepted", updated_at: new Date().toISOString(), file_path: "sites/omezy/profiles/default/session.json" },
      { site_id: "omezy", profile: "warmup", cookies: 9, local_storage: 2, http_status: 200, note: "kept for retry", updated_at: new Date(Date.now() - 86400000).toISOString(), file_path: "sites/omezy/profiles/warmup/session.json" },
    ],
    camgo: [
      { site_id: "camgo", profile: "default", cookies: 15, local_storage: 6, http_status: 200, note: "text toggle remembered", updated_at: new Date().toISOString(), file_path: "sites/camgo/profiles/default/session.json" },
    ],
  };
  const SEQUENCES = [
    { name: "a.txt", lines: ["hey there", "what's up?", "how is your day going"] },
    { name: "b.txt", lines: ["hi", "where are you from?", "nice"] },
    { name: "c.txt", lines: ["hello", "do you like music?", "me too"] },
  ];

  const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

  function push(kind, text, meta, site) {
    version += 1;
    entries.push({ id: version, time: new Date().toLocaleTimeString([], { hour12: false }), kind, text, meta: meta || "", site: site || "" });
    if (entries.length > 400) entries = entries.slice(-400);
  }

  function siteByID(id) {
    return SITES.find((site) => site.id === id) || null;
  }

  function recomputeStats() {
    stats.sites = SITES.length;
    stats.enabled = SITES.filter((site) => site.enabled).length;
    stats.accounts = SITES.reduce((sum, site) => sum + (site.accounts || 0), 0);
    stats.sessions = SITES.filter((site) => site.has_session).length;
    stats.running = running ? 1 : 0;
  }

  const script = [
    (site) => push("system", `opening ${site.name} (${site.family} · ${site.account_mode})`, `${site.id}/site.json`, site.id),
    (site) => push("system", `onboarding: ${site.entry_flow}`, "auto-onboard", site.id),
    (site) => push("out", "hey there", "line 1/2 · single", site.id),
    (site) => push("in", "hey, how's it going?", "partner reply", site.id),
    (site) => push("out", "what's up?", "line 2/2 · single", site.id),
    (site) => push("skip", "partner went quiet, rotating", "no-reply timeout 60s", site.id),
  ];

  function startTicker(siteIDs) {
    if (timer) clearInterval(timer);
    let step = 0;
    let index = 0;
    timer = setInterval(() => {
      const site = siteByID(siteIDs[index % siteIDs.length]);
      if (!site) return;
      script[step % script.length](site);
      step += 1;
      if (step % script.length === 0) {
        index += 1;
        stats.completed += 1;
      }
      recomputeStats();
    }, 1200);
  }

  function stopTicker() {
    if (timer) clearInterval(timer);
    timer = null;
  }

  const mock = {
    async GetSnapshot() {
      await sleep(30);
      recomputeStats();
      return {
        settings,
        strategy: settings.message.strategy,
        sites: SITES.map((site) => ({ ...site })),
        stats: { ...stats },
        runtime: {
          running, mode: runMode, kind: "mock", data_dir: "data (demo)", log_path: "data/logs.txt (demo)",
          version: "0.5.0-step5", started_at: "09:00:00", sites: SITES.length,
        },
      };
    },

    async SaveSettings(payload) {
      await sleep(120);
      const section = payload && payload.section;
      const values = (payload && payload.values) || {};
      if (!settings[section]) throw new Error(`unknown settings section "${section}"`);
      settings[section] = { ...settings[section], ...values };
      if (section === "message") {
        settings.message.max_lines = Number(settings.message.max_lines) || 1;
      }
      if (section === "worker") {
        settings.worker.sites = (values.sites || []).filter((id) => siteByID(id));
      }
      return this.GetSnapshot();
    },

    async GetLogs(payload) {
      await sleep(20);
      const after = Number((payload && payload.after) || 0);
      recomputeStats();
      if (after > version) return { version, entries: [], cleared: true, running, stats: { ...stats } };
      return {
        version,
        entries: entries.filter((entry) => entry.id > after),
        cleared: false,
        running,
        stats: { ...stats },
      };
    },

    async ClearLogs() {
      await sleep(40);
      entries = [];
      version += 1;
      return { ok: true };
    },

    async TestSite(payload) {
      await sleep(600);
      const site = siteByID(payload && payload.site_id);
      if (!site) throw new Error(`unknown site "${payload && payload.site_id}"`);
      const ok = site.id !== "chatusa";
      const result = {
        entry: site.start_url, ok,
        ready: ok,
        latency_ms: 180 + Math.floor(Math.random() * 220),
        http: ok ? 200 : 403,
        final_url: site.start_url,
        redirects: site.id === "viby" ? 1 : 0,
        title: ok ? `${site.name.split(" (")[0]} — chat` : "",
        bytes: ok ? 48000 + Math.floor(Math.random() * 20000) : 900,
        evidence: ok ? ["textarea composer", "websocket reference", "chat keyword"] : [],
        message: ok ? "HTTP 200" : "HTTP 403 (demo: protected)",
      };
      latencies[site.id] = (latencies[site.id] || []).concat(result.latency_ms).slice(-10);
      lastProbes[site.id] = result;
      push(ok ? "system" : "error",
        ok ? `${site.start_url} reachable · ${result.message} · ${result.latency_ms} ms`
           : `${site.start_url} returned ${result.message}`,
        "site probe", site.id);
      site.last_test = result.message;
      return result;
    },

    async TestConnections(payload) {
      await sleep(500);
      const lines = String((payload && payload.entries) || "").split(/\r?\n/).map((l) => l.trim()).filter(Boolean);
      return {
        results: lines.map((entry, index) => ({
          entry, ok: index !== 1, latency_ms: 40 + index * 35,
          message: index === 1 ? "connection refused (demo)" : "tcp ok",
        })),
      };
    },

    async StartRun(payload) {
      await sleep(260);
      if (running) throw new Error("already running");
      const mode = payload && payload.mode;
      const sites = (payload && payload.sites) || [];
      if (!sites.length) throw new Error("select at least one site");
      if (mode !== "show" && mode !== "worker") throw new Error(`mode must be show or worker, got "${mode}"`);
      running = true;
      runMode = mode;
      stats.completed = 0;
      stats.failed = 0;
      push("system", `${mode} run started · ${sites.length} site(s) · profile ${payload.profile || "default"}`, "runner", "");
      startTicker(sites);
      return { ok: true, message: "started" };
    },

    async StopRun() {
      await sleep(100);
      running = false;
      runMode = "";
      stopTicker();
      push("system", "run stopped by user", "runner", "");
      recomputeStats();
      return { ok: true };
    },

    async LaunchLogin(payload) {
      await sleep(200);
      const site = siteByID(payload && payload.site_id);
      if (!site) throw new Error("unknown site");
      push("system", `login window requested for ${site.name} (profile ${payload.profile || "default"})`, "browser", site.id);
      push("skip", "browser driver is wired in step 4 — no window opened yet", "scaffold", site.id);
      return { ok: true, message: "login queued (demo)" };
    },

    async AutoRegisterAccounts(payload) {
      await sleep(500);
      const site = siteByID(payload && payload.site_id);
      if (!site) throw new Error("unknown site");
      const count = Number((payload && payload.count) || 1);
      if (count < 1 || count > 20) throw new Error("count must be between 1 and 20");
      const pool = String(settings.accounts.nickname_pool || "Sophia,Emma").split(",").map((s) => s.trim()).filter(Boolean);
      const created = [];
      for (let i = 0; i < count; i++) {
        const nickname = `${pool[i % pool.length]}${10 + Math.floor(Math.random() * 89)}`;
        const account = {
          id: `${site.id}-${Date.now()}-${i}`, site_id: site.id, site_name: site.name,
          nickname, gender: settings.accounts.gender, age: settings.accounts.age,
          status: "registered", created_at: new Date().toISOString(),
          file_path: `sites/${site.id}/accounts/${nickname}.json`,
        };
        created.push(account);
        push("out", `registered ${nickname} (${account.gender}, ${account.age})`, "auto-register", site.id);
      }
      site.accounts = (site.accounts || 0) + created.length;
      accountsCreated = accountsCreated.concat(created);
      recomputeStats();
      return { accounts: created, count: created.length };
    },

    async GetSiteDriver(payload) {
      await sleep(120);
      const site = siteByID(payload && payload.site_id);
      if (!site) throw new Error("unknown site");
      return {
        site_id: site.id,
        site_json: JSON.stringify({
          id: site.id, name: site.name, family: site.family, account_mode: site.account_mode,
          start_url: site.start_url, selectors: {
            composer_input: "textarea:not([disabled]), [contenteditable=\"true\"]",
            send_button: "button[type=\"submit\"], button[aria-label*=\"Send\" i]",
            skip_button: "button.skip, button.next, button[aria-label*=\"Skip\" i]",
          },
        }, null, 2),
        protection_note: `${site.name}\n${site.protection_summary}`,
        dom_driver_js: `// demo driver for ${site.id} (real file: sites/${site.id}/dom_driver.js)\n// isolated observer + auto-onboard agent lives here in the real build`,
        selector_family: site.family,
      };
    },

    async GetSiteDiagnostics(payload) {
      await sleep(140);
      const site = siteByID(payload && payload.site_id);
      if (!site) throw new Error(`unknown site "${payload && payload.site_id}"`);
      const history = latencies[site.id] || [220, 260];
      const probe = lastProbes[site.id] || null;
      const checks = [
        { name: "site.json parsed", ok: true, detail: `id=${site.id} family=${site.family} mode=${site.account_mode}` },
        { name: "site enabled", ok: site.enabled !== false, detail: "enabled in site.json" },
        { name: "start URL", ok: /^https:\/\//.test(site.start_url || ""), detail: site.start_url },
        { name: "dom_driver.js", ok: (site.driver_bytes || 0) > 500, detail: `${site.driver_bytes} bytes` },
        { name: "selector coverage", ok: (site.selector_filled || 0) >= 6, detail: `${site.selector_filled}/11 filled` },
        { name: "protection note", ok: !!site.protection_summary, detail: "present" },
        { name: "accounts or session", ok: (site.accounts || 0) > 0 || (site.sessions || 0) > 0, detail: `${site.accounts || 0} accounts · ${site.sessions || 0} sessions` },
        { name: "sequence scripts", ok: SEQUENCES.length > 0, detail: `${SEQUENCES.length} file(s) in data/sequences` },
      ];
      if (probe) checks.push({ name: "last probe", ok: probe.ok, detail: `${probe.message} · ${probe.latency_ms} ms` });
      return {
        site_id: site.id, name: site.name, family: site.family, enabled: site.enabled !== false,
        driver_bytes: site.driver_bytes || 0, site_json_valid: true,
        selector_total: 11, selector_filled: site.selector_filled || 0,
        empty_selectors: ["partner_name", "chat_ended_text"],
        selectors: {},
        protection_note: site.protection_summary || "",
        accounts: site.accounts || 0, sessions: (profiles[site.id] || []).length,
        checks, last_probe: probe || undefined,
        latency_history: history.slice(-10),
        sequence_files: SEQUENCES.map((seq) => seq.name),
        ready: checks.every((check) => check.ok),
      };
    },

    async GetBrowserStatus() {
      await sleep(80);
      return {
        found: true,
        path: "C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe",
        name: "Google Chrome",
        source: "common-dir",
        version: "Google Chrome 141.0.7390.66 (demo)",
      };
    },

    async TestSiteBrowser(payload) {
      await sleep(900);
      const site = siteByID(payload && payload.site_id);
      if (!site) throw new Error(`unknown site "${payload && payload.site_id}"`);
      const ok = site.status === "ready";
      push(ok ? "system" : "skip",
        ok ? `browser check OK · phase=chatting · messages=2 · family=${site.family} phase=chatting input=true msgs=2 prot=clean`
           : `browser check: page loaded but no composer yet · family=${site.family} phase=lobby input=false msgs=0 prot=clean`,
        "dom", site.id);
      return {
        site_id: site.id, ok, phase: ok ? "chatting" : "lobby",
        has_input: ok, can_send: ok, protection: "clean",
        messages: ok ? 2 : 0, revision: ok ? 4 : 0,
        probe: `family=${site.family} phase=${ok ? "chatting" : "lobby"} input=${ok} msgs=${ok ? 2 : 0} prot=clean`,
        browser: "Google Chrome (demo)", screenshot: ok ? `data/screenshots/${site.id}-demo.png` : "",
      };
    },

    async GetSessions(payload) {
      await sleep(90);
      const site = siteByID(payload && payload.site_id);
      if (!site) throw new Error(`unknown site "${payload && payload.site_id}"`);
      const list = (profiles[site.id] || []).map((session) => ({ ...session }));
      return { site_id: site.id, sessions: list, count: list.length };
    },

    async SaveSession(payload) {
      await sleep(120);
      const site = siteByID(payload && payload.site_id);
      if (!site) throw new Error(`unknown site "${payload && payload.site_id}"`);
      const session = {
        site_id: site.id, profile: (payload && payload.profile) || "default",
        cookies: 0, local_storage: 0, note: "created by console", status: "pending",
        updated_at: new Date().toISOString(),
        file_path: `sites/${site.id}/profiles/${(payload && payload.profile) || "default"}/session.json`,
      };
      profiles[site.id] = (profiles[site.id] || []).concat([session]);
      site.sessions = profiles[site.id].length;
      push("system", `${site.id}: session saved for profile "${session.profile}"`, "sessions", site.id);
      return session;
    },

    async DeleteSession(payload) {
      await sleep(110);
      const site = siteByID(payload && payload.site_id);
      if (!site) throw new Error(`unknown site "${payload && payload.site_id}"`);
      const profile = payload && payload.profile;
      if (!profile) throw new Error("missing required field: profile");
      profiles[site.id] = (profiles[site.id] || []).filter((session) => session.profile !== profile);
      site.sessions = profiles[site.id].length;
      site.has_session = site.sessions > 0;
      push("system", `${site.id}: session for "${profile}" deleted`, "sessions", site.id);
      return { ok: true, site_id: site.id, profile };
    },

    async LoadFolder() {
      await sleep(300);
      push("system", `folder loaded: ${SEQUENCES.length} sequence file(s) · ${SEQUENCES.reduce((sum, seq) => sum + seq.lines.length, 0)} line(s)`, "sequences", "");
      return {
        count: SEQUENCES.length,
        names: SEQUENCES.map((seq) => seq.name),
        sequences: SEQUENCES.map((seq) => ({ ...seq, lines: seq.lines.slice() })),
        total: SEQUENCES.reduce((sum, seq) => sum + seq.lines.length, 0),
        dir: "data/sequences (demo)",
      };
    },

    async OpenDataFolder() {
      await sleep(60);
      return { path: "data (demo — no folder opened)" };
    },
  };

  push("system", "console started · 5 sites · backend mock", "0.3.0-step3", "");
  push("system", "file log at data/logs.txt (demo)", "applog", "");
  push("system", "settings at data/settings.json (demo)", "storage", "");
  recomputeStats();

  window.__consoleMock = mock;
})();
