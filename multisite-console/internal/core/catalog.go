package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// LoadSites reads sites/<id>/site.json for every folder under sitesDir.
// A broken or missing site.json is skipped (and reported in the error slice)
// so one bad site can never take the dashboard down.
func LoadSites(sitesDir string) ([]SiteConfig, []error) {
	entries, err := os.ReadDir(sitesDir)
	if err != nil {
		return nil, []error{fmt.Errorf("read sites dir: %w", err)}
	}

	sites := make([]SiteConfig, 0, len(entries))
	var problems []error
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		id := entry.Name()
		if id == "template_new_site" || strings.HasPrefix(id, ".") {
			continue
		}
		path := filepath.Join(sitesDir, id, "site.json")
		raw, err := os.ReadFile(path)
		if err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", id, err))
			continue
		}
		var cfg SiteConfig
		if err := json.Unmarshal(raw, &cfg); err != nil {
			problems = append(problems, fmt.Errorf("%s: bad site.json: %w", id, err))
			continue
		}
		if cfg.ID == "" {
			cfg.ID = id
		}
		if cfg.ID != id {
			problems = append(problems, fmt.Errorf("%s: site.json id %q does not match folder", id, cfg.ID))
			cfg.ID = id
		}
		sites = append(sites, cfg)
	}

	sort.Slice(sites, func(i, j int) bool { return sites[i].ID < sites[j].ID })
	return sites, problems
}

// SiteIDs returns the ids in registry order (used for settings defaults).
func SiteIDs(sites []SiteConfig) []string {
	ids := make([]string, 0, len(sites))
	for _, site := range sites {
		ids = append(ids, site.ID)
	}
	return ids
}

// FindSite returns the config for one id.
func FindSite(sites []SiteConfig, id string) (SiteConfig, bool) {
	for _, site := range sites {
		if site.ID == id {
			return site, true
		}
	}
	return SiteConfig{}, false
}

// ReadProtectionNote returns the *_protection.txt text for a site (may be empty).
func ReadProtectionNote(sitesDir, id string) string {
	matches, err := filepath.Glob(filepath.Join(sitesDir, id, "*_protection.txt"))
	if err != nil || len(matches) == 0 {
		return ""
	}
	raw, err := os.ReadFile(matches[0])
	if err != nil {
		return ""
	}
	return string(raw)
}

// ReadDriver returns the per-site DOM driver script and its site.json text.
func ReadDriver(sitesDir, id string) (driver string, siteJSON string, err error) {
	driverRaw, err := os.ReadFile(filepath.Join(sitesDir, id, "dom_driver.js"))
	if err != nil {
		return "", "", fmt.Errorf("read dom_driver.js for %s: %w", id, err)
	}
	siteRaw, err := os.ReadFile(filepath.Join(sitesDir, id, "site.json"))
	if err != nil {
		return "", "", fmt.Errorf("read site.json for %s: %w", id, err)
	}
	return string(driverRaw), string(siteRaw), nil
}

// CountFiles returns how many regular files live in dir (0 when missing).
func CountFiles(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			count++
		}
	}
	return count
}

// DirHasFiles reports whether dir exists and holds at least one file.
func DirHasFiles(dir string) bool { return CountFiles(dir) > 0 }
