package core

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DriverSession is one site, one profile, one real browser page.
//
// The site's own dom_driver.js is injected into that page. Everything after the
// injection talks to it through its small API (probe/snapshot/click/focusInput/
// readInput/clearInput), so a site that changes markup only needs a new driver
// file — the console core never learns site specifics.
type DriverSession struct {
	Site      SiteConfig
	Profile   string
	Browser   *browserProcess
	driverJS  string
	installed bool
}

// SiteSnapshot is the driver's view of the page, decoded for Go.
type SiteSnapshot struct {
	Controls struct {
		Phase      string `json:"phase"`
		HasInput   bool   `json:"hasInput"`
		CanSend    bool   `json:"canSend"`
		Protection string `json:"protection"`
		ObserverOK bool   `json:"observerReady"`
		Overflow   bool   `json:"overflow"`
	} `json:"controls"`
	Messages []struct {
		Author string `json:"author"`
		Text   string `json:"text"`
		Kind   string `json:"kind"`
		Order  int    `json:"order"`
	} `json:"messages"`
	Revision int `json:"revision"`
}

const driverGlobal = "window.__chitchatUniversalSiteDOM"

// driverScript reads sites/<id>/dom_driver.js.
func (a *App) driverScript(siteID string) string {
	raw, err := os.ReadFile(filepath.Join(a.sitesDir, siteID, "dom_driver.js"))
	if err != nil {
		return ""
	}
	return string(raw)
}

// profilePath is the persistent profile folder for a site profile.
func (a *App) profilePath(siteID, profile string) string {
	if strings.TrimSpace(profile) == "" {
		profile = "default"
	}
	return filepath.Join(a.sitesDir, siteID, "profiles", profile)
}

// openDriver launches a browser for one site and installs its DOM driver.
func (a *App) openDriver(site SiteConfig, profile string, headless bool, keepAlive bool) (*DriverSession, error) {
	settings := a.store.Get()
	status := DiscoverBrowser(settings.Browser.Path)
	if !status.Found {
		return nil, fmt.Errorf("%s", status.Note)
	}

	script := a.driverScript(site.ID)
	if strings.TrimSpace(script) == "" {
		return nil, fmt.Errorf("sites/%s/dom_driver.js is missing or empty", site.ID)
	}

	a.logf("system", fmt.Sprintf("launching %s (%s · profile %s)", status.Name, boolDetail(headless, "headless", "visible"), profile), "browser", site.ID)

	proc, err := LaunchBrowser(LaunchOptions{
		ExecPath:   status.Path,
		ProfileDir: a.profilePath(site.ID, profile),
		StartURL:   site.StartURL,
		Headless:   headless,
	}, true)
	if err != nil {
		return nil, err
	}

	session := &DriverSession{Site: site, Profile: profile, Browser: proc, driverJS: script}

	if err := proc.Navigate(site.StartURL, 45*time.Second); err != nil {
		proc.Close()
		return nil, err
	}

	// DOM must at least exist; the driver itself handles "still onboarding".
	if !proc.WaitFor("document.readyState === 'interactive' || document.readyState === 'complete'", 20*time.Second) {
		proc.Close()
		return nil, fmt.Errorf("page did not finish loading within 20s")
	}

	if _, err := proc.cdp.evaluate(script, 20*time.Second); err != nil {
		proc.Close()
		return nil, fmt.Errorf("install dom driver: %w", err)
	}
	session.installed = true

	// let the driver's own auto-onboard run (18+ tick, nickname, start button)
	_ = session.Probe()
	if !proc.WaitFor(fmt.Sprintf("!!(%s && %s.snapshot().controls.hasInput)", driverGlobal, driverGlobal), 25*time.Second) {
		a.logf("skip", "composer not detected yet · the driver will keep onboarding while the run continues", "browser", site.ID)
	}

	return session, nil
}

// Probe returns the driver's one-line status string.
func (d *DriverSession) Probe() string {
	if d == nil || d.Browser == nil || d.Browser.cdp == nil {
		return ""
	}
	out, err := d.Browser.cdp.evalString(fmt.Sprintf("(%s ? %s.probe() : 'no-driver')", driverGlobal, driverGlobal), 8*time.Second)
	if err != nil {
		return "probe-error: " + err.Error()
	}
	return out
}

// Snapshot decodes the driver's page state.
func (d *DriverSession) Snapshot() (SiteSnapshot, error) {
	var snapshot SiteSnapshot
	if d == nil || d.Browser == nil || d.Browser.cdp == nil {
		return snapshot, fmt.Errorf("no browser attached")
	}
	expression := fmt.Sprintf("JSON.stringify(%s ? %s.snapshot() : {})", driverGlobal, driverGlobal)
	var raw string
	if err := d.Browser.cdp.evalValue(expression, 8*time.Second, &raw); err != nil {
		return snapshot, err
	}
	if strings.TrimSpace(raw) == "" || raw == "null" {
		return snapshot, fmt.Errorf("driver returned no snapshot")
	}
	if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
		return snapshot, fmt.Errorf("decode snapshot: %w", err)
	}
	return snapshot, nil
}

// Send types one line into the composer and presses the site's send button.
func (d *DriverSession) Send(text string) error {
	if d == nil || d.Browser == nil || d.Browser.cdp == nil {
		return fmt.Errorf("no browser attached")
	}
	cdp := d.Browser.cdp

	var focused bool
	if err := cdp.evalValue(fmt.Sprintf("!!(%s && %s.focusInput())", driverGlobal, driverGlobal), 8*time.Second, &focused); err != nil {
		return err
	}
	if !focused {
		return fmt.Errorf("composer not focusable (still onboarding?)")
	}

	if _, err := cdp.call("Input.insertText", map[string]any{"text": text}, 10*time.Second); err != nil {
		return err
	}

	var clicked string
	if err := cdp.evalValue(fmt.Sprintf("String(%s.click(['SEND','ENTER']) || '')", driverGlobal), 8*time.Second, &clicked); err != nil {
		return err
	}
	if clicked == "" {
		// fall back to a real Enter key press
		if _, err := cdp.call("Input.dispatchKeyEvent", map[string]any{
			"type": "keyDown", "key": "Enter", "code": "Enter", "windowsVirtualKeyCode": 13,
		}, 8*time.Second); err != nil {
			return err
		}
	}

	var remaining string
	_ = cdp.evalValue(fmt.Sprintf("String(%s.readInput())", driverGlobal), 5*time.Second, &remaining)
	if strings.TrimSpace(remaining) != "" {
		_, _ = cdp.evaluate(fmt.Sprintf("%s.clearInput()", driverGlobal), 5*time.Second)
		return fmt.Errorf("composer still holds %q after send", trimForLog(remaining, 40))
	}
	return nil
}

// Click asks the driver to press one of its named controls.
func (d *DriverSession) Click(labels ...string) string {
	if d == nil || d.Browser == nil || d.Browser.cdp == nil {
		return ""
	}
	quoted := make([]string, 0, len(labels))
	for _, label := range labels {
		quoted = append(quoted, `"`+strings.ReplaceAll(label, `"`, "")+`"`)
	}
	expression := fmt.Sprintf("String(%s.click([%s]) || '')", driverGlobal, strings.Join(quoted, ","))
	var out string
	if err := d.Browser.cdp.evalValue(expression, 8*time.Second, &out); err != nil {
		return ""
	}
	return out
}

// Screenshot captures the visible page as PNG (used by "open profile" previews).
func (d *DriverSession) Screenshot() ([]byte, error) {
	if d == nil || d.Browser == nil || d.Browser.cdp == nil {
		return nil, fmt.Errorf("no browser attached")
	}
	raw, err := d.Browser.cdp.call("Page.captureScreenshot", map[string]any{"format": "png"}, 15*time.Second)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	if payload.Data == "" {
		return nil, fmt.Errorf("empty screenshot")
	}
	return decodeBase64(payload.Data)
}

// Close tears the browser down.
func (d *DriverSession) Close() {
	if d == nil || d.Browser == nil {
		return
	}
	d.Browser.Close()
	d.Browser = nil
}

// WriteSessionFile records what the driver achieved so the next run can reuse it.
func (a *App) writeSessionFile(siteID, profile string, cookies int, status string, note string) {
	if _, err := SaveSession(a.sitesDir, siteID, profile, map[string]any{
		"cookies":     cookies,
		"http_status": 200,
		"note":        note,
		"status":      status,
	}); err != nil {
		a.logf("error", fmt.Sprintf("could not write session for %s: %v", siteID, err), "sessions", siteID)
	}
}

func decodeBase64(raw string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(raw)
}

func trimForLog(text string, limit int) string {
	clean := strings.Join(strings.Fields(text), " ")
	if len(clean) <= limit {
		return clean
	}
	return clean[:limit] + "…"
}

// TestSiteBrowser is the deep version of TestSite: it opens the site in a real
// browser, installs the site driver and reports what the DOM actually looks
// like. Slow (seconds) but the only honest "can I drive this?" answer.
func (a *App) TestSiteBrowser(siteID, profile string) (map[string]any, error) {
	site, ok := FindSite(a.sites, siteID)
	if !ok {
		return nil, fmt.Errorf("unknown site %q", siteID)
	}
	if profile == "" {
		profile = "default"
	}
	status := DiscoverBrowser(a.store.Get().Browser.Path)
	if !status.Found {
		return nil, fmt.Errorf("%s", status.Note)
	}

	a.logf("system", fmt.Sprintf("browser check started · %s · profile %s", status.Name, profile), "browser", siteID)

	driver, err := a.openDriver(site, profile, true, false)
	if err != nil {
		a.logf("error", "browser check failed: "+err.Error(), "browser", siteID)
		return nil, err
	}
	defer driver.Close()

	probe := driver.Probe()
	snapshot, snapErr := driver.Snapshot()
	if snapErr != nil {
		a.logf("error", "browser check: snapshot failed · "+snapErr.Error(), "dom", siteID)
		return nil, snapErr
	}

	shot, shotErr := driver.Screenshot()
	shotPath := ""
	if shotErr == nil && len(shot) > 0 {
		dir := filepath.Join(a.dataDir, "screenshots")
		if err := os.MkdirAll(dir, 0o755); err == nil {
			shotPath = filepath.Join(dir, siteID+"-"+time.Now().Format("20060102-150405")+".png")
			if err := os.WriteFile(shotPath, shot, 0o644); err != nil {
				shotPath = ""
			}
		}
	}

	out := map[string]any{
		"site_id":    site.ID,
		"ok":         snapshot.Controls.HasInput,
		"probe":      probe,
		"phase":      snapshot.Controls.Phase,
		"has_input":  snapshot.Controls.HasInput,
		"can_send":   snapshot.Controls.CanSend,
		"protection": snapshot.Controls.Protection,
		"messages":   len(snapshot.Messages),
		"revision":   snapshot.Revision,
		"browser":    status.Name,
		"screenshot": shotPath,
	}
	if snapshot.Controls.HasInput {
		a.logf("system", fmt.Sprintf("browser check OK · phase=%s · messages=%d · %s", snapshot.Controls.Phase, len(snapshot.Messages), probe), "dom", siteID)
	} else {
		a.logf("skip", "browser check: page loaded but no composer yet · "+probe, "dom", siteID)
	}
	return out, nil
}
