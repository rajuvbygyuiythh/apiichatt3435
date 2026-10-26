# 13 · AGENT PROMPTS (copy-paste)

Three prompts cover the whole job: **build**, **adapt**, **review**. Paste the block as-is,
then fill the `<>` slots. They assume the agent has this folder in its workspace.

---

## Prompt 1 · Build the 70% (new project from the kit)

> You are implementing a dashboard from a design kit located in `./design-kit/`.
>
> 1. Read, in this order: `design-kit/01-AGENT-BRIEF.md`,
>    `design-kit/04-UI-DESIGN-SYSTEM.md`, `design-kit/05-DASHBOARD-SPEC.md`,
>    `design-kit/06-FRONTEND-PATTERNS.md`, `design-kit/08-API-CONTRACT.md`.
> 2. Copy `design-kit/starter/` into `cmd/<app>/ui/` of the new project. Do **not** rewrite
>    `styles.css`. Do not introduce a frontend framework or a bundler.
> 3. Get it running on the mock backend first (`python3 -m http.server`), and confirm in the
>    browser: tab switching, subnav, every Save, the Test button, Run, log feed, filters,
>    Clear, theme switch. Fix nothing by changing CSS.
> 4. Then implement the Go side per `07-BACKEND-PATTERNS.md`: `main.go` with `go:embed` +
>    `wails.Run`, an `App` struct exposing exactly the methods in `08-API-CONTRACT.md`,
>    Badger storage with `settings/*` + `logs/*`, and `emit("ui:running"|"ui:log", …)`.
> 5. Verify all three transports behave identically, then walk
>    `design-kit/12-CHECKLIST-70-30.md` section A.
>
> Constraints: no new npm dependencies; no colour literals outside `:root`; every action
> must produce a status line, log row, or toast; every backend error must surface in the UI
> verbatim. Report `NOT RUN` for anything you did not execute.

---

## Prompt 2 · Adapt it to my site (the 30%)

> The dashboard in `./cmd/<app>/ui/` is a working copy of a design kit. Adapt it to
> **<site/product>**, whose job is **<one sentence: what the user configures and what
> happens when they press Run>**.
>
> Before coding, write `NOTES.md` containing the filled tables from
> `design-kit/10-REBRAND-FOR-NEW-SITE.md` step 0: product name, initials, subtitle, the two
> mode names, the five section names per mode, the settings structs, what "Test" tests, what
> "Run" runs, the log vocabulary, the data folder layout, and the list table columns.
> Wait for my approval of `NOTES.md`.
>
> Then implement in this order, one commit each:
> 1. Brand + title + exe name + `--bg`-matched window colour.
> 2. Section names (sidebar button, card `h2`, `data-subpanel`) — no other change.
> 3. Go structs + `settings/<section>` store pairs + `SaveSettings` sections.
> 4. `read<Section>()` + `applySnapshot()` lines + HTML fields (1:1:1, no duplicates).
> 5. `TestConnection` / `TestAPI` / `StartRun` / `StopRun` for the real job.
> 6. Log vocabulary + labels; data folder seeding + Open Folder button.
> 7. Delete the mock + demo banner, or gate them behind `?demo`.
>
> Do not change: the 2×5 layout, component markup, stylesheet, `formStore`/`dirtyFields`,
> the single bridge. If a requirement seems to need a new pattern, stop and explain why.
> Finish by walking `12-CHECKLIST-70-30.md` sections A–C and listing every `NOT RUN` item.

---

## Prompt 3 · Review a derived dashboard

> Review `./cmd/<app>/ui/` (HTML/CSS/JS + Go handlers) against the design kit in
> `./design-kit/`. Be strict; this is a fidelity review, not a taste review.
>
> Check and report as a table of `PASS / FAIL / NOT RUN`, with file:line evidence:
> 1. **Structure** — 2 modes × 5 sections, numbered step heads, no nested cards, subpage widths.
> 2. **Design system** — tokens only (no hex literals outside `:root`), radii 8/10/14/999,
>    one accent, no gradients/shadows on cards, type sizes from the scale.
> 3. **Components** — labelled controls, min/max pairs, radio-card strategies, entry→test→
>    results block, password fields, run card, log card, status lines, toasts.
> 4. **Behaviour** — single `invoke()` bridge, transport pill, draft protection, save
>    re-applies snapshot, Run saves first, `finally` releases busy, log cursor + `pending`
>    guard + scroll pinning, `window.onerror` reporting.
> 5. **Contract** — every method exists in Go and in the mock; error shapes match; no
>    whole-history log polls; mutating methods return the snapshot.
> 6. **Hygiene** — no original-domain strings or logic, no secrets in repo, gitignore,
>    `node --check`, `go vet`.
>
> For each FAIL: give the minimal patch (file + replacement snippet) that keeps the design
> intact. Do not propose architectural rewrites. Do not mark anything PASS that you did not
> execute (`NOT RUN` is the correct label for unrun desktop checks).

---

## Prompt 4 · Emergency: "the UI is broken after my change"

> The dashboard at `./cmd/<app>/ui/` was working and now breaks as follows: **<symptom,
> console message>**. Diff against the design kit's `starter/app.js` (the reference
> implementation) and find the smallest behavioural difference. Likely suspects, in order:
> bridge resolution, an id renamed in HTML but not in JS (or vice versa), `dirtyFields`
> never cleared after a save, `setBusy` not released in `finally`, log cursor not advanced,
> a `<script>` order change, a stylesheet rule edited instead of extended. Report the root
> cause, the one-file patch, and how you verified it (mock path, and HTTP/Wails if run).

---

## Prompt 5 · Design review only (no code)

> Read `design-kit/04-UI-DESIGN-SYSTEM.md` and `05-DASHBOARD-SPEC.md`. For my new site
> **<site>**, propose the five sections of each of the two modes as a table:
> `# | section name | fields | buttons | feedback | backend method`. Keep the field count
> close to the kit's minimums (`05` §4). Call out anything that would force a new visual
> component, and propose how to express it with existing classes instead.

---

## House rules for any agent working in this repo

1. Never edit `design-kit/reference/**` — it is the source of truth for the design.
2. Never edit `styles.css` rules above the "STARTER ADDITIONS" banner.
3. Never ship `reference/` or the mock's fake data in a production build.
4. One commit per step of Prompt 2; each commit must leave the UI working on the mock.
5. When unsure, prefer the smaller change and write the question in `NOTES.md`.
