// Command desktop is the Wails shell for the Multi-Site Console.
//
// It is a separate Go module on purpose: the console core (`internal/core`) is
// stdlib-only and stays buildable/testable anywhere, while this shell pulls the
// Wails runtime. Build it on Windows with `wails build`
// (scripts/build-desktop.bat); `go test ./...` at the repository root never
// touches this module.
package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"multisiteconsole/internal/core"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app, dataDir, sitesDir, err := bootstrap()
	if err != nil {
		log.Fatalf("startup: %v", err)
	}
	defer app.Close()

	fmt.Printf("Multi-Site Console %s (desktop)\n  data : %s\n  sites: %s\n", core.Version, dataDir, sitesDir)

	err = wails.Run(&options.App{
		Title:     "Multi-Site Console " + core.Version,
		Width:     1320,
		Height:    900,
		MinWidth:  1080,
		MinHeight: 720,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 18, G: 22, B: 20, A: 1},
		Bind:             []interface{}{&api{app: app}},
	})
	if err != nil {
		log.Fatalf("wails: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Binding surface
//
// The dashboard always calls one object per method (bridge.invoke(name, payload)).
// Every method below therefore takes exactly one argument struct so the same
// app.js works unchanged over HTTP (/api/<Method>) and over the native bridge.
// ---------------------------------------------------------------------------

type api struct {
	app *core.App
}

type EmptyArgs struct{}

type SiteArgs struct {
	SiteID string `json:"site_id"`
}

type SiteProfileArgs struct {
	SiteID  string `json:"site_id"`
	Profile string `json:"profile"`
}

type LogsArgs struct {
	After int `json:"after"`
}

type SaveSettingsArgs struct {
	Section string          `json:"section"`
	Values  json.RawMessage `json:"values"`
}

type RunArgs struct {
	Mode    string   `json:"mode"`
	Sites   []string `json:"sites"`
	Profile string   `json:"profile"`
}

type RegisterArgs struct {
	SiteID string `json:"site_id"`
	Count  int    `json:"count"`
}

type FolderArgs struct {
	Dir string `json:"dir"`
}

type SaveSessionArgs struct {
	SiteID  string         `json:"site_id"`
	Profile string         `json:"profile"`
	Values  map[string]any `json:"values"`
}

func (a *api) GetSnapshot(EmptyArgs) core.Snapshot { return a.app.Snapshot() }

func (a *api) SaveSettings(args SaveSettingsArgs) (core.Snapshot, error) {
	return a.app.SaveSettings(args.Section, args.Values)
}

func (a *api) GetLogs(args LogsArgs) core.LogPage { return a.app.Logs(args.After) }

func (a *api) ClearLogs(EmptyArgs) map[string]any { return a.app.ClearLogs() }

func (a *api) TestSite(args SiteArgs) (core.TestResult, error) { return a.app.TestSite(args.SiteID) }

func (a *api) TestSiteBrowser(args SiteProfileArgs) (map[string]any, error) {
	return a.app.TestSiteBrowser(args.SiteID, args.Profile)
}

func (a *api) GetBrowserStatus(EmptyArgs) core.BrowserStatus { return a.app.GetBrowserStatus() }

func (a *api) TestConnections(args core.ProxySettings) (map[string][]core.TestResult, error) {
	return a.app.TestConnections(args)
}

func (a *api) StartRun(args RunArgs) (map[string]any, error) {
	return a.app.StartRun(args.Mode, args.Sites, args.Profile)
}

func (a *api) StopRun(EmptyArgs) (map[string]any, error) { return a.app.StopRun() }

func (a *api) LaunchLogin(args SiteProfileArgs) (map[string]any, error) {
	return a.app.LaunchLogin(args.SiteID, args.Profile)
}

func (a *api) AutoRegisterAccounts(args RegisterArgs) (map[string]any, error) {
	accounts, err := a.app.AutoRegisterAccounts(args.SiteID, args.Count)
	if err != nil {
		return nil, err
	}
	return map[string]any{"accounts": accounts, "count": len(accounts)}, nil
}

func (a *api) GetSiteDriver(args SiteArgs) (map[string]string, error) {
	return a.app.SiteDriver(args.SiteID)
}

func (a *api) LoadFolder(FolderArgs) (map[string]any, error) { return a.app.LoadFolder() }

func (a *api) OpenDataFolder(EmptyArgs) (map[string]any, error) {
	path, err := a.app.OpenDataFolder()
	if err != nil {
		return nil, err
	}
	return map[string]any{"path": path}, nil
}

func (a *api) GetSiteDiagnostics(args SiteArgs) (core.SiteDiagnostics, error) {
	return a.app.GetSiteDiagnostics(args.SiteID)
}

func (a *api) GetSessions(args SiteArgs) (map[string]any, error) {
	sessions, err := a.app.GetSessions(args.SiteID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"site_id": args.SiteID, "sessions": sessions, "count": len(sessions)}, nil
}

func (a *api) SaveSession(args SaveSessionArgs) (core.SiteSession, error) {
	return a.app.SaveSession(args.SiteID, args.Profile, args.Values)
}

func (a *api) DeleteSession(args SiteProfileArgs) (map[string]any, error) {
	return a.app.DeleteSession(args.SiteID, args.Profile)
}

// ---------------------------------------------------------------------------
// Startup paths
// ---------------------------------------------------------------------------

// bootstrap resolves the data + sites folders the same way the web build does,
// but rooted in the user's profile so a desktop install never writes next to
// Program Files.
func bootstrap() (*core.App, string, string, error) {
	base := os.Getenv("MULTISITE_HOME")
	if base == "" {
		configDir, err := os.UserConfigDir()
		if err != nil {
			execDir, execErr := os.Executable()
			if execErr != nil {
				return nil, "", "", err
			}
			base = filepath.Dir(execDir)
		} else {
			base = filepath.Join(configDir, "MultiSiteConsole")
		}
	}

	dataDir := filepath.Join(base, "data")
	sitesDir := os.Getenv("MULTISITE_SITES")
	if sitesDir == "" {
		sitesDir = filepath.Join(base, "sites")
		if _, err := os.Stat(sitesDir); err != nil {
			// fall back to the sites folder shipped beside the executable
			execDir, execErr := os.Executable()
			if execErr == nil {
				candidate := filepath.Join(filepath.Dir(execDir), "sites")
				if info, statErr := os.Stat(candidate); statErr == nil && info.IsDir() {
					sitesDir = candidate
				}
			}
		}
	}

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, dataDir, sitesDir, fmt.Errorf("create data dir: %w", err)
	}

	app, err := core.New(dataDir, sitesDir, "wails")
	if err != nil {
		return nil, dataDir, sitesDir, err
	}
	return app, dataDir, sitesDir, nil
}
