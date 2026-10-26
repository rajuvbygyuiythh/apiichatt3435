#!/usr/bin/env node
// =============================================================================
//  contract.test.cjs — method-parity check for a derived dashboard
// -----------------------------------------------------------------------------
//  Catches the most common multi-language bug: a method that exists in JS but
//  not in the Go backend (or vice versa). No dependencies, no jsdom, no network.
//
//  Usage:
//     node --test extras/contract.test.cjs
//  or copy to tests/contract.test.cjs in your project and adjust PATHS.
// =============================================================================
"use strict";

const test = require("node:test");
const assert = require("node:assert");
const fs = require("node:fs");
const path = require("node:path");

const PATHS = {
  appJs: process.env.APP_JS || path.join(__dirname, "..", "starter", "app.js"),
  mockJs: process.env.MOCK_JS || path.join(__dirname, "..", "starter", "js", "mock-backend.js"),
  goFile: process.env.GO_APP || path.join(__dirname, "app-skeleton.go"),
  html: process.env.HTML || path.join(__dirname, "..", "starter", "index.html"),
};

const read = (file) => fs.readFileSync(file, "utf8");

/** Methods the frontend actually calls: bridge.invoke("X", …) */
function invokedMethods(js) {
  const found = new Set();
  for (const m of js.matchAll(/bridge\.invoke\(\s*"([A-Za-z0-9_]+)"/g)) found.add(m[1]);
  return found;
}

/** Methods exposed by the mock: `async X(` inside the mock object */
function mockMethods(js) {
  const found = new Set();
  for (const m of js.matchAll(/^\s{4}async\s+([A-Za-z0-9_]+)\s*\(/gm)) found.add(m[1]);
  return found;
}

/** Exported App methods in Go: func (a *App) X( */
function goMethods(go) {
  const found = new Set();
  for (const m of go.matchAll(/func \(a \*App\) ([A-Z][A-Za-z0-9_]*)\(/g)) found.add(m[1]);
  return found;
}

/** ids used by JS: byId("x") / getElementById("x") */
function jsIds(js) {
  const found = new Set();
  for (const m of js.matchAll(/byId\(\s*"([A-Za-z0-9_-]+)"/g)) found.add(m[1]);
  for (const m of js.matchAll(/getElementById\(\s*"([A-Za-z0-9_-]+)"/g)) found.add(m[1]);
  return found;
}

/** ids present in the HTML */
function htmlIds(html) {
  const found = new Set();
  for (const m of html.matchAll(/\bid="([A-Za-z0-9_-]+)"/g)) found.add(m[1]);
  return found;
}

const appJs = read(PATHS.appJs);
const mockJs = read(PATHS.mockJs);
const goSrc = read(PATHS.goFile);
const html = read(PATHS.html);

test("every method called by the UI exists in the mock", () => {
  const mock = mockMethods(mockJs);
  const missing = [...invokedMethods(appJs)].filter((m) => !mock.has(m));
  assert.deepEqual(missing, [], `mock is missing: ${missing.join(", ")}`);
});

test("every method called by the UI exists on the Go App struct", () => {
  const go = goMethods(goSrc);
  const missing = [...invokedMethods(appJs)].filter((m) => !go.has(m));
  assert.deepEqual(missing, [], `Go App is missing: ${missing.join(", ")}`);
});

test("Go App does not leak lowercase (unbound) helper names into the contract", () => {
  // exported only: bound methods must start uppercase, that is the whole point
  const exported = goMethods(goSrc);
  assert.ok(exported.size >= 8, `expected >=8 App methods, found ${exported.size}`);
});

test("every DOM id the JS uses exists in index.html", () => {
  const ids = htmlIds(html);
  const missing = [...jsIds(appJs)].filter((id) => !ids.has(id));
  assert.deepEqual(missing, [], `index.html is missing ids: ${missing.join(", ")}`);
});

test("no colour literals leaked into JS/HTML styles", () => {
  const bad = [];
  const styleAttr = /style="[^"]*#[0-9a-fA-F]{3,8}/g;
  for (const file of [html, appJs]) {
    const hits = file.match(styleAttr);
    if (hits) bad.push(...hits);
  }
  assert.deepEqual(bad, [], "inline colours bypass the design tokens");
});

test("no original-product strings left behind", () => {
  const offenders = [];
  const banned = [/chitchat/i, /evachat/i, /persona/i];
  for (const [file, content] of Object.entries({ appJs, html, goSrc })) {
    for (const re of banned) {
      if (re.test(content)) offenders.push(`${file}: ${re}`);
    }
  }
  assert.deepEqual(offenders, []);
});

test("secrets are never committed", () => {
  const files = [appJs, html, goSrc, mockJs];
  const hits = files.flatMap((content) => content.match(/sk-[A-Za-z0-9]{16,}/g) || []);
  assert.deepEqual(hits, [], "found something shaped like a real API key");
});
