package core

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// Handler returns the whole HTTP surface: the JSON API under /api/ and the
// dashboard itself at /. The frontend's bridge talks to exactly these routes
// when it is not running inside Wails (see design-kit/08-API-CONTRACT.md).
func (a *App) Handler(uiDir string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"ok": true, "version": Version, "kind": a.kind, "sites": len(a.sites)}, nil)
	})

	mux.HandleFunc("/api/GetSnapshot", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			writeJSON(w, nil, errBadMethod(r))
			return
		}
		writeJSON(w, a.Snapshot(), nil)
	})

	mux.HandleFunc("/api/SaveSettings", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Section string          `json:"section"`
			Values  json.RawMessage `json:"values"`
		}
		if err := decodeBody(r, &req); err != nil {
			writeJSON(w, nil, err)
			return
		}
		snapshot, err := a.SaveSettings(req.Section, req.Values)
		writeJSON(w, snapshot, err)
	})

	mux.HandleFunc("/api/GetLogs", func(w http.ResponseWriter, r *http.Request) {
		after, _ := strconv.Atoi(r.URL.Query().Get("after"))
		if after == 0 && r.Method == http.MethodPost {
			var req struct {
				After int `json:"after"`
			}
			_ = decodeBody(r, &req)
			after = req.After
		}
		writeJSON(w, a.Logs(after), nil)
	})

	mux.HandleFunc("/api/ClearLogs", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, a.ClearLogs(), nil)
	})

	mux.HandleFunc("/api/TestSite", func(w http.ResponseWriter, r *http.Request) {
		siteID := r.URL.Query().Get("id")
		if siteID == "" {
			var req struct {
				SiteID string `json:"site_id"`
			}
			_ = decodeBody(r, &req)
			siteID = req.SiteID
		}
		result, err := a.TestSite(siteID)
		writeJSON(w, result, err)
	})

	mux.HandleFunc("/api/TestConnections", func(w http.ResponseWriter, r *http.Request) {
		var req ProxySettings
		if err := decodeBody(r, &req); err != nil {
			writeJSON(w, nil, err)
			return
		}
		results, err := a.TestConnections(req)
		writeJSON(w, results, err)
	})

	mux.HandleFunc("/api/StartRun", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Mode    string   `json:"mode"`
			Sites   []string `json:"sites"`
			Profile string   `json:"profile"`
		}
		if err := decodeBody(r, &req); err != nil {
			writeJSON(w, nil, err)
			return
		}
		out, err := a.StartRun(req.Mode, req.Sites, req.Profile)
		writeJSON(w, out, err)
	})

	mux.HandleFunc("/api/StopRun", func(w http.ResponseWriter, r *http.Request) {
		out, err := a.StopRun()
		writeJSON(w, out, err)
	})

	mux.HandleFunc("/api/LaunchLogin", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			SiteID  string `json:"site_id"`
			Profile string `json:"profile"`
		}
		if err := decodeBody(r, &req); err != nil {
			writeJSON(w, nil, err)
			return
		}
		out, err := a.LaunchLogin(req.SiteID, req.Profile)
		writeJSON(w, out, err)
	})

	mux.HandleFunc("/api/AutoRegisterAccounts", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			SiteID string `json:"site_id"`
			Count  int    `json:"count"`
		}
		if err := decodeBody(r, &req); err != nil {
			writeJSON(w, nil, err)
			return
		}
		accounts, err := a.AutoRegisterAccounts(req.SiteID, req.Count)
		writeJSON(w, map[string]any{"accounts": accounts, "count": len(accounts)}, err)
	})

	mux.HandleFunc("/api/GetSiteDriver", func(w http.ResponseWriter, r *http.Request) {
		siteID := r.URL.Query().Get("id")
		if siteID == "" {
			var req struct {
				SiteID string `json:"site_id"`
			}
			_ = decodeBody(r, &req)
			siteID = req.SiteID
		}
		driver, err := a.SiteDriver(siteID)
		writeJSON(w, driver, err)
	})

	mux.HandleFunc("/api/GetBrowserStatus", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, a.GetBrowserStatus(), nil)
	})

	mux.HandleFunc("/api/TestSiteBrowser", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			SiteID  string `json:"site_id"`
			Profile string `json:"profile"`
		}
		if err := decodeBody(r, &req); err != nil {
			writeJSON(w, nil, err)
			return
		}
		if req.SiteID == "" {
			req.SiteID = r.URL.Query().Get("id")
		}
		if req.SiteID == "" {
			writeJSON(w, nil, errMissing("site_id"))
			return
		}
		out, err := a.TestSiteBrowser(req.SiteID, req.Profile)
		writeJSON(w, out, err)
	})

	mux.HandleFunc("/api/GetSiteDiagnostics", func(w http.ResponseWriter, r *http.Request) {
		siteID := r.URL.Query().Get("id")
		if siteID == "" {
			var req struct {
				SiteID string `json:"site_id"`
			}
			_ = decodeBody(r, &req)
			siteID = req.SiteID
		}
		if siteID == "" {
			writeJSON(w, a.AllDiagnostics(), nil)
			return
		}
		diag, err := a.GetSiteDiagnostics(siteID)
		writeJSON(w, diag, err)
	})

	mux.HandleFunc("/api/GetSessions", func(w http.ResponseWriter, r *http.Request) {
		siteID := r.URL.Query().Get("id")
		if siteID == "" {
			var req struct {
				SiteID string `json:"site_id"`
			}
			_ = decodeBody(r, &req)
			siteID = req.SiteID
		}
		if siteID == "" {
			writeJSON(w, nil, errMissing("site_id"))
			return
		}
		sessions, err := a.GetSessions(siteID)
		writeJSON(w, map[string]any{"site_id": siteID, "sessions": sessions, "count": len(sessions)}, err)
	})

	mux.HandleFunc("/api/SaveSession", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			SiteID  string         `json:"site_id"`
			Profile string         `json:"profile"`
			Values  map[string]any `json:"values"`
		}
		if err := decodeBody(r, &req); err != nil {
			writeJSON(w, nil, err)
			return
		}
		if req.SiteID == "" {
			writeJSON(w, nil, errMissing("site_id"))
			return
		}
		if req.Profile == "" {
			req.Profile = "default"
		}
		session, err := a.SaveSession(req.SiteID, req.Profile, req.Values)
		writeJSON(w, session, err)
	})

	mux.HandleFunc("/api/DeleteSession", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			SiteID  string `json:"site_id"`
			Profile string `json:"profile"`
		}
		if err := decodeBody(r, &req); err != nil {
			writeJSON(w, nil, err)
			return
		}
		if req.SiteID == "" {
			writeJSON(w, nil, errMissing("site_id"))
			return
		}
		if req.Profile == "" {
			writeJSON(w, nil, errMissing("profile"))
			return
		}
		out, err := a.DeleteSession(req.SiteID, req.Profile)
		writeJSON(w, out, err)
	})

	mux.HandleFunc("/api/LoadFolder", func(w http.ResponseWriter, r *http.Request) {
		out, err := a.LoadFolder()
		writeJSON(w, out, err)
	})

	mux.HandleFunc("/api/OpenDataFolder", func(w http.ResponseWriter, r *http.Request) {
		path, err := a.OpenDataFolder()
		writeJSON(w, map[string]any{"path": path}, err)
	})

	// static dashboard (index.html, styles.css, app.js, js/mock-backend.js)
	mux.Handle("/", http.FileServer(http.Dir(uiDir)))
	return withNoStore(mux)
}

func withNoStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, value any, err error) {
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	if value == nil {
		value = map[string]any{"ok": true}
	}
	_ = json.NewEncoder(w).Encode(value)
}

func decodeBody(r *http.Request, target any) error {
	if r.Body == nil {
		return nil
	}
	defer r.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil
	}
	return json.Unmarshal(raw, target)
}

func errMissing(field string) error {
	return &missingError{field: field}
}

type missingError struct{ field string }

func (m *missingError) Error() string { return "missing required field: " + m.field }

func errBadMethod(r *http.Request) error {
	return &methodError{method: r.Method, path: r.URL.Path}
}

type methodError struct{ method, path string }

func (m *methodError) Error() string { return "method " + m.method + " not allowed on " + m.path }

// EnsureUIDir is a startup helper: it fails fast when the UI is missing.
func EnsureUIDir(uiDir string) error {
	info, err := os.Stat(uiDir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return &os.PathError{Op: "stat", Path: uiDir, Err: os.ErrInvalid}
	}
	return nil
}
