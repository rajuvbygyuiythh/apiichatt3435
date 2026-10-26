// =============================================================================
//  mock-backend.js — in-page fake backend so the UI runs with zero dependencies
// -----------------------------------------------------------------------------
//  Delete this file (and its <script> tag) once a real backend exists.
//  It implements exactly the same method names as the Go App struct, so app.js
//  cannot tell the difference — that is the whole point of the bridge layer.
//  Useful for: design review, screenshots, UI-only PRs, and non-Go stacks
//  (Node/Python backends just have to serve the same JSON shapes over /api).
// =============================================================================
(function () {
  const settings = {
    setup: {
      name: "demo-workspace",
      mode: "balanced",
      concurrency: 2,
      timeout_seconds: 60,
      headless: false,
      notes: "One line = one rule. Everything here is placeholder data.",
    },
    connections: {
      enabled: false,
      scheme: "http",
      entries: "127.0.0.1:8080\nsocks5://user:pass@127.0.0.1:1080",
    },
    credentials: {
      base_url: "https://api.example.com/v1",
      api_key: "",
      model: "default-model",
      temperature: 0.8,
    },
    actions: {
      strategy: "single",
      sequence: "hello %username%\nchecking now, one moment\nok, all set",
      targets: "@demo_one\n@demo_two",
    },
    timings: {
      poll_min: 5, poll_max: 12,
      action_min: 3, action_max: 8,
      timeout_min: 60, timeout_max: 90,
      retry_min: 10, retry_max: 30,
    },
    profile: { name: "default", backend: "rod", headless: false },
  };

  const stats = { profiles: 2, completed: 0, queued: 0, failed: 0 };
  const items = [
    { name: "alpha", profile: "default", state: "ok", updated: "10:02:11" },
    { name: "bravo", profile: "session", state: "pending", updated: "10:02:40" },
  ];

  let entries = [];
  let version = 0;
  let running = false;
  let timer = null;
  let step = 0;

  const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

  function push(kind, text, meta) {
    version += 1;
    entries.push({
      id: version,
      time: new Date().toLocaleTimeString([], { hour12: false }),
      kind,
      text,
      meta: meta || "",
    });
    if (entries.length > 400) entries = entries.slice(-400);
  }

  const script = [
    () => push("system", "worker picked up the job", "rule: scheduler"),
    () => push("out", "hello @demo_one", "rule: first message · ok"),
    () => push("in", "hey, who is this?", "detail: inbound"),
    () => push("out", "checking now, one moment", "rule: delay · ok"),
    () => push("skip", "target not responding", "rule: timeout 60s"),
    () => push("system", "rotating to next target", "detail: queue 1 left"),
    () => push("out", "ok, all set", "rule: closing line · ok"),
    () => push("error", "connection refused (demo error)", "detail: retry in 10s"),
    () => push("system", "job finished: 1 ok, 1 skipped", "detail: cleanup done"),
  ];

  function startTicker() {
    if (timer) return;
    timer = setInterval(() => {
      const line = script[step % script.length];
      line();
      step += 1;
      if (step % 4 === 0) stats.completed += 1;
      if (step % 9 === 0) stats.failed += 1;
    }, 1100);
  }

  function stopTicker() {
    if (timer) clearInterval(timer);
    timer = null;
  }

  const mock = {
    async GetSnapshot() {
      await sleep(40);
      return {
        settings,
        strategy: settings.actions.strategy,
        stats: { ...stats, profiles: 2, queued: items.filter((i) => i.state === "pending").length },
        items,
        runtime: {
          running,
          kind: "mock",
          data_dir: "/mock/data",
          version: "design-kit-starter",
        },
      };
    },

    async SaveSettings(section, values) {
      await sleep(120);
      if (settings[section] && values && typeof values === "object") {
        settings[section] = { ...settings[section], ...values };
      }
      if (section === "actions" && values && values.strategy) {
        settings.actions.strategy = values.strategy;
      }
      return this.GetSnapshot();
    },

    async TestConnection(payload) {
      await sleep(420);
      const raw = String((payload && payload.entries) || "").split(/\r?\n/).map((l) => l.trim()).filter(Boolean);
      return {
        results: raw.map((entry, index) => ({
          entry,
          ok: index % 3 !== 2,
          latency_ms: 40 + ((index * 37) % 180),
          message: index % 3 === 2 ? "timeout" : "ok",
        })),
      };
    },

    async TestAPI(credentials) {
      await sleep(520);
      if (!credentials || !String(credentials.base_url || "").trim()) {
        throw new Error("Base URL is empty");
      }
      if (!String(credentials.api_key || "").trim()) {
        throw new Error("API key is empty (demo backend never calls a real API)");
      }
      return "Demo: 200 OK · 1 choice · 12 tokens";
    },

    async StartRun(payload) {
      await sleep(280);
      if (running) return { ok: false, message: "already running" };
      running = true;
      stats.completed = 0;
      stats.failed = 0;
      step = 0;
      push("system", `run started (profile: ${(payload && payload.profile) || "default"})`, "detail: demo mode");
      startTicker();
      return { ok: true, message: "started" };
    },

    async StopRun() {
      await sleep(120);
      running = false;
      stopTicker();
      push("system", "run stopped by user", "detail: cleanup done");
      return { ok: true };
    },

    async LaunchLogin(payload) {
      await sleep(200);
      push("system", `login window requested (${(payload && payload.backend) || "rod"})`, "detail: demo mode");
      return { ok: true, message: "login requested" };
    },

    async GetLogs(after) {
      await sleep(30);
      const from = Number(after || 0);
      return {
        version,
        entries: entries.filter((entry) => entry.id > from),
        cleared: from > version,
        running,
        stats: { ...stats },
      };
    },

    async ClearLogs() {
      await sleep(60);
      entries = [];
      return { ok: true };
    },

    async OpenDataFolder() {
      await sleep(60);
      return "/mock/data (demo path — no folder is opened)";
    },

    async LoadFolder() {
      await sleep(360);
      return {
        count: 3,
        names: ["sequence-a.txt", "sequence-b.txt", "sequence-c.txt"],
      };
    },
  };

  window.__consoleMock = mock;
})();
