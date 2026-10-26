import { JSDOM } from "jsdom";
import fs from "node:fs";
const dir = new URL("../cmd/console/ui/", import.meta.url).pathname;
const html = fs.readFileSync(dir + "/index.html", "utf8").replace(/<script[^>]*><\/script>/g, "");
const dom = new JSDOM(html, { url: "http://localhost:8090/", pretendToBeVisual: true, runScripts: "outside-only" });
const { window } = dom;
const errors = [];
window.addEventListener("error", (e) => errors.push(String(e.error || e.message)));
window.eval(fs.readFileSync(dir + "/js/mock-backend.js", "utf8"));
window.eval(fs.readFileSync(dir + "/app.js", "utf8"));
window.document.dispatchEvent(new window.Event("DOMContentLoaded"));
const wait = (ms) => new Promise((r) => setTimeout(r, ms));
const $ = (id) => window.document.getElementById(id);
const ok = (label, cond, extra = "") => console.log((cond ? "PASS " : "FAIL ") + label + (extra ? " — " + extra : ""));

await wait(1200);
ok("boot on mock", $("backend-pill").textContent === "demo");
ok("5 site cards rendered", window.document.querySelectorAll(".site-card").length === 5);
ok("site facts filled", $("site-fact-url").textContent.includes("omezy"));
ok("stats: sites", $("stat-sites").textContent === "5");
ok("stats: accounts", $("stat-accounts").textContent === "6");
ok("worker checklist rows", window.document.querySelectorAll(".site-check-row").length === 5);
ok("site table rows", $("sites-table-body").querySelectorAll("tr").length === 5);
ok("worker summary", $("worker-summary").textContent.includes("5 sites"));

// pick a different site
const vibyCard = [...window.document.querySelectorAll(".site-card")].find((c) => c.dataset.site === "viby");
vibyCard.querySelector("input").dispatchEvent(new window.Event("change"));
await wait(100);
ok("site switch updates facts", $("site-fact-url").textContent.includes("viby"));
ok("run summary updates", $("run-site-summary").textContent.includes("Viby"));

// subnav show all 5
for (const id of ["show-setting", "show-message", "show-run", "show-logs", "show-site"]) {
  window.document.querySelector(`[data-subpanel="${id}"]`).click();
  ok("subnav " + id, window.document.querySelector(`.subpanel[data-subpanel="${id}"]`).classList.contains("active"));
}

// save setting
$("sf-chance").value = "45";
$("sf-chance").dispatchEvent(new window.Event("input"));
$("sf-save").click();
await wait(500);
ok("save show setting", $("sf-status").textContent === "Saved.");
await wait(3500);
ok("draft survives polling", $("sf-chance").value === "45");

// message strategy switch + save
window.document.querySelector('[data-subpanel="show-message"]').click();
$("msg-strategy-folder").checked = true;
$("msg-strategy-folder").dispatchEvent(new window.Event("change"));
ok("folder block visible", $("msg-section-folder").style.display === "" && $("msg-section-single").style.display === "none");
$("msg-save").click();
await wait(400);
ok("save message", $("msg-status").textContent === "Saved.");

// site test + driver
window.document.querySelector('[data-subpanel="show-site"]').click();
$("site-test").click();
await wait(900);
ok("test site verdict", /PASS|FAIL/.test($("site-test-status").textContent), $("site-test-status").textContent);
$("site-driver-load").click();
await wait(400);
ok("driver viewer filled", !$("site-driver-code").hidden && $("site-driver-code").textContent.includes("site.json"));

// ---------- step 3: diagnostics ----------
window.document.querySelector('[data-subpanel="show-site"]').click();
await wait(400);
const diagItems = [...window.document.querySelectorAll("#site-diag-checks .diag-item")];
ok("diagnostics rendered", diagItems.length >= 7, diagItems.length + " checks");
ok("diagnostics keeps check order", diagItems[0].dataset.check === "site.json parsed");
ok("diagnostics badge", /ready|attention/.test($("site-diag-badge").textContent), $("site-diag-badge").textContent);
ok("diagnostics coverage text", $("site-diag-coverage").textContent.includes("/11"), $("site-diag-coverage").textContent);
ok("diagnostics sequences counted", $("site-diag-sequences").textContent.includes("3"), $("site-diag-sequences").textContent);
$("site-diag-refresh").click();
await wait(400);
ok("diagnostics refresh", window.document.querySelectorAll("#site-diag-checks .diag-item").length >= 7);

// switching site reloads diagnostics for that site
const camgoCard = [...window.document.querySelectorAll(".site-card")].find((c) => c.dataset.site === "camgo");
camgoCard.querySelector("input").dispatchEvent(new window.Event("change"));
await wait(400);
ok("diagnostics follow site switch", $("site-diag-coverage").textContent.length > 0);

// ---------- step 3: sessions ----------
window.document.querySelector('[data-subpanel="worker-accounts"]').click();
await wait(300);
$("sess-site").value = "omezy";
$("sess-site").dispatchEvent(new window.Event("change"));
await wait(300);
ok("session rows", $("sess-table-body").querySelectorAll("tr").length === 2, $("sess-table-body").querySelectorAll("tr").length + " rows");
ok("session profile cell", $("sess-table-body").textContent.includes("default"));
const deleteButtons = [...$("sess-table-body").querySelectorAll("button")];
deleteButtons[deleteButtons.length - 1].click();
await wait(500);
ok("session deleted", $("sess-table-body").querySelectorAll("tr").length === 1, $("sess-table-body").textContent);

// log site filter options exist
ok("show log site options", $("show-log-site").options.length >= 5);
ok("worker log site options", $("worker-log-site").options.length >= 5);

// ---------- step 4: browser driver ----------
window.document.querySelector('.nav-button[data-tab="worker"]').click();
window.document.querySelector('[data-subpanel="worker-settings"]').click();
await wait(300);
ok("browser status line", $("wk-browser-status").textContent.includes("Google Chrome"), $("wk-browser-status").textContent);
ok("browser headless default", $("wk-browser-headless").checked === true);
ok("page timeout default", $("wk-page-timeout").value === "45000", $("wk-page-timeout").value);
$("wk-page-timeout").value = "30000";
$("wk-page-timeout").dispatchEvent(new window.Event("input"));
$("wk-browser-save").click();
await wait(600);
ok("save browser settings", /Chrome|Saved/.test($("wk-browser-status").textContent), $("wk-browser-status").textContent);
$("wk-browser-detect").click();
await wait(400);
ok("detect browser", $("wk-browser-status").textContent.includes("Google Chrome"));

window.document.querySelector('.nav-button[data-tab="show"]').click();
window.document.querySelector('[data-subpanel="show-site"]').click();
const readyCard = [...window.document.querySelectorAll(".site-card")].find((c) => c.dataset.site === "omezy");
readyCard.querySelector("input").dispatchEvent(new window.Event("change"));
await wait(300);
$("site-browser-check").click();
await wait(1400);
ok("browser check verdict", $("site-browser-status").textContent.includes("PASS"), $("site-browser-status").textContent);
ok("browser check reports screenshot", $("site-browser-status").textContent.includes(".png"), $("site-browser-status").textContent);
ok("browser check shows phase", $("site-browser-status").textContent.includes("phase="), $("site-browser-status").textContent);

// worker: selection + accounts
window.document.querySelector('.nav-button[data-tab="worker"]').click();
await wait(200);
ok("worker panel active", window.document.querySelector('[data-panel="worker"]').classList.contains("active"));
const thirdCheck = window.document.querySelectorAll(".site-check-row input")[2];
thirdCheck.checked = true;
thirdCheck.dispatchEvent(new window.Event("change"));
$("wsel-save").click();
await wait(500);
ok("save selection", $("wsel-status").textContent === "Saved.");
ok("selection count text", $("wsel-count").textContent.startsWith("3"), $("wsel-count").textContent);

window.document.querySelector('[data-subpanel="worker-accounts"]').click();
$("acc-count").value = "2";
$("acc-count").dispatchEvent(new window.Event("input"));
$("acc-register").click();
await wait(900);
ok("accounts registered", $("acc-table-body").querySelectorAll("tr").length === 2, $("acc-status").textContent);
ok("account rows show nicknames", /\w+\d+/.test($("acc-table-body").textContent));

// proxy test
window.document.querySelector('[data-subpanel="worker-settings"]').click();
$("wk-proxy-entries").value = "127.0.0.1:8080\n10.0.0.9:1080";
$("wk-proxy-entries").dispatchEvent(new window.Event("input"));
$("wk-proxy-test").click();
await wait(900);
ok("proxy results", $("wk-proxy-results").children.length === 2, $("wk-proxy-status").textContent);

// start worker + logs
$("worker-start").click();
await wait(1400);
ok("worker running", $("running-pill").textContent === "Running");
await wait(5200);
ok("log rows", $("worker-log-list").children.length > 0, $("worker-log-list").children.length + " rows");
ok("log kinds", [...window.document.querySelectorAll("#worker-log-list .log-badge")].map((b) => b.textContent).filter((v, i, a) => a.indexOf(v) === i).length >= 2);
$("worker-log-filter").value = "io";
$("worker-log-filter").dispatchEvent(new window.Event("change"));
await wait(150);
ok("filter narrows", $("worker-log-list").children.length <= 6);
$("worker-log-filter").value = "all";
$("worker-log-filter").dispatchEvent(new window.Event("change"));
const seenSites = [...window.document.querySelectorAll("#worker-log-list .log-meta")].map((el) => el.textContent.split(" · ")[0]);
const siteOption = [...$("worker-log-site").options].map((o) => o.value).find((v) => v !== "all" && seenSites.includes(v)) || "all";
$("worker-log-site").value = siteOption;
$("worker-log-site").dispatchEvent(new window.Event("change"));
await wait(150);
const siteRows = [...window.document.querySelectorAll("#worker-log-list .log-meta")].map((el) => el.textContent);
ok("site filter shows only one site", siteRows.length > 0 && siteRows.every((t) => t.includes(siteOption)), siteOption + " · " + siteRows.length + " rows");
$("worker-log-site").value = "all";
$("worker-log-site").dispatchEvent(new window.Event("change"));
$("worker-log-filter").value = "all";
$("worker-log-filter").dispatchEvent(new window.Event("change"));
$("worker-stop").click();
await wait(700);
ok("stopped", $("running-pill").textContent === "Stopped");
$("worker-log-clear").click();
await wait(400);
ok("log cleared", $("worker-log-list").children.length === 0);

// themes
for (const t of ["ocean", "light", "forest"]) {
  $("theme-select").value = t;
  $("theme-select").dispatchEvent(new window.Event("change"));
  ok("theme " + t, window.document.documentElement.dataset.theme === t);
}
// ---------- step 5: footer + runtime info ----------
ok("footer version", /^v\d/.test($("footer-version").textContent), $("footer-version").textContent);
ok("footer data path", $("footer-data").textContent.length > 0, $("footer-data").textContent);
ok("browser pill ready", $("browser-pill").textContent === "ready", $("browser-pill").textContent);
$("open-data").click();
await wait(300);
ok("open data folder toast", $("toast-wrap").textContent.includes("data"), $("toast-wrap").textContent.slice(0, 60));
ok("no runtime errors", errors.length === 0, errors.join(" | "));
window.close();
process.exit(0);
