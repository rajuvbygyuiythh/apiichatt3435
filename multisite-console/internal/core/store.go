package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Store persists Settings as one JSON document (data/settings.json).
//
// Why a file and not a database: the console's state is a single small struct,
// and a plain file makes the data folder inspectable, diffable and testable.
// The original project used BadgerDB; the settings contract is unchanged, only
// the storage call site differs (see design-kit/07-BACKEND-PATTERNS.md §4).
type Store struct {
	mu       sync.Mutex
	path     string
	settings Settings
	siteIDs  []string
}

// OpenStore loads (or seeds) data/settings.json.
func OpenStore(dataDir string, siteIDs []string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	s := &Store{
		path:     filepath.Join(dataDir, "settings.json"),
		settings: Defaults(siteIDs),
		siteIDs:  siteIDs,
	}

	raw, err := os.ReadFile(s.path)
	switch {
	case err == nil:
		var loaded Settings
		if jsonErr := json.Unmarshal(raw, &loaded); jsonErr != nil {
			// keep defaults, but never crash on a corrupt file: rename it aside
			_ = os.Rename(s.path, s.path+".corrupt")
			_ = s.persist()
		} else {
			s.settings = loaded
			s.settings.Normalise(siteIDs)
		}
	case os.IsNotExist(err):
		if err := s.persist(); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("read settings: %w", err)
	}

	return s, nil
}

// Path is where settings live (shown in the UI).
func (s *Store) Path() string { return s.path }

// Get returns a copy of the current settings.
func (s *Store) Get() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings
}

// Apply merges one section from raw JSON (the UI sends {"section":…,"values":…}),
// normalises, persists, and returns the new settings.
func (s *Store) Apply(section string, values json.RawMessage) (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	candidate := s.settings
	var target any
	switch section {
	case "show":
		target = &candidate.Show
	case "worker":
		target = &candidate.Worker
	case "message":
		target = &candidate.Message
	case "proxy":
		target = &candidate.Proxy
	case "accounts":
		target = &candidate.Accounts
	case "browser":
		target = &candidate.Browser
	default:
		return s.settings, fmt.Errorf("unknown settings section %q", section)
	}

	if len(values) > 0 {
		if err := json.Unmarshal(values, target); err != nil {
			return s.settings, fmt.Errorf("bad values for %q: %w", section, err)
		}
	}
	candidate.Normalise(s.siteIDs)
	s.settings = candidate
	if err := s.persist(); err != nil {
		return s.settings, err
	}
	return s.settings, nil
}

// Save replaces all settings at once (used by "Save all" style buttons/tests).
func (s *Store) Save(next Settings) (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next.Normalise(s.siteIDs)
	s.settings = next
	if err := s.persist(); err != nil {
		return s.settings, err
	}
	return s.settings, nil
}

// persist writes atomically (temp file + rename) so a crash cannot truncate it.
func (s *Store) persist() error {
	raw, err := json.MarshalIndent(s.settings, "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, append(raw, '\n'), 0o644); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("replace settings: %w", err)
	}
	return nil
}
