package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// App is the application layer. In the desktop build these exported methods are
// bound to Wails (window.go.main.App.Method); in web mode they are exposed as
// /api/<Method> by http.go and by cmd/console. Same methods, same JSON.
type App struct {
	dataDir  string
	sitesDir string
	kind     string // "http" | "wails"

	store *Store
	log   *LogRing
	sites []SiteConfig

	mu        sync.Mutex
	lastTests map[string]TestResult
	running   bool
	runMode   string
	cancel    context.CancelFunc
	stats     Stats
	startedAt time.Time

	httpClient *http.Client
}

// New wires the app: load the registry, open the store, seed the log.
func New(dataDir, sitesDir, kind string) (*App, error) {
	sites, problems := LoadSites(sitesDir)
	if len(sites) == 0 {
		return nil, fmt.Errorf("no sites found in %s (expected sites/<id>/site.json)", sitesDir)
	}

	store, err := OpenStore(dataDir, SiteIDs(sites))
	if err != nil {
		return nil, err
	}

	app := &App{
		dataDir:    dataDir,
		sitesDir:   sitesDir,
		kind:       kind,
		store:      store,
		log:        NewLogRing(2000),
		sites:      sites,
		startedAt:  time.Now(),
		httpClient: &http.Client{Timeout: 8 * time.Second},
	}

	app.log.Append("system", fmt.Sprintf("console started · %d sites · backend %s", len(sites), kind), Version, "")
	for _, problem := range problems {
		app.log.Append("error", "site skipped: "+problem.Error(), "registry", "")
	}
	app.log.Append("system", "settings at "+store.Path(), "storage", "")
	return app, nil
}

// Sites exposes the registry (read-only use).
func (a *App) Sites() []SiteConfig { return a.sites }

// DataDir is where settings/logs/accounts live.
func (a *App) DataDir() string { return a.dataDir }

// SitesDir is where per-site folders live.
func (a *App) SitesDir() string { return a.sitesDir }

// Log exposes the ring to the desktop event bridge.
func (a *App) Log() *LogRing { return a.log }

// SiteStates joins registry config with on-disk facts.
//
// status is deliberately simple and computed only from disk, so the dashboard
// can explain itself without trusting any in-memory flag:
//
//	disabled    -> site.json has enabled:false
//	no-driver   -> dom_driver.js missing/empty
//	needs-setup -> no accounts/ and no saved profile yet
//	ready       -> has a driver and at least one account or session
func (a *App) SiteStates() []SiteState {
	states := make([]SiteState, 0, len(a.sites))
	for _, site := range a.sites {
		base := filepath.Join(a.sitesDir, site.ID)
		state := SiteState{
			SiteConfig:  site,
			Accounts:    CountFiles(filepath.Join(base, "accounts")),
			HasSession:  DirHasFiles(filepath.Join(base, "profiles")) || DirHasFiles(filepath.Join(base, "sessions")),
			DriverBytes: len(readFileOrEmpty(filepath.Join(base, "dom_driver.js"))),
			LastTest:    "never",
		}
		switch {
		case !site.Enabled:
			state.Status = "disabled"
		case state.DriverBytes == 0:
			state.Status = "no-driver"
		case state.Accounts == 0 && !state.HasSession:
			state.Status = "needs-setup"
		default:
			state.Status = "ready"
		}
		if a.lastTests != nil {
			if last, ok := a.lastTests[site.ID]; ok && last.Entry != "" {
				state.HTTPStatus = last.HTTP
				state.LatencyMS = last.LatencyMS
				state.LastTest = fmt.Sprintf("%s · %s · %d ms", time.Now().Format("15:04:05"), fallback(last.Message, "probe"), last.LatencyMS)
			}
		}
		states = append(states, state)
	}
	return states
}

// Stats recomputes the capsule numbers.
func (a *App) Stats() Stats {
	states := a.SiteStates()
	stats := Stats{Sites: len(states)}
	for _, state := range states {
		if state.SiteConfig.Enabled {
			stats.Enabled++
		}
		stats.Accounts += state.Accounts
		if state.HasSession {
			stats.Sessions++
		}
	}
	a.mu.Lock()
	stats.Running = 0
	if a.running {
		stats.Running = 1
	}
	stats.Completed = a.stats.Completed
	stats.Failed = a.stats.Failed
	a.mu.Unlock()
	return stats
}

// Snapshot is the single object the dashboard loads with.
func (a *App) Snapshot() Snapshot {
	a.mu.Lock()
	running, mode := a.running, a.runMode
	a.mu.Unlock()

	settings := a.store.Get()
	return Snapshot{
		Settings: settings,
		Strategy: settings.Message.Strategy,
		Sites:    a.SiteStates(),
		Stats:    a.Stats(),
		Runtime: Runtime{
			Running:   running,
			Mode:      mode,
			Kind:      a.kind,
			DataDir:   a.dataDir,
			Version:   Version,
			StartedAt: a.startedAt.Format("15:04:05"),
			Sites:     len(a.sites),
		},
	}
}

// SaveSettings merges one section and returns the fresh snapshot.
func (a *App) SaveSettings(section string, values json.RawMessage) (Snapshot, error) {
	if _, err := a.store.Apply(section, values); err != nil {
		return a.Snapshot(), err
	}
	a.log.Append("system", "settings saved: "+section, "storage", "")
	return a.Snapshot(), nil
}

// Logs returns everything after the cursor plus current stats.
func (a *App) Logs(after int) LogPage {
	version, entries, cleared := a.log.Since(after)
	a.mu.Lock()
	running := a.running
	a.mu.Unlock()
	return LogPage{Version: version, Entries: entries, Cleared: cleared, Running: running, Stats: a.Stats()}
}

// ClearLogs empties the feed.
func (a *App) ClearLogs() map[string]any {
	a.log.Clear()
	return map[string]any{"ok": true}
}

// TestSite performs a real HTTP probe of the site's start URL and reports
// what the dashboard needs: reachability, latency, and the protection note.
func (a *App) TestSite(siteID string) (TestResult, error) {
	site, ok := FindSite(a.sites, siteID)
	if !ok {
		return TestResult{}, fmt.Errorf("unknown site %q", siteID)
	}
	target := site.StartURL
	if target == "" {
		return TestResult{}, fmt.Errorf("site %q has no start_url", siteID)
	}
	return a.probe(target, siteID)
}

func (a *App) probe(target, siteID string) (TestResult, error) {
	start := time.Now()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, target, nil)
	if err != nil {
		result := TestResult{Entry: target, OK: false, Message: err.Error()}
		a.rememberTest(siteID, result)
		return result, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) MultiSiteConsole/"+Version)
	resp, err := a.httpClient.Do(req)
	latency := int(time.Since(start).Milliseconds())
	if err != nil {
		result := TestResult{Entry: target, OK: false, LatencyMS: latency, Message: shortError(err)}
		a.log.Append("error", fmt.Sprintf("%s unreachable (%d ms)", target, latency), shortError(err), siteID)
		a.rememberTest(siteID, result)
		return result, nil
	}
	defer resp.Body.Close()

	result := TestResult{
		Entry:     target,
		OK:        resp.StatusCode < 400,
		LatencyMS: latency,
		HTTP:      resp.StatusCode,
		Message:   fmt.Sprintf("HTTP %d", resp.StatusCode),
	}
	if result.OK {
		a.log.Append("system", fmt.Sprintf("%s reachable · HTTP %d · %d ms", hostOf(target), resp.StatusCode, latency), "site probe", siteID)
	} else {
		a.log.Append("error", fmt.Sprintf("%s returned HTTP %d", hostOf(target), resp.StatusCode), "site probe", siteID)
	}
	a.rememberTest(siteID, result)
	return result, nil
}

func (a *App) rememberTest(siteID string, result TestResult) {
	if result.Entry == "" {
		return
	}
	a.mu.Lock()
	if a.lastTests == nil {
		a.lastTests = map[string]TestResult{}
	}
	a.lastTests[siteID] = result
	a.mu.Unlock()
}

// TestConnections dials each proxy/host line and reports per-line verdicts.
func (a *App) TestConnections(payload ProxySettings) (map[string][]TestResult, error) {
	lines := splitLines(payload.Entries)
	results := make([]TestResult, 0, len(lines))
	for _, line := range lines {
		hostPort := normaliseHostPort(line, payload.Scheme)
		start := time.Now()
		conn, err := net.DialTimeout("tcp", hostPort, 3*time.Second)
		latency := int(time.Since(start).Milliseconds())
		if err != nil {
			results = append(results, TestResult{Entry: line, OK: false, LatencyMS: latency, Message: shortError(err)})
			continue
		}
		_ = conn.Close()
		results = append(results, TestResult{Entry: line, OK: true, LatencyMS: latency, Message: "tcp ok"})
	}
	passed := 0
	for _, result := range results {
		if result.OK {
			passed++
		}
	}
	a.log.Append("system", fmt.Sprintf("connections tested: %d/%d passed", passed, len(results)), "proxy", "")
	return map[string][]TestResult{"results": results}, nil
}

// StartRun starts the job for the given mode ("show" or "worker").
func (a *App) StartRun(mode string, siteIDs []string, profile string) (map[string]any, error) {
	if mode != "show" && mode != "worker" {
		return nil, fmt.Errorf("mode must be show or worker, got %q", mode)
	}
	if len(siteIDs) == 0 {
		return nil, errors.New("select at least one site")
	}
	for _, id := range siteIDs {
		if _, ok := FindSite(a.sites, id); !ok {
			return nil, fmt.Errorf("unknown site %q", id)
		}
	}

	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return nil, errors.New("already running")
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.running, a.runMode, a.cancel = true, mode, cancel
	a.stats.Completed, a.stats.Failed = 0, 0
	a.mu.Unlock()

	if profile == "" {
		profile = "default"
	}
	a.log.Append("system", fmt.Sprintf("%s run started · %d site(s) · profile %s", mode, len(siteIDs), profile), "runner", "")
	go a.runJob(ctx, mode, siteIDs, profile)
	return map[string]any{"ok": true, "message": "started"}, nil
}

// StopRun cancels the job. Safe to call when nothing is running.
func (a *App) StopRun() (map[string]any, error) {
	a.mu.Lock()
	cancel := a.cancel
	wasRunning := a.running
	a.cancel, a.running, a.runMode = nil, false, ""
	a.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if wasRunning {
		a.log.Append("system", "run stopped by user", "runner", "")
	}
	return map[string]any{"ok": true}, nil
}

// runJob is the Step-2 job engine: it walks the configured sequence per site and
// logs every step, honouring the configured pacing and the stop signal.
// Step 4 replaces the pacing/logging body with the real DOM driver per site.
func (a *App) runJob(ctx context.Context, mode string, siteIDs []string, profile string) {
	defer func() {
		a.mu.Lock()
		a.running, a.runMode = false, ""
		a.mu.Unlock()
		a.log.Append("system", "run finished", "runner", "")
	}()

	settings := a.store.Get()
	lines := splitLines(settings.Message.Sequence)
	if len(lines) == 0 {
		lines = []string{"hello there"}
	}

	for _, siteID := range siteIDs {
		site, _ := FindSite(a.sites, siteID)
		if !site.Enabled {
			a.log.Append("skip", fmt.Sprintf("%s is disabled in site.json", site.Name), "registry", siteID)
			continue
		}
		if !a.sleep(ctx, 700*time.Millisecond) {
			return
		}
		a.log.Append("system", fmt.Sprintf("opening %s (%s · %s)", site.Name, site.Family, site.AccountMode), site.ID+"/site.json", siteID)
		a.log.Append("system", fmt.Sprintf("onboarding: %s", fallback(site.EntryFlow, "guest entry")), "auto-onboard", siteID)

		if !a.sleep(ctx, a.pace(settings, mode)) {
			return
		}
		for i, line := range lines {
			if i >= settings.Message.MaxLines {
				break
			}
			a.log.Append("out", line, fmt.Sprintf("line %d/%d · %s", i+1, len(lines), settings.Message.Strategy), siteID)
			if !a.sleep(ctx, a.pace(settings, mode)) {
				return
			}
			if i == 0 {
				a.log.Append("in", "hey, how's it going?", "partner reply", siteID)
			}
		}
		a.mu.Lock()
		a.stats.Completed++
		a.mu.Unlock()
		a.log.Append("skip", "closing chat and rotating", "next target", siteID)
	}
}

// pace returns the delay between steps for the mode (seconds, min 1s for the demo).
func (a *App) pace(settings Settings, mode string) time.Duration {
	lo, hi := settings.Worker.ActionMin, settings.Worker.ActionMax
	if mode == "show" {
		lo, hi = settings.Show.First.ReplyMin, settings.Show.First.ReplyMax
	}
	if hi < lo {
		hi = lo
	}
	seconds := lo
	if hi > lo {
		seconds = lo + rand.Intn(hi-lo+1)
	}
	if seconds < 1 {
		seconds = 1
	}
	return time.Duration(seconds) * time.Second
}

func (a *App) sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// LaunchLogin opens a browser for a manual login (Step 4 wires the real driver).
func (a *App) LaunchLogin(siteID, profile string) (map[string]any, error) {
	site, ok := FindSite(a.sites, siteID)
	if !ok {
		return nil, fmt.Errorf("unknown site %q", siteID)
	}
	if profile == "" {
		profile = "default"
	}
	dir := filepath.Join(a.sitesDir, site.ID, "profiles", profile)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("prepare profile dir: %w", err)
	}
	a.log.Append("system", fmt.Sprintf("login window requested for %s (profile %s)", site.Name, profile), "browser", siteID)
	a.log.Append("skip", "browser driver is wired in step 4 — no window opened yet", "scaffold", siteID)
	return map[string]any{"ok": true, "message": "login queued", "profile_dir": dir}, nil
}

// AutoRegisterAccounts creates count identities inside sites/<id>/accounts/.
// No browser is involved: it writes the session files the driver will later use.
func (a *App) AutoRegisterAccounts(siteID string, count int) ([]Account, error) {
	site, ok := FindSite(a.sites, siteID)
	if !ok {
		return nil, fmt.Errorf("unknown site %q", siteID)
	}
	if count < 1 || count > 20 {
		return nil, errors.New("count must be between 1 and 20")
	}
	settings := a.store.Get()
	pool := splitCSV(settings.Accounts.NicknamePool)
	if len(pool) == 0 {
		pool = []string{"Sophia", "Emma", "Olivia", "Mia", "Chloe"}
	}

	dir := filepath.Join(a.sitesDir, site.ID, "accounts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create accounts dir: %w", err)
	}

	created := make([]Account, 0, count)
	for i := 0; i < count; i++ {
		nickname := pool[rand.Intn(len(pool))] + fmt.Sprintf("%02d", 10+rand.Intn(90))
		account := Account{
			ID:        fmt.Sprintf("%s-%d", site.ID, time.Now().UnixNano()+int64(i)),
			SiteID:    site.ID,
			SiteName:  site.Name,
			Nickname:  nickname,
			Gender:    settings.Accounts.Gender,
			Age:       settings.Accounts.Age,
			Status:    "registered",
			CreatedAt: time.Now(),
			FilePath:  filepath.Join(dir, fmt.Sprintf("%s_%d.json", nickname, time.Now().UnixNano()+int64(i))),
		}
		raw, err := json.MarshalIndent(account, "", "  ")
		if err != nil {
			return created, err
		}
		if err := os.WriteFile(account.FilePath, append(raw, '\n'), 0o644); err != nil {
			return created, fmt.Errorf("write account: %w", err)
		}
		created = append(created, account)
		a.log.Append("out", fmt.Sprintf("registered %s (%s, %d)", account.Nickname, account.Gender, account.Age), "auto-register", siteID)
	}
	return created, nil
}

// SiteDriver returns the per-site DOM driver script and config for the viewer.
func (a *App) SiteDriver(siteID string) (map[string]string, error) {
	driver, siteJSON, err := ReadDriver(a.sitesDir, siteID)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"site_id":         siteID,
		"dom_driver_js":   driver,
		"site_json":       siteJSON,
		"protection_note": ReadProtectionNote(a.sitesDir, siteID),
		"selector_family": mustFind(a.sites, siteID).Family,
	}, nil
}

// LoadFolder lists .txt sequences from data/sequences/.
func (a *App) LoadFolder() (map[string]any, error) {
	dir := filepath.Join(a.dataDir, "sequences")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create sequences dir: %w", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".txt") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	a.log.Append("system", fmt.Sprintf("folder loaded: %d sequence file(s)", len(names)), "sequences", "")
	return map[string]any{"count": len(names), "names": names}, nil
}

// OpenDataFolder returns the absolute data path (the desktop build opens Explorer).
func (a *App) OpenDataFolder() (string, error) {
	abs, err := filepath.Abs(a.dataDir)
	if err != nil {
		return a.dataDir, nil
	}
	a.log.Append("system", "data folder: "+abs, "storage", "")
	return abs, nil
}

// --- small helpers ----------------------------------------------------------

func readFileOrEmpty(path string) []byte {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return raw
}

func splitLines(raw string) []string {
	out := make([]string, 0, 8)
	for _, line := range strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func normaliseHostPort(line, scheme string) string {
	value := strings.TrimSpace(line)
	for _, prefix := range []string{"http://", "https://", "socks5://", "socks4://"} {
		value = strings.TrimPrefix(value, prefix)
	}
	if at := strings.LastIndex(value, "@"); at >= 0 {
		value = value[at+1:]
	}
	parts := strings.Split(value, ":")
	if len(parts) >= 2 {
		return parts[0] + ":" + parts[1]
	}
	if scheme == "socks5" {
		return value + ":1080"
	}
	return value + ":8080"
}

func hostOf(raw string) string {
	trimmed := raw
	for _, prefix := range []string{"https://", "http://"} {
		trimmed = strings.TrimPrefix(trimmed, prefix)
	}
	if slash := strings.Index(trimmed, "/"); slash >= 0 {
		trimmed = trimmed[:slash]
	}
	return trimmed
}

func shortError(err error) string {
	text := err.Error()
	if len(text) > 120 {
		return text[:117] + "..."
	}
	return text
}

func fallback(value, def string) string {
	if strings.TrimSpace(value) == "" {
		return def
	}
	return value
}

func mustFind(sites []SiteConfig, id string) SiteConfig {
	site, _ := FindSite(sites, id)
	return site
}
