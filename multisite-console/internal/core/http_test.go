package core

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestServer(t *testing.T, app *App) *httptest.Server {
	t.Helper()
	uiDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(uiDir, "index.html"), []byte("<!doctype html><title>ui</title>"), 0o644); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(app.Handler(uiDir))
	t.Cleanup(server.Close)
	return server
}

func getJSON(t *testing.T, url string, target any) int {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	if target != nil {
		if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
			t.Fatalf("decode %s: %v", url, err)
		}
	}
	return resp.StatusCode
}

func postJSON(t *testing.T, url string, body any, target any) (int, string) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()
	var envelope map[string]json.RawMessage
	decoded := json.NewDecoder(resp.Body)
	if target != nil {
		if err := decoded.Decode(target); err != nil {
			return resp.StatusCode, ""
		}
		return resp.StatusCode, ""
	}
	if err := decoded.Decode(&envelope); err != nil {
		return resp.StatusCode, ""
	}
	if rawErr, ok := envelope["error"]; ok {
		var message string
		_ = json.Unmarshal(rawErr, &message)
		return resp.StatusCode, message
	}
	return resp.StatusCode, ""
}

func TestHTTPHealthAndSnapshot(t *testing.T) {
	app := newTestApp(t, 5)
	server := newTestServer(t, app)

	var health map[string]any
	if code := getJSON(t, server.URL+"/api/health", &health); code != http.StatusOK {
		t.Fatalf("health status %d", code)
	}
	if health["ok"] != true {
		t.Fatalf("health body: %+v", health)
	}

	var snapshot Snapshot
	if code := getJSON(t, server.URL+"/api/GetSnapshot", &snapshot); code != http.StatusOK {
		t.Fatalf("snapshot status %d", code)
	}
	if len(snapshot.Sites) != 5 || snapshot.Runtime.Version != Version {
		t.Fatalf("bad snapshot: sites=%d version=%s", len(snapshot.Sites), snapshot.Runtime.Version)
	}
}

func TestHTTPSaveSettingsAndErrors(t *testing.T) {
	app := newTestApp(t, 2)
	server := newTestServer(t, app)

	var snapshot Snapshot
	code, _ := postJSON(t, server.URL+"/api/SaveSettings", map[string]any{
		"section": "message",
		"values":  map[string]any{"strategy": "model", "max_lines": 4},
	}, &snapshot)
	if code != http.StatusOK {
		t.Fatalf("save status %d", code)
	}
	if snapshot.Settings.Message.Strategy != "model" || snapshot.Settings.Message.MaxLines != 4 {
		t.Fatalf("save not applied: %+v", snapshot.Settings.Message)
	}

	code, message := postJSON(t, server.URL+"/api/SaveSettings", map[string]any{"section": "nope"}, nil)
	if code != http.StatusBadRequest || message == "" {
		t.Fatalf("want 400 + error text, got %d %q", code, message)
	}
}

func TestHTTPLogsAndClear(t *testing.T) {
	app := newTestApp(t, 1)
	server := newTestServer(t, app)

	var page LogPage
	if code := getJSON(t, server.URL+"/api/GetLogs?after=0", &page); code != http.StatusOK {
		t.Fatalf("logs status %d", code)
	}
	if len(page.Entries) == 0 {
		t.Fatal("expected boot log entries")
	}
	cursor := page.Version

	if code, _ := postJSON(t, server.URL+"/api/ClearLogs", map[string]any{}, nil); code != http.StatusOK {
		t.Fatalf("clear status %d", code)
	}
	if code := getJSON(t, server.URL+"/api/GetLogs?after=0", &page); code != http.StatusOK {
		t.Fatalf("logs after clear status %d", code)
	}
	if len(page.Entries) != 0 || !page.Cleared || page.Version == cursor {
		t.Fatalf("clear did not reset feed: entries=%d cleared=%v version=%d", len(page.Entries), page.Cleared, page.Version)
	}
}

func TestHTTPSiteDriverAndFolder(t *testing.T) {
	app := newTestApp(t, 2)
	server := newTestServer(t, app)

	var driver map[string]string
	if code := getJSON(t, server.URL+"/api/GetSiteDriver?id=alpha", &driver); code != http.StatusOK {
		t.Fatalf("driver status %d", code)
	}
	if driver["dom_driver_js"] == "" || driver["site_json"] == "" {
		t.Fatalf("driver payload incomplete: %+v", driver)
	}

	// sequences folder is created on demand and counted
	seqDir := filepath.Join(app.DataDir(), "sequences")
	if err := os.MkdirAll(seqDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seqDir, "a.txt"), []byte("line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var folder map[string]any
	if code, _ := postJSON(t, server.URL+"/api/LoadFolder", map[string]any{}, &folder); code != http.StatusOK {
		t.Fatalf("folder status %d", code)
	}
	if folder["count"].(float64) != 1 {
		t.Fatalf("expected 1 sequence file, got %+v", folder)
	}
}

func TestHTTPRunAndStop(t *testing.T) {
	app := newTestApp(t, 2)
	settings := app.Snapshot().Settings
	settings.Worker.ActionMin, settings.Worker.ActionMax = 1, 1
	settings.Show.First.ReplyMin, settings.Show.First.ReplyMax = 1, 1
	settings.Message.MaxLines = 1
	if _, err := app.store.Save(settings); err != nil {
		t.Fatal(err)
	}
	server := newTestServer(t, app)

	code, message := postJSON(t, server.URL+"/api/StartRun", map[string]any{
		"mode": "worker", "sites": []string{"alpha"}, "profile": "default",
	}, nil)
	if code != http.StatusOK {
		t.Fatalf("start status %d (%s)", code, message)
	}

	var snapshot Snapshot
	getJSON(t, server.URL+"/api/GetSnapshot", &snapshot)
	if !snapshot.Runtime.Running {
		t.Fatal("runtime should report running")
	}

	code, message = postJSON(t, server.URL+"/api/StartRun", map[string]any{"mode": "worker", "sites": []string{}}, nil)
	if code != http.StatusBadRequest || message == "" {
		t.Fatalf("empty site list must be rejected, got %d %q", code, message)
	}

	if code, _ := postJSON(t, server.URL+"/api/StopRun", map[string]any{}, nil); code != http.StatusOK {
		t.Fatalf("stop status %d", code)
	}
	// wait for the goroutine to settle
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		getJSON(t, server.URL+"/api/GetSnapshot", &snapshot)
		if !snapshot.Runtime.Running {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("runtime still running after StopRun")
}

func TestHTTPStaticUIServed(t *testing.T) {
	app := newTestApp(t, 1)
	server := newTestServer(t, app)
	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("index status %d", resp.StatusCode)
	}
	if resp.Header.Get("Cache-Control") == "no-store" {
		t.Fatal("static assets should not be marked no-store")
	}
}
