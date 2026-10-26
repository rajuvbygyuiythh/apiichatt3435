package core

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SelectorCoverage counts how many of the 11 selectors a site actually defines.
// A family bundle with 4-5 filled selectors is normal; 0 means the driver has
// nothing to work with and the site cannot be driven reliably.
func SelectorCoverage(sel Selectors) (filled int, empty []string) {
	pairs := []struct {
		name  string
		value string
	}{
		{"composer_input", sel.ComposerInput},
		{"send_button", sel.SendButton},
		{"skip_button", sel.SkipButton},
		{"start_button", sel.StartButton},
		{"message_history", sel.MessageHistory},
		{"message_row", sel.MessageRow},
		{"message_text", sel.MessageText},
		{"partner_message_selector", sel.PartnerMessageSelector},
		{"own_message_selector", sel.OwnMessageSelector},
		{"partner_name", sel.PartnerName},
		{"chat_ended_text", sel.ChatEndedText},
	}
	empty = make([]string, 0, len(pairs))
	for _, pair := range pairs {
		if strings.TrimSpace(pair.value) == "" {
			empty = append(empty, pair.name)
			continue
		}
		filled++
	}
	return filled, empty
}

// ListSequenceFiles returns the .txt scripts available for the folder strategy.
func ListSequenceFiles(dataDir string) ([]string, error) {
	dir := filepath.Join(dataDir, "sequences")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("read sequences: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".txt") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names, nil
}

// ReadSequences loads every script with its lines (empty lines dropped).
func ReadSequences(dataDir string) ([]SequenceFile, error) {
	names, err := ListSequenceFiles(dataDir)
	if err != nil {
		return nil, err
	}
	out := make([]SequenceFile, 0, len(names))
	for _, name := range names {
		raw, readErr := os.ReadFile(filepath.Join(dataDir, "sequences", name))
		if readErr != nil {
			continue
		}
		out = append(out, SequenceFile{Name: name, Lines: splitLines(string(raw))})
	}
	return out, nil
}

// buildDiagnostics assembles the deep health view for one site.
func (a *App) buildDiagnostics(site SiteConfig) SiteDiagnostics {
	base := filepath.Join(a.sitesDir, site.ID)
	driverBytes := len(readFileOrEmpty(filepath.Join(base, "dom_driver.js")))
	filled, empty := SelectorCoverage(site.Selectors)
	accounts := CountFiles(filepath.Join(base, "accounts"))
	sessions, _ := ListSessions(a.sitesDir, site.ID)
	protection := ReadProtectionNote(a.sitesDir, site.ID)
	sequences, _ := ListSequenceFiles(a.dataDir)

	diag := SiteDiagnostics{
		SiteID:         site.ID,
		Name:           site.Name,
		Family:         site.Family,
		Enabled:        site.Enabled,
		DriverBytes:    driverBytes,
		SiteJSONValid:  true,
		SelectorTotal:  11,
		SelectorFilled: filled,
		EmptySelectors: empty,
		Selectors:      site.Selectors,
		ProtectionNote: protection,
		Accounts:       accounts,
		Sessions:       len(sessions),
		SequenceFiles:  sequences,
		LatencyHistory: a.latencyHistory(site.ID),
		LastProbe:      a.lastProbe(site.ID),
	}

	parseURL, urlErr := url.Parse(strings.TrimSpace(site.StartURL))
	httpsOK := urlErr == nil && parseURL.Scheme == "https" && parseURL.Host != ""

	diag.Checks = []DiagCheck{
		{Name: "site.json parsed", OK: true, Detail: fmt.Sprintf("id=%s family=%s mode=%s", site.ID, site.Family, site.AccountMode)},
		{Name: "site enabled", OK: site.Enabled, Detail: boolDetail(site.Enabled, "enabled in site.json", "disabled in site.json")},
		{Name: "start URL", OK: httpsOK, Detail: fallback(site.StartURL, "(missing start_url)")},
		{Name: "dom_driver.js", OK: driverBytes > 500, Detail: fmt.Sprintf("%d bytes", driverBytes)},
		{Name: "selector coverage", OK: filled >= 6, Detail: fmt.Sprintf("%d/11 filled", filled)},
		{Name: "protection note", OK: strings.TrimSpace(protection) != "", Detail: boolDetail(strings.TrimSpace(protection) != "", "present", "missing <id>_protection.txt")},
		{Name: "accounts or session", OK: accounts > 0 || len(sessions) > 0, Detail: fmt.Sprintf("%d accounts · %d sessions", accounts, len(sessions))},
		{Name: "sequence scripts", OK: len(sequences) > 0, Detail: fmt.Sprintf("%d file(s) in data/sequences", len(sequences))},
	}

	if diag.LastProbe != nil {
		diag.Checks = append(diag.Checks, DiagCheck{
			Name:   "last probe",
			OK:     diag.LastProbe.OK,
			Detail: fmt.Sprintf("%s · %d ms", fallback(diag.LastProbe.Message, "no message"), diag.LastProbe.LatencyMS),
		})
	}

	ready := true
	for _, check := range diag.Checks {
		if !check.OK {
			ready = false
		}
	}
	diag.Ready = ready
	return diag
}

func (a *App) lastProbe(siteID string) *TestResult {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.lastTests == nil {
		return nil
	}
	result, ok := a.lastTests[siteID]
	if !ok || result.Entry == "" {
		return nil
	}
	copy := result
	return &copy
}

func (a *App) latencyHistory(siteID string) []int {
	a.mu.Lock()
	defer a.mu.Unlock()
	history := a.latencies[siteID]
	out := make([]int, len(history))
	copy(out, history)
	return out
}

func (a *App) rememberLatency(siteID string, latencyMS int) {
	if latencyMS < 0 {
		return
	}
	if latencyMS == 0 {
		// a sub-millisecond local probe still counts; keep the history honest
		latencyMS = 1
	}
	a.mu.Lock()
	if a.latencies == nil {
		a.latencies = map[string][]int{}
	}
	history := append(a.latencies[siteID], latencyMS)
	if len(history) > 10 {
		history = history[len(history)-10:]
	}
	a.latencies[siteID] = history
	a.mu.Unlock()
}

func boolDetail(ok bool, whenTrue, whenFalse string) string {
	if ok {
		return whenTrue
	}
	return whenFalse
}

// GetSiteDiagnostics is the API method behind the diagnostics panel.
func (a *App) GetSiteDiagnostics(siteID string) (SiteDiagnostics, error) {
	site, ok := FindSite(a.sites, siteID)
	if !ok {
		return SiteDiagnostics{}, fmt.Errorf("unknown site %q", siteID)
	}
	return a.buildDiagnostics(site), nil
}

// AllDiagnostics returns every site's diagnostics in catalog order.
func (a *App) AllDiagnostics() []SiteDiagnostics {
	out := make([]SiteDiagnostics, 0, len(a.sites))
	for _, site := range a.sites {
		out = append(out, a.buildDiagnostics(site))
	}
	return out
}

// GetSessions lists the saved profiles of one site.
func (a *App) GetSessions(siteID string) ([]SiteSession, error) {
	if _, ok := FindSite(a.sites, siteID); !ok {
		return nil, fmt.Errorf("unknown site %q", siteID)
	}
	sessions, err := ListSessions(a.sitesDir, siteID)
	if err != nil {
		return nil, err
	}
	a.logf("system", fmt.Sprintf("%s: %d saved profile(s)", siteID, len(sessions)), "sessions", siteID)
	return sessions, nil
}

// SaveSession stores a (possibly empty) session marker for a profile.
func (a *App) SaveSession(siteID, profile string, payload map[string]any) (SiteSession, error) {
	if _, ok := FindSite(a.sites, siteID); !ok {
		return SiteSession{}, fmt.Errorf("unknown site %q", siteID)
	}
	session, err := SaveSession(a.sitesDir, siteID, profile, payload)
	if err != nil {
		return session, err
	}
	a.logf("system", fmt.Sprintf("%s: session saved for profile %q", siteID, session.Profile), "sessions", siteID)
	return session, nil
}

// DeleteSession removes one profile folder.
func (a *App) DeleteSession(siteID, profile string) (map[string]any, error) {
	if _, ok := FindSite(a.sites, siteID); !ok {
		return nil, fmt.Errorf("unknown site %q", siteID)
	}
	if err := DeleteSession(a.sitesDir, siteID, profile); err != nil {
		return nil, err
	}
	a.logf("system", fmt.Sprintf("%s: session for %q deleted", siteID, profile), "sessions", siteID)
	return map[string]any{"ok": true, "site_id": siteID, "profile": profile}, nil
}

func profileDirs(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	return names
}

func sequenceFileNames(dataDir string) []string {
	names, err := ListSequenceFiles(dataDir)
	if err != nil {
		return nil
	}
	return names
}
