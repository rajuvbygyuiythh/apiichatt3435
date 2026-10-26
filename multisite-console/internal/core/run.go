package core

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// The runner owns everything that actually happens on a site.
//
// Two execution paths, one log vocabulary:
//   real  · a Chromium-family browser is driven over CDP with the site's own
//           dom_driver.js (step 4)
//   sim   · no browser on this machine: the same steps are logged at the same
//           pace so the dashboard, the pacing rules and the log feed stay usable
//
// The fallback is announced loudly (error entry + "simulation" meta) so nobody
// mistakes it for a working run.

func (a *App) runJob(ctx context.Context, mode string, siteIDs []string, profile string) {
	defer func() {
		a.closeAllDrivers()
		a.mu.Lock()
		a.running, a.runMode = false, ""
		a.mu.Unlock()
		a.logf("system", "run finished", "runner", "")
	}()

	settings := a.store.Get()
	defaultLines := splitLines(settings.Message.Sequence)
	if len(defaultLines) == 0 {
		defaultLines = []string{"hello there"}
	}

	browser := DiscoverBrowser(settings.Browser.Path)
	if browser.Found {
		a.logf("system", fmt.Sprintf("browser driver: %s · %s", browser.Name, fallback(browser.Version, "version unknown")), "browser", "")
	} else {
		a.logf("error", "no browser found — running in SIMULATION mode ("+browser.Note+")", "browser", "")
	}

	headless := settings.Browser.Headless
	if mode == "show" {
		headless = false // Show mode is the visible window, by definition
	}

	for _, siteID := range siteIDs {
		if ctx.Err() != nil {
			return
		}
		site, ok := FindSite(a.sites, siteID)
		if !ok {
			continue
		}
		if !site.Enabled {
			a.logf("skip", fmt.Sprintf("%s is disabled in site.json", site.Name), "registry", siteID)
			continue
		}
		if !a.sleep(ctx, 700*time.Millisecond) {
			return
		}

		lines := defaultLines
		if settings.Message.Strategy == "folder" {
			if fromFile := a.sequenceForSite(siteID); len(fromFile) > 0 {
				lines = fromFile
				a.logf("system", fmt.Sprintf("folder strategy: %d line(s) from data/sequences", len(fromFile)), "sequences", siteID)
			} else {
				a.logf("skip", "folder strategy but data/sequences is empty · using inline sequence", "sequences", siteID)
			}
		}

		a.logf("system", fmt.Sprintf("opening %s (%s · %s)", site.Name, site.Family, site.AccountMode), site.ID+"/site.json", siteID)
		a.logf("system", fmt.Sprintf("onboarding: %s", fallback(site.EntryFlow, "guest entry")), "auto-onboard", siteID)

		if browser.Found {
			a.runSiteReal(ctx, site, profile, settings, lines, headless)
		} else {
			a.runSiteSim(ctx, site, settings, lines, mode)
		}
	}
}

// runSiteReal drives one site with a real browser.
func (a *App) runSiteReal(ctx context.Context, site SiteConfig, profile string, settings Settings, lines []string, headless bool) {
	driver, err := a.openDriver(site, profile, headless, false)
	if err != nil {
		a.logf("error", fmt.Sprintf("%s: browser open failed · %v", site.Name, err), "browser", site.ID)
		a.mu.Lock()
		a.stats.Failed++
		a.mu.Unlock()
		return
	}
	a.registerDriver(site.ID, driver)
	defer func() {
		a.unregisterDriver(site.ID)
		driver.Close()
	}()

	snapshot, snapErr := driver.Snapshot()
	if snapErr != nil {
		a.logf("error", fmt.Sprintf("%s: driver snapshot failed · %v", site.Name, snapErr), "dom", site.ID)
		a.mu.Lock()
		a.stats.Failed++
		a.mu.Unlock()
		return
	}
	a.logf("system", fmt.Sprintf("driver ready · phase=%s · input=%s · protection=%s",
		snapshot.Controls.Phase, boolDetail(snapshot.Controls.HasInput, "yes", "no"), fallback(snapshot.Controls.Protection, "clean")), "dom", site.ID)

	if snapshot.Controls.Protection != "" && snapshot.Controls.Protection != "clean" {
		a.logf("skip", "protection detected: "+snapshot.Controls.Protection+" · rotating away", "protection", site.ID)
		a.mu.Lock()
		a.stats.Failed++
		a.mu.Unlock()
		return
	}

	seen := map[string]bool{}
	for i, line := range lines {
		if i >= settings.Message.MaxLines || ctx.Err() != nil {
			break
		}
		if err := driver.Send(line); err != nil {
			a.logf("error", fmt.Sprintf("send failed: %v", err), "composer", site.ID)
			if !a.sleep(ctx, a.pace(settings, "worker")) {
				return
			}
			continue
		}
		a.logf("out", line, fmt.Sprintf("line %d/%d · %s · real", i+1, len(lines), settings.Message.Strategy), site.ID)

		// wait for a partner reply, but never longer than the configured window
		window := time.Duration(clamp(settings.Show.First.ReplyMax, 2, 60)) * time.Second
		a.pumpMessages(ctx, driver, site, seen, window)
	}

	if ctx.Err() == nil {
		switch driver.Click("SKIP") {
		case "SKIP":
			a.logf("skip", "closing chat and rotating", "next target", site.ID)
		default:
			a.logf("skip", "no skip button · leaving the tab open for the next run", "next target", site.ID)
		}
		a.writeSessionFile(site.ID, profile, 1, "used", fmt.Sprintf("%d line(s) sent by the console", len(lines)))
	}

	a.mu.Lock()
	a.stats.Completed++
	a.mu.Unlock()
}

// pumpMessages reads the driver's message list until a partner message appears
// or the window runs out, logging every new inbound line exactly once.
func (a *App) pumpMessages(ctx context.Context, driver *DriverSession, site SiteConfig, seen map[string]bool, window time.Duration) {
	deadline := time.Now().Add(window)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return
		}
		snapshot, err := driver.Snapshot()
		if err != nil {
			time.Sleep(700 * time.Millisecond)
			continue
		}
		found := false
		for _, message := range snapshot.Messages {
			author := strings.ToLower(strings.TrimSpace(message.Author))
			if author == "self" || strings.TrimSpace(message.Text) == "" {
				continue
			}
			key := author + "|" + message.Text
			if seen[key] {
				continue
			}
			seen[key] = true
			found = true
			a.logf("in", message.Text, "partner reply · dom", site.ID)
		}
		if found {
			return
		}
		time.Sleep(900 * time.Millisecond)
	}
	a.logf("skip", "no reply inside the window", "timeout", site.ID)
}

// runSiteSim keeps the dashboard honest when no browser exists.
func (a *App) runSiteSim(ctx context.Context, site SiteConfig, settings Settings, lines []string, mode string) {
	if !a.sleep(ctx, a.pace(settings, mode)) {
		return
	}
	a.logf("system", "simulation: no browser on this machine — steps are logged, not executed", "simulation", site.ID)

	for i, line := range lines {
		if i >= settings.Message.MaxLines || ctx.Err() != nil {
			break
		}
		a.logf("out", line, fmt.Sprintf("line %d/%d · %s · simulated", i+1, len(lines), settings.Message.Strategy), site.ID)
		if !a.sleep(ctx, a.pace(settings, mode)) {
			return
		}
		if i == 0 {
			a.logf("in", "hey, how's it going?", "partner reply · simulated", site.ID)
		}
	}

	a.mu.Lock()
	a.stats.Completed++
	a.mu.Unlock()
	a.logf("skip", "closing chat and rotating", "next target", site.ID)
}

// --- driver registry (so Stop can kill browsers) ----------------------------

func (a *App) registerDriver(siteID string, driver *DriverSession) {
	a.mu.Lock()
	if a.drivers == nil {
		a.drivers = map[string]*DriverSession{}
	}
	a.drivers[siteID] = driver
	a.mu.Unlock()
}

func (a *App) unregisterDriver(siteID string) {
	a.mu.Lock()
	delete(a.drivers, siteID)
	a.mu.Unlock()
}

func (a *App) closeAllDrivers() {
	a.mu.Lock()
	drivers := a.drivers
	a.drivers = map[string]*DriverSession{}
	a.mu.Unlock()
	for _, driver := range drivers {
		driver.Close()
	}
}
