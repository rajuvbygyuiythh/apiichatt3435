# SITE-ANALYSIS.md — what the source release actually contains

Source analysed: `github.com/rajuvbygyuiythh/fghfghgfhgf`, release
`v2.0.0-multisite-final` → "Multi-Site Engine (102 Sites Unified incl. ChitChat.gg — v3)",
inner archive `ChitChat-App-clean-fixed-2026-10-20 (11).zip` (24 MB).

## 1 · Layout of the release

```
install.bat / apichat.bat / debug.bat
build/          chitchatui.exe · apichat.exe · chitchat-Check.exe
memory/         chat_history.jsonl · operation_memory.json · sites_status.md · deep_debug_report
skills/         auto_onboard_agent.js · dom_auto_scout.js · protection_detector.js · ws_protocol_sniffer.js
sites/          102 site folders + template_new_site
source_code/    the Go module (same engine as the 2026-10-20 build, plus internal/sites/)
```

## 2 · The site-folder contract (the important part)

Every one of the 102 sites is an **isolated folder** with identical shape:

| File/folder | Purpose |
|---|---|
| `site.json` | id, name, `family`, `account_mode`, `enabled`, `start_url`, `app_url`, `auth_mode`, `entry_flow`, `protection_summary`, and 11 CSS selectors (`composer_input`, `send_button`, `skip_button`, `start_button`, `message_history`, `message_row`, `message_text`, `partner_message_selector`, `own_message_selector`, `partner_name`, `chat_ended_text`) |
| `dom_driver.js` | isolated DOM observer + auto-onboard agent, parameterised by the same config; fills names/age, ticks 18+, picks gender, clicks Start |
| `<id>_protection.txt` | onboarding flow, anti-bot notes, safe pacing rules |
| `accounts/` | auto-registered / browser-captured identities (JSON per account) |
| `profiles/`, `sessions/` | saved browser session per profile |
| `README.txt` | folder contract reminder |
| `template_new_site/` | blank template for adding site #103 |

**Consequence for our build:** the site is *data*, not code. Adding or removing a site is a
folder operation; the engine and the dashboard never change.

## 3 · Engine families (as counted across the 102 folders)

| Family | Count | Shape |
|---|---|---|
| `family_a` | 30 | Omegle-style text roulette (18+ modal → Start/Esc/Send) |
| `family_b` | 20 | Hybrid text/video roulette (StrangerLine/Thundr/Camgo pattern) |
| `family_c` | 25 | Guest nickname/gender form → 1v1 DM (Chatib/Chatiw/Viby pattern) |
| `family_d` | 15 | Guest chat network / lobby → rooms & DM (Chatzy/Rolechat/321Chat) |
| `family_e` | 10 | One-click stranger match (ChatForFree/ChatUSA/Knot style) |
| `custom` / unset | 2 | template + one special folder |

`source_code/internal/sites/` implements this as data-driven Go:

| File | Lines | Role |
|---|---|---|
| `catalog.go` | ~1.3 k | `ArchetypeSelectors(family)` — the universal selector bundle per family |
| `manager.go` | 833 | load/scan sites, per-site session + driver wiring |
| `autoreg.go` | ~330 | `AutoRegisterSiteAccounts` (no browser), guest alias generation |
| `diagnostics.go` | 407 | site-level health checks |
| `session_store.go` | 256 | per-site session persistence |
| `sites_test.go` | 284 | registry tests |

Other engine packages: `internal/bot` (sequences, first message, keyword/realtime),
`internal/apichat` (LLM path), `internal/browser` (Rod + chromedp backends),
`internal/proxy`, `internal/uistore`, `internal/workerplan`, `internal/memory`.

## 4 · The 5 sites chosen for this build (ChitChat excluded)

One per family, all `auto_register` (no manual browser capture needed to start):

| # | Site | Family | Start URL | Why this one |
|---|---|---|---|---|
| 1 | `omezy` | a | `https://omezy.io/text/` | Listed first in the registry, classic 18+-modal roulette — the family_a reference |
| 2 | `camgo` | b | `https://camgo.com/` | Text panel appears only after Start: exercises the "wait for composer" path |
| 3 | `viby` | c | `https://viby.chat/free-chat-rooms` | Guest nickname → private 1v1; 10 chats/day protection note is a good pacing example |
| 4 | `chat42` | d | `https://chat42.com/` | Name form + lobby; simplest family_d representation |
| 5 | `chatusa` | e | `https://www.chatusa.club/stranger-chat` | One-click match; fastest cycle — the pacing stress case |

Their folders were copied **verbatim** (`site.json`, `dom_driver.js`, protection notes) into
`sites/`. Runtime folders (`accounts/`, `profiles/`, `logs/`, `sessions/`) are gitignored and
created on demand.

### Rejected candidates (kept for step 6+ if you want more sites)

`funchatx`, `chatrio`, `opentalk`, `joingy`, `strangertalk` (all family_a, redundant with
omezy) · `thundr_cc`, `emeraldchat`, `chatrandom_text`, `badoocam` (family_b, some need
`browser_setup`) · `chatiw_com`, `y99_in` (family_c) · `chatblink`, `chatzy`, `imeetzu`
(family_d) · `chatusa` picked over `thxchat`, `knotchat` (simplest entry flow).

## 5 · What we deliberately did NOT copy

- ChitChat itself, its persona/message content and share-turn behaviour
- `skills/*.js` reconnaissance scripts and `memory/*` operational data
- The 97 unused site folders (the registry supports adding them back by folder copy)
- Any credentials or account files

## 6 · Adding a 6th site later

```bash
cp -r sites/template_new_site sites/newsite      # or copy an existing site and edit
# edit sites/newsite/site.json: id, name, family, start_url, selectors
go run ./cmd/console                             # it appears in the registry automatically
```
No Go change, no UI change: the picker, the checklist, the state table and the stats all
read from the registry.
