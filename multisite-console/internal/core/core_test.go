package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeSites creates a throwaway registry: n sites, one per family, with a
// driver file so SiteState can see it.
func writeSites(t *testing.T, count int) string {
	t.Helper()
	dir := t.TempDir()
	families := []string{"family_a", "family_b", "family_c", "family_d", "family_e"}
	for i := 0; i < count; i++ {
		id := []string{"alpha", "bravo", "charlie", "delta", "echo"}[i%5]
		siteDir := filepath.Join(dir, id)
		if err := os.MkdirAll(siteDir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		cfg := SiteConfig{
			ID:          id,
			Name:        id + " site",
			Family:      families[i%len(families)],
			AccountMode: "auto_register",
			Enabled:     true,
			StartURL:    "https://example.com/" + id,
			EntryFlow:   "guest entry -> 1v1 text",
			Selectors:   Selectors{ComposerInput: "#msg", SendButton: "button.send"},
		}
		raw, _ := json.MarshalIndent(cfg, "", "  ")
		if err := os.WriteFile(filepath.Join(siteDir, "site.json"), raw, 0o644); err != nil {
			t.Fatalf("write site.json: %v", err)
		}
		if err := os.WriteFile(filepath.Join(siteDir, "dom_driver.js"), []byte("// driver for "+id), 0o644); err != nil {
			t.Fatalf("write driver: %v", err)
		}
	}
	return dir
}

func newTestApp(t *testing.T, siteCount int) *App {
	t.Helper()
	sitesDir := writeSites(t, siteCount)
	dataDir := t.TempDir()
	app, err := New(dataDir, sitesDir, "http")
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	return app
}

func TestLoadSites(t *testing.T) {
	dir := writeSites(t, 5)
	sites, problems := LoadSites(dir)
	if len(problems) != 0 {
		t.Fatalf("unexpected problems: %v", problems)
	}
	if len(sites) != 5 {
		t.Fatalf("want 5 sites, got %d", len(sites))
	}
	seen := map[string]bool{}
	for _, site := range sites {
		if site.ID == "" || site.Name == "" {
			t.Fatalf("site missing identity: %+v", site)
		}
		if seen[site.Family] && len(sites) == 5 {
			// families must be distinct for the 5-site fixture
			t.Logf("family repeated: %s", site.Family)
		}
		seen[site.Family] = true
	}
	if len(seen) != 5 {
		t.Fatalf("want 5 distinct families, got %d", len(seen))
	}
}

func TestLoadSitesSkipsBroken(t *testing.T) {
	dir := writeSites(t, 1)
	broken := filepath.Join(dir, "broken")
	if err := os.MkdirAll(broken, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(broken, "site.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	sites, problems := LoadSites(dir)
	if len(sites) != 1 {
		t.Fatalf("want 1 usable site, got %d", len(sites))
	}
	if len(problems) != 1 {
		t.Fatalf("want 1 reported problem, got %d: %v", len(problems), problems)
	}
}

func TestDefaultsAndNormalise(t *testing.T) {
	ids := []string{"alpha", "bravo"}
	settings := Defaults(ids)
	if settings.Show.SiteID != "alpha" {
		t.Fatalf("show site should default to the first site, got %q", settings.Show.SiteID)
	}

	bad := Settings{}
	bad.Show.First.Mode = "nonsense"
	bad.Message.Strategy = "weird"
	bad.Proxy.Scheme = "ftp"
	bad.Accounts.Gender = "x"
	bad.Accounts.Count = 99
	bad.Worker.Concurrency = 0
	bad.Worker.PollMin = 20
	bad.Worker.PollMax = 5
	bad.Worker.Sites = []string{"alpha", "ghost", "alpha"}
	bad.Normalise(ids)

	if bad.Show.First.Mode != "bot" || bad.Message.Strategy != "single" || bad.Proxy.Scheme != "http" || bad.Accounts.Gender != "female" {
		t.Fatalf("enums not normalised: %+v", bad)
	}
	if bad.Accounts.Count != 20 || bad.Worker.Concurrency != 1 {
		t.Fatalf("clamps failed: count=%d conc=%d", bad.Accounts.Count, bad.Worker.Concurrency)
	}
	if bad.Worker.PollMax < bad.Worker.PollMin {
		t.Fatalf("max must be >= min: %d < %d", bad.Worker.PollMax, bad.Worker.PollMin)
	}
	if len(bad.Worker.Sites) != 1 || bad.Worker.Sites[0] != "alpha" {
		t.Fatalf("site list not cleaned: %v", bad.Worker.Sites)
	}
	if bad.Show.SiteID != "alpha" {
		t.Fatalf("show site not repaired, got %q", bad.Show.SiteID)
	}
}

func TestStorePersistsAcrossReopen(t *testing.T) {
	sitesDir := writeSites(t, 2)
	dataDir := t.TempDir()
	app, err := New(dataDir, sitesDir, "http")
	if err != nil {
		t.Fatal(err)
	}
	values := json.RawMessage(`{"strategy":"folder","sequence":"one\ntwo","max_lines":2}`)
	if _, err := app.SaveSettings("message", values); err != nil {
		t.Fatalf("save message: %v", err)
	}

	// reopen: the value must survive a process restart
	reopened, err := New(dataDir, sitesDir, "http")
	if err != nil {
		t.Fatal(err)
	}
	got := reopened.Snapshot().Settings.Message
	if got.Strategy != "folder" || got.MaxLines != 2 || got.Sequence == "" {
		t.Fatalf("settings did not persist: %+v", got)
	}
}

func TestStoreRejectsUnknownSection(t *testing.T) {
	app := newTestApp(t, 1)
	if _, err := app.SaveSettings("nope", json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected an error for an unknown section")
	}
}

func TestSnapshotShape(t *testing.T) {
	app := newTestApp(t, 5)
	snapshot := app.Snapshot()
	if len(snapshot.Sites) != 5 {
		t.Fatalf("want 5 sites in snapshot, got %d", len(snapshot.Sites))
	}
	if snapshot.Runtime.Kind != "http" || snapshot.Runtime.Version != Version {
		t.Fatalf("bad runtime block: %+v", snapshot.Runtime)
	}
	if snapshot.Stats.Sites != 5 {
		t.Fatalf("bad stats: %+v", snapshot.Stats)
	}
	if snapshot.Strategy != "single" {
		t.Fatalf("strategy should mirror message.strategy, got %q", snapshot.Strategy)
	}
	for _, site := range snapshot.Sites {
		if site.Status == "" || site.LastTest == "" {
			t.Fatalf("site state incomplete: %+v", site)
		}
	}
}

func TestAutoRegisterAccountsWritesFiles(t *testing.T) {
	app := newTestApp(t, 2)
	accounts, err := app.AutoRegisterAccounts("alpha", 3)
	if err != nil {
		t.Fatalf("auto register: %v", err)
	}
	if len(accounts) != 3 {
		t.Fatalf("want 3 accounts, got %d", len(accounts))
	}
	for _, account := range accounts {
		if _, err := os.Stat(account.FilePath); err != nil {
			t.Fatalf("account file missing: %v", err)
		}
		if account.SiteID != "alpha" || account.Nickname == "" {
			t.Fatalf("bad account: %+v", account)
		}
	}
	states := app.SiteStates()
	found := false
	for _, state := range states {
		if state.ID == "alpha" {
			found = true
			if state.Accounts != 3 {
				t.Fatalf("account count not reflected: %d", state.Accounts)
			}
		}
	}
	if !found {
		t.Fatal("alpha missing from site states")
	}
	if _, err := app.AutoRegisterAccounts("ghost", 2); err == nil {
		t.Fatal("expected error for unknown site")
	}
	if _, err := app.AutoRegisterAccounts("alpha", 99); err == nil {
		t.Fatal("expected error for out-of-range count")
	}
}

func TestRunLifecycle(t *testing.T) {
	app := newTestApp(t, 2)
	settings := app.Snapshot().Settings
	settings.Worker.ActionMin, settings.Worker.ActionMax = 1, 1
	settings.Show.First.ReplyMin, settings.Show.First.ReplyMax = 1, 1
	settings.Message.MaxLines = 1
	settings.Message.Sequence = "hello there"
	if _, err := app.store.Save(settings); err != nil {
		t.Fatal(err)
	}

	if _, err := app.StartRun("show", []string{"alpha"}, "default"); err != nil {
		t.Fatalf("start: %v", err)
	}
	if _, err := app.StartRun("show", []string{"alpha"}, "default"); err == nil {
		t.Fatal("second start must fail while running")
	}

	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		if app.Snapshot().Stats.Completed > 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if app.Snapshot().Stats.Completed == 0 {
		t.Fatal("run did not complete a site")
	}
	if _, err := app.StopRun(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if app.Snapshot().Runtime.Running {
		t.Fatal("runtime still running after stop")
	}

	page := app.Logs(0)
	if len(page.Entries) == 0 {
		t.Fatal("no log entries recorded")
	}
	kinds := map[string]bool{}
	for _, entry := range page.Entries {
		kinds[entry.Kind] = true
	}
	if !kinds["out"] || !kinds["system"] {
		t.Fatalf("expected out and system entries, got %v", kinds)
	}
}

func TestLogRingCursor(t *testing.T) {
	ring := NewLogRing(3)
	ring.Append("system", "one", "", "")
	ring.Append("out", "two", "", "")
	version, entries, _ := ring.Since(0)
	if version != 2 || len(entries) != 2 {
		t.Fatalf("since(0): version=%d entries=%d", version, len(entries))
	}
	_, tail, _ := ring.Since(2)
	if len(tail) != 0 {
		t.Fatalf("since(version) should be empty, got %d", len(tail))
	}
	for i := 0; i < 5; i++ {
		ring.Append("system", "fill", "", "")
	}
	if ring.Len() != 3 {
		t.Fatalf("ring must cap at 3, got %d", ring.Len())
	}
	ring.Clear()
	_, _, cleared := ring.Since(1)
	if !cleared {
		t.Fatal("cleared flag not reported")
	}
	if _, entries, _ := ring.Since(0); len(entries) != 0 {
		t.Fatal("entries should be empty after clear")
	}
}

func TestPaceIsAtLeastOneSecond(t *testing.T) {
	app := newTestApp(t, 1)
	settings := app.Snapshot().Settings
	settings.Worker.ActionMin, settings.Worker.ActionMax = 0, 0
	if got := app.pace(settings, "worker"); got < time.Second {
		t.Fatalf("pace must be >= 1s, got %v", got)
	}
	settings.Worker.ActionMin, settings.Worker.ActionMax = 3, 3
	if got := app.pace(settings, "worker"); got != 3*time.Second {
		t.Fatalf("pace should be 3s, got %v", got)
	}
}
