# TEST REPORT — design kit v1.0.0 (2026-10-26)

Everything below was executed on the **freshly extracted zip**, not on a working copy.

## 1 · Static checks

| Check | Command | Result |
|---|---|---|
| JS syntax (starter shell) | `node --check starter/app.js` | PASS |
| JS syntax (mock backend) | `node --check starter/js/mock-backend.js` | PASS |
| Token file validity | `python3 -c "json.load(open('design-tokens.json'))"` | PASS |
| Manifest validity | `python3 -c "json.load(open('manifest.json'))"` | PASS — 43 files, 414 677 bytes |

## 2 · Contract parity (`node --test extras/contract.test.cjs`)

```
# tests 7   # pass 7   # fail 0
```
1. every method called by the UI exists in the mock — PASS
2. every method called by the UI exists on the Go `App` struct — PASS
3. Go `App` exposes ≥ 8 bound methods — PASS
4. every DOM id the JS uses exists in `index.html` — PASS
5. no inline colour literals in HTML/JS — PASS
6. no original-product strings (chitchat / evachat / persona) — PASS
7. no secret-shaped strings in the tracked files — PASS

## 3 · Live UI run (jsdom, 27 assertions)

Executed in a virtual browser against the extracted `starter/` on the mock backend:

| Area | Assertions | Result |
|---|---|---|
| Boot + bridge detection (`demo` pill) | 1 | PASS |
| Snapshot applied to fields | 1 | PASS |
| Stats rendered | 1 | PASS |
| Mode A subnav (5 sections) | 5 | PASS |
| Mode B tab + subnav (5 sections) | 6 | PASS |
| Save → status line "Saved." | 1 | PASS |
| Draft survives 3 s polling | 1 | PASS |
| Connection tester (3 entries → 3 verdicts, "2/3 passed.") | 1 | PASS |
| Strategy switch reveals the right block | 1 | PASS |
| Run started (sidebar pill → Running) | 1 | PASS |
| Log feed streaming (5 rows after ~6 s) | 1 | PASS |
| Log filter narrows the feed (3 IO rows) | 1 | PASS |
| Clear empties feed + empty hint returns | 1 | PASS |
| Theme switch ocean / light / forest | 3 | PASS |
| No uncaught runtime errors | 1 | PASS |

**27 / 27 PASS** (one earlier instrumented check used a text-matching regex that could not
match concatenated DOM text; it was re-verified with element queries — see §4).

## 4 · Log feed vocabulary (element-level verification)

```
badges      : SYS, OUT, IN, SKIP, ERR
row classes : log-system | log-out | log-in | log-skip | log-error
rows        : 10 after ~12 s of run
```
All five kinds render with the correct CSS class, badge label and filter grouping. PASS.

## 5 · What was NOT run here (honest list)

- `go vet ./...` / `go test ./...` — **NOT RUN**: no Go toolchain in the build sandbox.
  The Go files in `extras/` and `reference/` are reference material, not part of a module.
- `wails build` (desktop exe) — **NOT RUN** for the same reason; run `install.bat` on
  Windows and follow `11-BUILD-RUNBOOK.md`.
- Real browser rendering (Chrome/WebView2) — **NOT RUN**: the DOM/logic was exercised in
  jsdom. Visual confirmation is one `python3 -m http.server 8080` away.
- Real network calls (proxy test, model API) — **NOT RUN** by design: the mock answers.

## 6 · How to reproduce

```bash
unzip chitchat-design-kit.zip && cd chitchat-design-kit
node --check starter/app.js
node --test extras/contract.test.cjs
cd starter && python3 -m http.server 8080      # click through both modes
```
