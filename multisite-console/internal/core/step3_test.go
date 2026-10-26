package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Step 3 · site-engine depth + hardening
// ---------------------------------------------------------------------------

func TestSelectorCoverageCountsFilled(t *testing.T) {
	full := Selectors{
		ComposerInput: "#msg", SendButton: "button.send", SkipButton: "button.skip",
		StartButton: "button.start", MessageHistory: "#history", MessageRow: ".row",
		MessageText: ".row .text", PartnerMessageSelector: ".in", OwnMessageSelector: ".out",
		PartnerName: ".name", ChatEndedText: "text=ended",
	}
	filled, empty := SelectorCoverage(full)
	if filled != 11 || len(empty) != 0 {
		t.Fatalf("expected 11/11, got %d filled, empty=%v", filled, empty)
	}

	partial := Selectors{ComposerInput: "#msg", SendButton: "button.send"}
	filled, empty = SelectorCoverage(partial)
	if filled != 2 || len(empty) != 9 {
		t.Fatalf("expected 2/11 with 9 empty, got %d/%d", filled, len(empty))
	}
	if empty[0] != "skip_button" {
		t.Fatalf("empty list must follow field order, got %v", empty)
	}
}

func TestDiskLogWritesAndRedacts(t *testing.T) {
	dataDir := t.TempDir()
	disk, err := OpenDiskLog(dataDir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	disk.Write("app", "system", "probe with api_key=sk-abc123456789 and token: deadbeef99")
	disk.Write("site", "error", "password=hunter2 for alpha")
	if err := disk.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dataDir, "logs.txt"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	text := string(raw)
	if strings.Contains(text, "sk-abc123456789") || strings.Contains(text, "deadbeef99") || strings.Contains(text, "hunter2") {
		t.Fatalf("secrets leaked into log: %s", text)
	}
	if !strings.Contains(text, "[redacted]") {
		t.Fatalf("expected redaction marker, got: %s", text)
	}
	if !strings.Contains(text, "| app ") || !strings.Contains(text, "| site ") {
		t.Fatalf("expected source column, got: %s", text)
	}
	if lines := strings.Split(strings.TrimSpace(text), "\n"); len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
}

func TestRedactKeepsHarmlessText(t *testing.T) {
	in := "GET https://example.com/chat 200 in 231ms"
	if got := Redact(in); got != in {
		t.Fatalf("Redact changed harmless text: %q", got)
	}
}

func TestSessionLifecycle(t *testing.T) {
	sitesDir := t.TempDir()
	siteDir := filepath.Join(sitesDir, "alpha")
	if err := os.MkdirAll(siteDir, 0o755); err != nil {
		t.Fatal(err)
	}

	sessions, err := ListSessions(sitesDir, "alpha")
	if err != nil {
		t.Fatalf("list empty: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected no sessions, got %d", len(sessions))
	}

	saved, err := SaveSession(sitesDir, "alpha", "default", map[string]any{
		"cookies":       []any{"a", "b", "c"},
		"local_storage": map[string]any{"k": "v"},
		"http_status":   float64(200),
		"note":          "18+ accepted",
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if saved.Cookies != 3 || saved.LocalStorage != 1 || saved.HTTPStatus != 200 {
		t.Fatalf("unexpected session counts: %+v", saved)
	}
	if _, err := os.Stat(filepath.Join(siteDir, "profiles", "default", "session.json")); err != nil {
		t.Fatalf("session.json missing: %v", err)
	}

	sessions, err = ListSessions(sitesDir, "alpha")
	if err != nil || len(sessions) != 1 || sessions[0].Profile != "default" {
		t.Fatalf("list after save: %+v (%v)", sessions, err)
	}

	// a second save keeps created_at but updates the counters
	second, err := SaveSession(sitesDir, "alpha", "default", map[string]any{"cookies": []any{"x"}})
	if err != nil {
		t.Fatalf("resave: %v", err)
	}
	if !second.CreatedAt.Equal(saved.CreatedAt) {
		t.Fatalf("created_at must be preserved: %v vs %v", saved.CreatedAt, second.CreatedAt)
	}
	if second.Cookies != 1 {
		t.Fatalf("expected 1 cookie, got %d", second.Cookies)
	}

	if err := DeleteSession(sitesDir, "alpha", "default"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := os.Stat(filepath.Join(siteDir, "profiles", "default")); !os.IsNotExist(err) {
		t.Fatalf("profile dir should be gone, stat err = %v", err)
	}
	if err := DeleteSession(sitesDir, "alpha", ""); err == nil {
		t.Fatal("empty profile must be rejected")
	}
}

func TestDiagnosticsShape(t *testing.T) {
	app := newTestApp(t, 2)
	diag, err := app.GetSiteDiagnostics("alpha")
	if err != nil {
		t.Fatalf("diagnostics: %v", err)
	}
	if diag.SiteID != "alpha" || diag.SelectorTotal != 11 {
		t.Fatalf("unexpected header: %+v", diag)
	}
	if diag.SelectorFilled != 2 {
		t.Fatalf("test fixture defines 2 selectors, got %d", diag.SelectorFilled)
	}
	if len(diag.Checks) < 7 {
		t.Fatalf("expected at least 7 checks, got %d", len(diag.Checks))
	}
	if diag.DriverBytes == 0 {
		t.Fatal("driver bytes must be counted")
	}
	if diag.Ready {
		t.Fatal("fixture has too few selectors and no sequences/accounts, must not be ready")
	}
	for _, check := range diag.Checks {
		if check.Name == "" || check.Detail == "" {
			t.Fatalf("check without name/detail: %+v", check)
		}
	}

	if _, err := app.GetSiteDiagnostics("missing"); err == nil {
		t.Fatal("unknown site must error")
	}
	if all := app.AllDiagnostics(); len(all) != 2 {
		t.Fatalf("expected 2 diagnostics, got %d", len(all))
	}
}

func TestDiagnosticsBecomesReadyWithSetup(t *testing.T) {
	app := newTestApp(t, 1)

	// fill enough selectors in the fixture
	cfgPath := filepath.Join(app.SitesDir(), "alpha", "site.json")
	cfg := app.sites[0]
	cfg.Selectors = Selectors{
		ComposerInput: "#msg", SendButton: "button.send", SkipButton: "button.skip",
		StartButton: "button.start", MessageHistory: "#history", MessageRow: ".row",
	}
	cfg.ProtectionSummary = "keep it slow"
	raw, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(cfgPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app.SitesDir(), "alpha", "dom_driver.js"), []byte(strings.Repeat("// driver line\n", 60)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app.SitesDir(), "alpha", "alpha_protection.txt"), []byte("keep it slow\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(app.SitesDir(), "alpha", "accounts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app.SitesDir(), "alpha", "accounts", "a.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	seqDir := filepath.Join(app.DataDir(), "sequences")
	os.MkdirAll(seqDir, 0o755)
	os.WriteFile(filepath.Join(seqDir, "a.txt"), []byte("hi\n"), 0o644)

	// reload the registry so the app sees the edited site.json
	reloaded, err := New(app.DataDir(), app.SitesDir(), "http")
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	diag, err := reloaded.GetSiteDiagnostics("alpha")
	if err != nil {
		t.Fatalf("diagnostics: %v", err)
	}
	if !diag.Ready {
		var failing []string
		for _, check := range diag.Checks {
			if !check.OK {
				failing = append(failing, check.Name+": "+check.Detail)
			}
		}
		t.Fatalf("expected ready, failing checks: %v", failing)
	}
	if len(diag.SequenceFiles) != 1 || diag.SequenceFiles[0] != "a.txt" {
		t.Fatalf("unexpected sequence list: %v", diag.SequenceFiles)
	}
}

func TestProbeReadsDOMEvidence(t *testing.T) {
	page := `<!doctype html><html><head><title> Example  Chat </title></head>
<body><textarea id="msg"></textarea><script src="/socket.io/socket.io.js"></script>
<div class="messages"></div></body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(page))
	}))
	defer server.Close()

	app := newTestApp(t, 1)
	result, err := app.probe(server.URL, "alpha")
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if !result.OK || !result.Ready {
		t.Fatalf("expected ok+ready, got %+v", result)
	}
	if result.Title != "Example Chat" {
		t.Fatalf("title not normalised: %q", result.Title)
	}
	if result.Bytes == 0 {
		t.Fatal("bytes must be counted")
	}
	joined := strings.Join(result.Evidence, ",")
	for _, want := range []string{"textarea composer", "socket.io", "message list"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing evidence %q in %v", want, result.Evidence)
		}
	}
	if history := app.latencyHistory("alpha"); len(history) != 1 || history[0] <= 0 {
		t.Fatalf("latency history not recorded: %v", history)
	}
}

func TestProbeFollowsRedirect(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html><title>moved</title><textarea></textarea></html>"))
	}))
	defer target.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer server.Close()

	app := newTestApp(t, 1)
	result, err := app.probe(server.URL, "alpha")
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if !strings.HasPrefix(result.FinalURL, target.URL) {
		t.Fatalf("final url not captured: %q", result.FinalURL)
	}
	if result.Redirects < 1 {
		t.Fatalf("expected at least one redirect, got %d", result.Redirects)
	}
}

func TestProbeReportsNetworkFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := server.URL
	server.Close() // nothing is listening any more

	app := newTestApp(t, 1)
	result, err := app.probe(url, "alpha")
	if err != nil {
		t.Fatalf("probe must not return an error for transport failures: %v", err)
	}
	if result.OK || result.Ready {
		t.Fatalf("dead endpoint must fail: %+v", result)
	}
	if result.Message == "" {
		t.Fatal("failure must carry a message")
	}
}

func TestReadSequencesAndFolderPerSite(t *testing.T) {
	app := newTestApp(t, 2)
	seqDir := filepath.Join(app.DataDir(), "sequences")
	if err := os.MkdirAll(seqDir, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(seqDir, "one.txt"), []byte("a\nb\n\nc\n"), 0o644)
	os.WriteFile(filepath.Join(seqDir, "two.txt"), []byte("x\n"), 0o644)
	os.WriteFile(filepath.Join(seqDir, "ignore.md"), []byte("nope\n"), 0o644)

	sequences, err := ReadSequences(app.DataDir())
	if err != nil {
		t.Fatalf("read sequences: %v", err)
	}
	if len(sequences) != 2 {
		t.Fatalf("expected 2 txt files, got %d", len(sequences))
	}
	if sequences[0].Name != "one.txt" || len(sequences[0].Lines) != 3 {
		t.Fatalf("blank lines must be dropped: %+v", sequences[0])
	}

	first := app.sequenceForSite("alpha")
	second := app.sequenceForSite("bravo")
	if len(first) == 0 || len(second) == 0 {
		t.Fatalf("sequenceForSite returned nothing: %v %v", first, second)
	}
	if strings.Join(first, "|") == strings.Join(second, "|") {
		t.Fatal("two sites must not receive the same script")
	}

	out, err := app.LoadFolder()
	if err != nil {
		t.Fatalf("load folder: %v", err)
	}
	if out["count"].(int) != 2 {
		t.Fatalf("expected count 2, got %v", out["count"])
	}
	if out["total"].(int) != 4 {
		t.Fatalf("expected 4 total lines, got %v", out["total"])
	}
}

func TestFolderStrategyLogsSequenceUse(t *testing.T) {
	app := newTestApp(t, 1)
	seqDir := filepath.Join(app.DataDir(), "sequences")
	os.MkdirAll(seqDir, 0o755)
	os.WriteFile(filepath.Join(seqDir, "only.txt"), []byte("from the file\n"), 0o644)

	if _, err := app.SaveSettings("message", json.RawMessage(`{"strategy":"folder","max_lines":2}`)); err != nil {
		t.Fatalf("save message settings: %v", err)
	}
	if _, err := app.StartRun("worker", []string{"alpha"}, "default"); err != nil {
		t.Fatalf("start: %v", err)
	}
	deadline := 0
	for {
		app.mu.Lock()
		running := app.running
		app.mu.Unlock()
		if !running || deadline > 80 {
			break
		}
		deadline++
		sleepMs(100)
	}

	_, entries, _ := app.Log().Since(0)
	found := false
	for _, entry := range entries {
		if strings.Contains(entry.Text, "folder strategy") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected a 'folder strategy' log line when strategy=folder")
	}
}

func TestLogfWritesRingAndDisk(t *testing.T) {
	app := newTestApp(t, 1)
	app.logf("system", "hello disk", "test", "")
	path := app.LogPath()
	if path == "" {
		t.Fatal("expected a log path")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(raw), "hello disk") {
		t.Fatalf("entry missing from file log: %s", raw)
	}
	_, ringEntries, _ := app.Log().Since(0)
	if len(ringEntries) == 0 {
		t.Fatal("entry missing from ring")
	}
	if err := app.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestSiteStatesExposeStep3Fields(t *testing.T) {
	app := newTestApp(t, 2)
	states := app.SiteStates()
	for _, state := range states {
		if state.SelectorTotal != 11 {
			t.Fatalf("selector_total must be 11, got %d", state.SelectorTotal)
		}
		if state.SelectorFilled != 2 {
			t.Fatalf("fixture fills 2 selectors, got %d", state.SelectorFilled)
		}
		if state.DiagReady {
			t.Fatalf("fixture must not be diag-ready: %+v", state)
		}
	}
}

func TestHTTPDiagnosticsAndSessions(t *testing.T) {
	app := newTestApp(t, 2)
	server := newTestServer(t, app)

	var diag SiteDiagnostics
	if code := getJSON(t, server.URL+"/api/GetSiteDiagnostics?id=alpha", &diag); code != http.StatusOK {
		t.Fatalf("diagnostics status %d", code)
	}
	if diag.SiteID != "alpha" || len(diag.Checks) == 0 {
		t.Fatalf("unexpected diagnostics: %+v", diag)
	}

	var list map[string]any
	if code := getJSON(t, server.URL+"/api/GetSessions?id=alpha", &list); code != http.StatusOK {
		t.Fatalf("sessions status %d", code)
	}
	if list["count"].(float64) != 0 {
		t.Fatalf("expected 0 sessions, got %v", list["count"])
	}

	var saved SiteSession
	code, body := postJSON(t, server.URL+"/api/SaveSession", map[string]any{
		"site_id": "alpha", "profile": "work", "values": map[string]any{"cookies": []any{"a"}},
	}, &saved)
	if code != http.StatusOK {
		t.Fatalf("save session status %d (%s)", code, body)
	}
	if saved.Profile != "work" || saved.Cookies != 1 {
		t.Fatalf("unexpected session: %+v", saved)
	}

	var deleted map[string]any
	if code, _ := postJSON(t, server.URL+"/api/DeleteSession", map[string]any{"site_id": "alpha", "profile": "work"}, &deleted); code != http.StatusOK {
		t.Fatalf("delete session status %d", code)
	}

	// validation paths
	if code, _ := postJSON(t, server.URL+"/api/DeleteSession", map[string]any{"site_id": "alpha"}, &deleted); code != http.StatusBadRequest {
		t.Fatalf("missing profile must be 400, got %d", code)
	}
	if code := getJSON(t, server.URL+"/api/GetSessions?id=nope", &list); code != http.StatusBadRequest {
		t.Fatalf("unknown site must be 400, got %d", code)
	}
	if code, _ := postJSON(t, server.URL+"/api/SaveSession", map[string]any{"profile": "x"}, &saved); code != http.StatusBadRequest {
		t.Fatalf("missing site_id must be 400, got %d", code)
	}
}

func sleepMs(ms int) { time.Sleep(time.Duration(ms) * time.Millisecond) }
