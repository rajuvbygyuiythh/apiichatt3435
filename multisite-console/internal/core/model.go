// Package core is the whole application layer of the Multi-Site Console:
// site registry, settings, log ring, job runner and the JSON API.
//
// It is deliberately standard-library only so it can be compiled, vetted and
// tested anywhere (no module downloads), and so the same code can be served
// over HTTP in a browser or bound to Wails for the desktop build.
package core

import "time"

// Version is shown in the sidebar tooltip and in the API snapshot.
const Version = "0.2.0-scaffold"

// Selectors is the per-site CSS bundle used by the DOM driver.
// Field names match sites/<id>/site.json exactly.
type Selectors struct {
	ComposerInput          string `json:"composer_input"`
	SendButton             string `json:"send_button"`
	SkipButton             string `json:"skip_button"`
	StartButton            string `json:"start_button"`
	MessageHistory         string `json:"message_history"`
	MessageRow             string `json:"message_row"`
	MessageText            string `json:"message_text"`
	PartnerMessageSelector string `json:"partner_message_selector"`
	OwnMessageSelector     string `json:"own_message_selector"`
	PartnerName            string `json:"partner_name"`
	ChatEndedText          string `json:"chat_ended_text"`
}

// SiteConfig is one isolated site folder (sites/<id>/site.json).
type SiteConfig struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Family            string    `json:"family"`
	AccountMode       string    `json:"account_mode"`
	Enabled           bool      `json:"enabled"`
	StartURL          string    `json:"start_url"`
	AppURL            string    `json:"app_url"`
	AuthMode          string    `json:"auth_mode"`
	EntryFlow         string    `json:"entry_flow"`
	ProtectionSummary string    `json:"protection_summary"`
	Selectors         Selectors `json:"selectors"`
}

// SiteState is what the dashboard renders: config plus live facts on disk.
type SiteState struct {
	SiteConfig
	Accounts    int    `json:"accounts"`
	HasSession  bool   `json:"has_session"`
	DriverBytes int    `json:"driver_bytes"`
	Status      string `json:"status"`      // ready | no-driver | no-session | disabled
	LastTest    string `json:"last_test"`   // "12:04:31 · 200 · 412 ms" or "never"
	HTTPStatus  int    `json:"http_status"` // last TestSite result
	LatencyMS   int    `json:"latency_ms"`  // last TestSite result
}

// FirstMessageSettings mirrors the original dashboard's onboarding policy.
type FirstMessageSettings struct {
	Mode       string `json:"mode"`        // bot | user | random
	Chance     int    `json:"chance"`      // 0..100, used by random
	WaitMode   string `json:"wait_mode"`   // unlimited | skip
	TimeoutMin int    `json:"timeout_min"` // seconds
	TimeoutMax int    `json:"timeout_max"`
	ReplyMin   int    `json:"reply_min"`
	ReplyMax   int    `json:"reply_max"`
	NoReplyMin int    `json:"no_reply_min"`
	NoReplyMax int    `json:"no_reply_max"`
	AutoSkip   bool   `json:"auto_skip"`
}

// ShowSettings is Mode A: one visible browser, one site at a time.
type ShowSettings struct {
	SiteID  string               `json:"site_id"`
	Profile string               `json:"profile"`
	First   FirstMessageSettings `json:"first"`
}

// WorkerSettings is Mode B: headless workers across several sites.
type WorkerSettings struct {
	Sites       []string `json:"sites"`
	Profile     string   `json:"profile"`
	Headless    bool     `json:"headless"`
	Concurrency int      `json:"concurrency"`
	PollMin     int      `json:"poll_min"`
	PollMax     int      `json:"poll_max"`
	ActionMin   int      `json:"action_min"`
	ActionMax   int      `json:"action_max"`
	TimeoutMin  int      `json:"timeout_min"`
	TimeoutMax  int      `json:"timeout_max"`
	RetryMin    int      `json:"retry_min"`
	RetryMax    int      `json:"retry_max"`
}

// MessageSettings is shared by both modes (same store, like the original).
type MessageSettings struct {
	Strategy        string   `json:"strategy"` // single | folder | model
	Sequence        string   `json:"sequence"`
	Targets         string   `json:"targets"`
	LoadedSequences []string `json:"loaded_sequences,omitempty"`
	MaxLines        int      `json:"max_lines"`
}

// ProxySettings is shared by both modes.
type ProxySettings struct {
	Enabled bool   `json:"enabled"`
	Scheme  string `json:"scheme"`
	Entries string `json:"entries"`
}

// AccountSettings drives the auto-register flow.
type AccountSettings struct {
	Count        int    `json:"count"`
	Gender       string `json:"gender"`
	Age          int    `json:"age"`
	NicknamePool string `json:"nickname_pool"`
}

// Settings is everything persisted in data/settings.json.
type Settings struct {
	Show     ShowSettings    `json:"show"`
	Worker   WorkerSettings  `json:"worker"`
	Message  MessageSettings `json:"message"`
	Proxy    ProxySettings   `json:"proxy"`
	Accounts AccountSettings `json:"accounts"`
}

// Stats is the capsule row.
type Stats struct {
	Sites     int `json:"sites"`
	Enabled   int `json:"enabled"`
	Accounts  int `json:"accounts"`
	Sessions  int `json:"sessions"`
	Running   int `json:"running"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
}

// Runtime describes the process for the sidebar pill.
type Runtime struct {
	Running   bool   `json:"running"`
	Mode      string `json:"mode"` // show | worker | ""
	Kind      string `json:"kind"` // wails | http
	DataDir   string `json:"data_dir"`
	Version   string `json:"version"`
	StartedAt string `json:"started_at"`
	Sites     int    `json:"sites"`
}

// Snapshot is the single object the dashboard loads with.
type Snapshot struct {
	Settings Settings    `json:"settings"`
	Strategy string      `json:"strategy"`
	Sites    []SiteState `json:"sites"`
	Stats    Stats       `json:"stats"`
	Runtime  Runtime     `json:"runtime"`
}

// LogEntry is one row of the live feed.
type LogEntry struct {
	ID   int    `json:"id"`
	Time string `json:"time"`
	Kind string `json:"kind"` // in | out | skip | system | error
	Text string `json:"text"`
	Meta string `json:"meta,omitempty"`
	Site string `json:"site,omitempty"`
}

// LogPage is the versioned poll payload.
type LogPage struct {
	Version int        `json:"version"`
	Entries []LogEntry `json:"entries"`
	Cleared bool       `json:"cleared"`
	Running bool       `json:"running"`
	Stats   Stats      `json:"stats"`
}

// TestResult is one per-entry verdict (connections) or a single site probe.
type TestResult struct {
	Entry     string `json:"entry"`
	OK        bool   `json:"ok"`
	LatencyMS int    `json:"latency_ms"`
	Message   string `json:"message"`
	HTTP      int    `json:"http,omitempty"`
}

// Account is one auto-registered identity stored in sites/<id>/accounts/.
type Account struct {
	ID        string    `json:"id"`
	SiteID    string    `json:"site_id"`
	SiteName  string    `json:"site_name"`
	Nickname  string    `json:"nickname"`
	Gender    string    `json:"gender"`
	Age       int       `json:"age"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	FilePath  string    `json:"file_path"`
}

// Defaults returns the settings a fresh install starts with.
func Defaults(siteIDs []string) Settings {
	s := Settings{
		Show: ShowSettings{
			Profile: "default",
			First: FirstMessageSettings{
				Mode: "bot", Chance: 70, WaitMode: "unlimited",
				TimeoutMin: 30, TimeoutMax: 60,
				ReplyMin: 4, ReplyMax: 9,
				NoReplyMin: 60, NoReplyMax: 90,
				AutoSkip: true,
			},
		},
		Worker: WorkerSettings{
			Profile: "default", Headless: true, Concurrency: 1,
			PollMin: 5, PollMax: 12,
			ActionMin: 3, ActionMax: 8,
			TimeoutMin: 60, TimeoutMax: 90,
			RetryMin: 10, RetryMax: 30,
			Sites: append([]string{}, siteIDs...),
		},
		Message: MessageSettings{Strategy: "single", MaxLines: 3, Sequence: "hey there\nwhat's up?", Targets: ""},
		Proxy:   ProxySettings{Enabled: false, Scheme: "http", Entries: ""},
		Accounts: AccountSettings{Count: 2, Gender: "female", Age: 21,
			NicknamePool: "Sophia,Emma,Olivia,Mia,Chloe,Ava,Lily,Zoe,Hannah,Maya"},
	}
	if len(siteIDs) > 0 {
		s.Show.SiteID = siteIDs[0]
	}
	return s
}

// Normalise repairs enum-ish fields and clamps ranges after any load or save.
func (s *Settings) Normalise(siteIDs []string) {
	switch s.Show.First.Mode {
	case "bot", "user", "random":
	default:
		s.Show.First.Mode = "bot"
	}
	if s.Show.First.WaitMode != "skip" {
		s.Show.First.WaitMode = "unlimited"
	}
	s.Show.First.Chance = clamp(s.Show.First.Chance, 0, 100)
	s.Show.First.TimeoutMin = clamp(s.Show.First.TimeoutMin, 1, 86400)
	s.Show.First.TimeoutMax = clampMax(s.Show.First.TimeoutMax, s.Show.First.TimeoutMin, 86400)

	switch s.Message.Strategy {
	case "single", "folder", "model":
	default:
		s.Message.Strategy = "single"
	}
	s.Message.MaxLines = clamp(s.Message.MaxLines, 1, 20)

	if s.Proxy.Scheme != "socks5" {
		s.Proxy.Scheme = "http"
	}

	switch s.Accounts.Gender {
	case "female", "male", "random":
	default:
		s.Accounts.Gender = "female"
	}
	s.Accounts.Count = clamp(s.Accounts.Count, 1, 20)
	s.Accounts.Age = clamp(s.Accounts.Age, 18, 99)

	s.Worker.Concurrency = clamp(s.Worker.Concurrency, 1, 20)
	s.Worker.PollMin = clamp(s.Worker.PollMin, 1, 86400)
	s.Worker.PollMax = clampMax(s.Worker.PollMax, s.Worker.PollMin, 86400)
	s.Worker.ActionMin = clamp(s.Worker.ActionMin, 1, 86400)
	s.Worker.ActionMax = clampMax(s.Worker.ActionMax, s.Worker.ActionMin, 86400)
	s.Worker.TimeoutMin = clamp(s.Worker.TimeoutMin, 1, 86400)
	s.Worker.TimeoutMax = clampMax(s.Worker.TimeoutMax, s.Worker.TimeoutMin, 86400)
	s.Worker.RetryMin = clamp(s.Worker.RetryMin, 0, 86400)
	s.Worker.RetryMax = clampMax(s.Worker.RetryMax, s.Worker.RetryMin, 86400)

	// keep the site list honest: drop unknown ids, keep registry order
	if siteIDs != nil {
		known := map[string]bool{}
		for _, id := range siteIDs {
			known[id] = true
		}
		seen := map[string]bool{}
		clean := make([]string, 0, len(s.Worker.Sites))
		for _, id := range s.Worker.Sites {
			if known[id] && !seen[id] {
				seen[id] = true
				clean = append(clean, id)
			}
		}
		s.Worker.Sites = clean
		if s.Show.SiteID == "" || !known[s.Show.SiteID] {
			s.Show.SiteID = siteIDs[0]
		}
	}
	if s.Show.Profile == "" {
		s.Show.Profile = "default"
	}
	if s.Worker.Profile == "" {
		s.Worker.Profile = "default"
	}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampMax(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	return clamp(v, lo, hi)
}
