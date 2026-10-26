package core

import (
	"sync"
	"time"
)

// LogRing is an append-only, version-cursored log buffer.
//
// The UI polls Since(after) once per second and appends; in the Wails build the
// same entries are also pushed as ui:log events. Cursor = last entry id.
type LogRing struct {
	mu      sync.Mutex
	entries []LogEntry
	seq     int
	limit   int
	cleared bool
}

// NewLogRing creates a ring that keeps the newest limit entries.
func NewLogRing(limit int) *LogRing {
	if limit <= 0 {
		limit = 2000
	}
	return &LogRing{limit: limit}
}

// Append adds a line and returns it.
func (l *LogRing) Append(kind, text, meta, site string) LogEntry {
	l.mu.Lock()
	l.seq++
	entry := LogEntry{
		ID:   l.seq,
		Time: time.Now().Format("15:04:05"),
		Kind: kind,
		Text: text,
		Meta: meta,
		Site: site,
	}
	l.entries = append(l.entries, entry)
	if len(l.entries) > l.limit {
		l.entries = l.entries[len(l.entries)-l.limit:]
	}
	l.cleared = false
	l.mu.Unlock()
	return entry
}

// Since returns every entry newer than the cursor.
func (l *LogRing) Since(after int) (int, []LogEntry, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if after > l.seq {
		// the UI holds a cursor from before a Clear
		return l.seq, nil, true
	}
	out := make([]LogEntry, 0, len(l.entries))
	for _, entry := range l.entries {
		if entry.ID > after {
			out = append(out, entry)
		}
	}
	return l.seq, out, l.cleared
}

// Clear empties the ring and bumps the version so old cursors are invalid.
func (l *LogRing) Clear() {
	l.mu.Lock()
	l.entries = nil
	l.seq++
	l.cleared = true
	l.mu.Unlock()
}

// Len reports how many entries are buffered (tests use it).
func (l *LogRing) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.entries)
}
