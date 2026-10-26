# 01 · AGENT BRIEF — obey this, it is the whole job

You are rebuilding a dashboard for a **new** site using a design kit extracted from an
existing product. The design is fixed. The domain is yours to define.

## Hard rules

1. **Read before writing.** Required order: `04-UI-DESIGN-SYSTEM.md`,
   `05-DASHBOARD-SPEC.md`, `06-FRONTEND-PATTERNS.md`, `08-API-CONTRACT.md`, then
   `starter/index.html` + `starter/app.js`.
2. **Copy `starter/styles.css` verbatim.** Add new classes only at the END, under a
   `/* ---- <project> additions ---- */` banner, using existing tokens. Never edit an
   existing rule, never introduce a new colour literal, never add a second stylesheet
   unless it is a theme (`data-theme`).
3. **Keep the skeleton.** 2 top-level modes, 5 sub-sections each, one card per job,
   numbered step heads, panel header with badge + actions. If you think you need a third
   mode, you actually need a sixth sub-section.
4. **One bridge.** All backend calls go through `bridge.invoke(method, ...args)`.
   No `fetch` anywhere else, no direct `window.go` access outside `resolveBridge()`.
5. **One source of truth per setting.** A field is read/written in exactly one place:
   a `read<Section>()` function and `applySnapshot()`. Never duplicate a field into two
   sections "for convenience"; share the section instead.
6. **Every action reports back.** Status line, log row, or toast — pick one, never nothing.
7. **Secrets:** `type="password"`, never logged, never echoed into a log row or a toast.
8. **No new dependencies** without writing down why in the project README. The kit is
   deliberately dependency-free on the frontend.
9. **Domain logic stays out of the UI.** The frontend renders state and sends intent
   (`StartRun`, `SaveSettings`). No business rules in app.js beyond mapping form → payload.
10. **Prompt/instructions content is not design.** Persona text, message scripts and
    targeting lists from the original product must not be carried over. If your new site
    needs copy, write new copy.

## The loop you must follow

```
1  scaffold        copy starter/ → your project, rename brand, run it on the mock backend
2  define the 30%  write down: mode names, section names, settings structs, Run/Test verbs,
                   log kinds, data folder layout (one page, before coding)
3  contract        add the Go/HTTP methods you need, matching 08-API-CONTRACT.md shapes
4  wire UI         read<Section>() + applySnapshot() + actions; zero new CSS
5  verify          mock path, HTTP path, Wails path all behave identically
6  checklist       go through 12-CHECKLIST-70-30.md line by line
```

## Definition of done

- [ ] `python3 -m http.server` on the starter → every button does something visible
- [ ] Real backend: snapshot loads, every Save persists across restart, Run starts and
      Stop ends it, logs stream, filters work, Clear works
- [ ] Unsaved draft survives: switch sub-panels, wait 10 s of polling, come back — text intact
- [ ] Errors surface as a red status line, never a console-only message
- [ ] All secrets masked; `grep -ri "sk-" <repo>` returns only placeholder text
- [ ] No hard-coded `chitchat` / original-brand strings remain (`grep -ri chitchat`)
- [ ] Works at 1280×820 (Wails default) and at 900 px width (subnav wraps, logs stay usable)
- [ ] Both alternates (`data-theme="ocean"`, `data-theme="light"`) still look correct
- [ ] `node --check` passes for every JS file; Go builds with `go vet ./...` clean

## Anti-patterns that will break review

| Don't | Do |
|---|---|
| New React/Vue app "because it's cleaner" | Keep the vanilla 3-file frontend; it is the design |
| A card inside a card | `card-grid` with sibling cards |
| Inline `style="color:#fff"` | Use a token/class |
| `alert()` / `window.prompt` | Status line or toast |
| Writing the whole log array into DOM each second | Append + filter, pin scroll |
| Two places writing `settings.actions` | One section, one reader, one writer |
| Login flow faked in the UI | Backend launches the browser; UI only reports |
| Silent `catch {}` | Report to status line + `reportFrontendError()` |

## Output expectations when you hand work back

1. A short `NOTES.md`: your 30% decisions (mode names, sections, methods added).
2. A screenshot or the starter still running in mock mode.
3. The completed checklist from `12-CHECKLIST-70-30.md` with anything skipped called out
   explicitly as `NOT RUN` — never silently skipped.
