package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Session storage lives inside each site folder, never in a global table:
//
//	sites/<id>/profiles/<profile>/session.json
//
// The browser driver fills in cookies/local storage in step 4; the console can
// already create, list and delete sessions, which is what the Accounts card needs.

const sessionFileName = "session.json"

func profileDir(sitesDir, siteID, profile string) string {
	if strings.TrimSpace(profile) == "" {
		profile = "default"
	}
	return filepath.Join(sitesDir, siteID, "profiles", profile)
}

func sessionPath(sitesDir, siteID, profile string) string {
	return filepath.Join(profileDir(sitesDir, siteID, profile), sessionFileName)
}

// ListSessions reads every profile folder for one site.
func ListSessions(sitesDir, siteID string) ([]SiteSession, error) {
	profilesDir := filepath.Join(sitesDir, siteID, "profiles")
	entries, err := os.ReadDir(profilesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []SiteSession{}, nil
		}
		return nil, fmt.Errorf("read profiles for %s: %w", siteID, err)
	}

	sessions := make([]SiteSession, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		profile := entry.Name()
		path := sessionPath(sitesDir, siteID, profile)
		session := SiteSession{SiteID: siteID, Profile: profile, FilePath: path, Note: "profile folder (no session.json yet)"}
		raw, readErr := os.ReadFile(path)
		if readErr == nil {
			var stored SiteSession
			if jsonErr := json.Unmarshal(raw, &stored); jsonErr == nil {
				session = stored
				session.SiteID = siteID
				session.Profile = profile
				session.FilePath = path
			} else {
				session.Note = "session.json is not valid JSON"
			}
		}
		if session.CreatedAt.IsZero() {
			if info, statErr := entry.Info(); statErr == nil {
				session.CreatedAt = info.ModTime()
				session.UpdatedAt = info.ModTime()
			}
		}
		sessions = append(sessions, session)
	}
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].Profile < sessions[j].Profile })
	return sessions, nil
}

// SaveSession writes (or updates) one profile's session.json.
func SaveSession(sitesDir, siteID, profile string, payload map[string]any) (SiteSession, error) {
	if strings.TrimSpace(profile) == "" {
		profile = "default"
	}
	dir := profileDir(sitesDir, siteID, profile)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return SiteSession{}, fmt.Errorf("create profile dir: %w", err)
	}

	session := SiteSession{
		SiteID:    siteID,
		Profile:   profile,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Status:    "",
		FilePath:  filepath.Join(dir, sessionFileName),
	}
	if payload != nil {
		if cookies, ok := payload["cookies"]; ok {
			session.Cookies = countAny(cookies)
		}
		if storage, ok := payload["local_storage"]; ok {
			session.LocalStorage = countAny(storage)
		}
		if status, ok := payload["http_status"]; ok {
			if value, ok := status.(float64); ok {
				session.HTTPStatus = int(value)
			}
		}
		if note, ok := payload["note"].(string); ok {
			session.Note = note
		}
	}

	// keep the original creation time if a session already exists
	if raw, err := os.ReadFile(session.FilePath); err == nil {
		var existing SiteSession
		if jsonErr := json.Unmarshal(raw, &existing); jsonErr == nil && !existing.CreatedAt.IsZero() {
			session.CreatedAt = existing.CreatedAt
		}
	}

	encoded, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return session, fmt.Errorf("encode session: %w", err)
	}
	if err := os.WriteFile(session.FilePath, append(encoded, '\n'), 0o644); err != nil {
		return session, fmt.Errorf("write session: %w", err)
	}
	return session, nil
}

// DeleteSession removes one profile folder (session included).
func DeleteSession(sitesDir, siteID, profile string) error {
	if strings.TrimSpace(profile) == "" {
		return fmt.Errorf("profile is required")
	}
	dir := profileDir(sitesDir, siteID, profile)
	if filepath.Clean(dir) == filepath.Clean(filepath.Join(sitesDir, siteID, "profiles")) {
		return fmt.Errorf("refusing to delete the whole profiles folder")
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("delete session %s/%s: %w", siteID, profile, err)
	}
	return nil
}

func countAny(value any) int {
	switch typed := value.(type) {
	case []any:
		return len(typed)
	case map[string]any:
		return len(typed)
	case float64:
		return int(typed)
	case bool:
		if typed {
			return 1
		}
		return 0
	case string:
		if strings.TrimSpace(typed) == "" {
			return 0
		}
		return len(strings.Split(typed, ";"))
	}
	return 0
}
