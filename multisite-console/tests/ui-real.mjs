import { JSDOM } from "jsdom";
import fs from "node:fs";
const BASE = process.env.BASE || "http://127.0.0.1:8090";
const dir = new URL("../cmd/console/ui/", import.meta.url).pathname;
const html = fs.readFileSync(dir + "/index.html", "utf8").replace(/<script[^>]*><\/script>/g, "");
const dom = new JSDOM(html, { url: BASE + "/", pretendToBeVisual: true, runScripts: "outside-only" });
const { window } = dom;
const errors = [];
window.addEventListener("error", (e) => errors.push(String(e.error || e.message)));
window.fetch = (input, init) => globalThis.fetch(new URL(String(input), BASE).toString(), init);
window.AbortController = globalThis.AbortController;
window.eval(fs.readFileSync(dir + "/app.js", "utf8"));   // real HTTP path, no mock on purpose
window.document.dispatchEvent(new window.Event("DOMContentLoaded"));
const wait = (ms) => new Promise((r) => setTimeout(r, ms));
const $ = (id) => window.document.getElementById(id);
const ok = (label, cond, extra = "") => console.log((cond ? "PASS " : "FAIL ") + label + (extra ? " — " + extra : ""));

await wait(1500);
ok("bridge resolved to real HTTP backend", $("backend-pill").textContent === "http", $("backend-pill").textContent);
ok("demo banner hidden", $("demo-banner").hidden === true);
ok("5 sites from Go registry", window.document.querySelectorAll(".site-card").length === 5);
ok("stats from Go", $("stat-sites").textContent === "5", $("stat-sites").textContent);
ok("version shown", /0\.[0-9]+\.[0-9]+/.test($("worker-summary").textContent), $("worker-summary").textContent);

// save a setting through the real API and re-read it
$("sf-reply-min").value = "2";
$("sf-reply-min").dispatchEvent(new window.Event("input"));
$("sf-save").click();
await wait(700);
ok("save via /api/SaveSettings", $("sf-status").textContent === "Saved.", $("sf-status").textContent);
const persisted = await (await fetch(BASE + "/api/GetSnapshot")).json();
ok("persisted in Go settings", persisted.settings.show.first.reply_min === 2, JSON.stringify(persisted.settings.show.first.reply_min));

// real HTTP probe of a site (sandbox has no route to the site -> verdict must still render)
$("site-test").click();
await wait(2500);
ok("TestSite returned a verdict", /PASS|FAIL/.test($("site-test-status").textContent), $("site-test-status").textContent);

// driver viewer reads the real files from sites/<id>/
$("site-driver-load").click();
await wait(700);
const driverText = $("site-driver-code").textContent;
ok("driver viewer reads real files", !$("site-driver-code").hidden && driverText.includes("composer_input"), driverText.slice(0, 60).replace(/\n/g, " "));

// ---------- step 3: diagnostics from the Go core ----------
await wait(400);
const diagItems = [...window.document.querySelectorAll("#site-diag-checks .diag-item")];
ok("diagnostics from Go", diagItems.length >= 7, diagItems.length + " checks");
ok("diagnostics coverage from Go", /selectors \d+\/11/.test($("site-diag-coverage").textContent), $("site-diag-coverage").textContent);
const diagJson = await (await fetch(BASE + "/api/GetSiteDiagnostics?id=omezy")).json();
ok("GetSiteDiagnostics endpoint", diagJson.site_id === "omezy" && Array.isArray(diagJson.checks), JSON.stringify(diagJson.checks || []).slice(0, 60));

// TestSite now reports final url + readiness heuristics
$("site-test").click();
await wait(1500);
const activeSite = window.document.querySelector(".site-card.active").dataset.site;
const probeDiag = await (await fetch(BASE + "/api/GetSiteDiagnostics?id=" + activeSite)).json();
ok("probe writes latency history", (probeDiag.latency_history || []).length >= 1, activeSite + " · " + JSON.stringify(probeDiag.latency_history || []));

// ---------- step 3: sessions through the real API ----------
window.document.querySelector('.nav-button[data-tab="worker"]').click();
window.document.querySelector('[data-subpanel="worker-accounts"]').click();
const created = await (await fetch(BASE + "/api/SaveSession", {
  method: "POST", headers: { "Content-Type": "application/json" },
  body: JSON.stringify({ site_id: "omezy", profile: "kitcheck", values: { cookies: ["a", "b"], note: "console check" } }),
})).json();
ok("SaveSession writes session.json", created.profile === "kitcheck" && created.cookies === 2, JSON.stringify(created).slice(0, 80));
$("sess-site").value = "omezy";
$("sess-site").dispatchEvent(new window.Event("change"));
await wait(600);
ok("sessions listed from disk", $("sess-table-body").textContent.includes("kitcheck"), $("sess-table-body").textContent.slice(0, 60));
const delButton = [...$("sess-table-body").querySelectorAll("button")].find((b) => b.closest("tr").dataset.profile === "kitcheck");
delButton.click();
await wait(800);
const afterDelete = await (await fetch(BASE + "/api/GetSessions?id=omezy")).json();
ok("DeleteSession removes the profile", afterDelete.count === 0, JSON.stringify(afterDelete.sessions));

// auto-register writes real account files
window.document.querySelector('.nav-button[data-tab="worker"]').click();
window.document.querySelector('[data-subpanel="worker-accounts"]').click();
$("acc-count").value = "2";
$("acc-count").dispatchEvent(new window.Event("input"));
$("acc-register").click();
await wait(1200);
ok("accounts registered via API", $("acc-table-body").querySelectorAll("tr").length === 2, $("acc-status").textContent);
const afterAccounts = await (await fetch(BASE + "/api/GetSnapshot")).json();
ok("stat-accounts updated", afterAccounts.stats.accounts >= 2, String(afterAccounts.stats.accounts));
const accSite = $("acc-site").value;
const accState = afterAccounts.sites.find((s) => s.id === accSite);
ok("site state shows accounts for " + accSite, accState.accounts >= 2, String(accState.accounts));

// run + logs through the real job engine
window.document.querySelector('[data-subpanel="worker-sites"]').click();
$("worker-start").click();
await wait(1600);
ok("worker running (Go)", $("running-pill").textContent === "Running");
await wait(6000);
const rows = $("worker-log-list").children.length;
ok("live logs from Go ring", rows > 0, rows + " rows");
ok("log rows carry site ids", /omezy|camgo|chat42|chatusa|viby/.test($("worker-log-list").textContent));
$("worker-stop").click();
await wait(900);
ok("stopped (Go)", $("running-pill").textContent === "Stopped");

// proxy dial test (127.0.0.1:8090 is open, the other is not)
window.document.querySelector('[data-subpanel="worker-settings"]').click();
$("wk-proxy-entries").value = "127.0.0.1:8090\n10.255.255.1:1080";
$("wk-proxy-entries").dispatchEvent(new window.Event("input"));
$("wk-proxy-test").click();
await wait(4500);
ok("proxy verdicts from Go dial", $("wk-proxy-results").children.length === 2, $("wk-proxy-status").textContent);

ok("no runtime errors", errors.length === 0, errors.join(" | "));
window.close();
process.exit(0);
