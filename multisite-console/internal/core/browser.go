package core

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// BrowserStatus is what the dashboard shows about the local browser: whether the
// console can actually drive something, and where that binary lives.
type BrowserStatus struct {
	Found   bool     `json:"found"`
	Path    string   `json:"path"`
	Name    string   `json:"name"`
	Source  string   `json:"source"` // env | settings | path | common-dir | missing
	Version string   `json:"version,omitempty"`
	Cands   []string `json:"candidates,omitempty"`
	Note    string   `json:"note,omitempty"`
}

// browserCandidates lists the usual Chrome/Edge/Chromium locations per OS.
func browserCandidates(extra string) []string {
	var out []string
	add := func(paths ...string) {
		for _, path := range paths {
			if strings.TrimSpace(path) != "" {
				out = append(out, path)
			}
		}
	}

	if extra != "" {
		add(extra)
	}
	if env := strings.TrimSpace(os.Getenv("MULTISITE_BROWSER")); env != "" {
		add(env)
	}

	switch runtime.GOOS {
	case "windows":
		local := os.Getenv("LOCALAPPDATA")
		programFiles := os.Getenv("ProgramFiles")
		programFilesX86 := os.Getenv("ProgramFiles(x86)")
		add(
			filepath.Join(programFiles, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(programFilesX86, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(local, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(programFiles, "Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join(programFilesX86, "Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join(programFiles, "BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
		)
	case "darwin":
		add(
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
		)
	default:
		add(
			"/usr/bin/google-chrome",
			"/usr/bin/google-chrome-stable",
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
			"/usr/bin/microsoft-edge",
			"/snap/bin/chromium",
			"/usr/bin/brave-browser",
		)
	}

	for _, name := range []string{"chrome", "google-chrome", "chromium", "chromium-browser", "msedge", "brave"} {
		if path, err := exec.LookPath(name); err == nil {
			add(path)
		}
	}
	return out
}

// DiscoverBrowser finds a usable Chromium-family binary. Never fails: the status
// carries Found=false and a human note instead.
func DiscoverBrowser(settingsPath string) BrowserStatus {
	status := BrowserStatus{Source: "missing"}

	candidates := browserCandidates(settingsPath)
	status.Cands = candidates

	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}
		status.Found = true
		status.Path = candidate
		status.Name = browserName(candidate)
		switch {
		case strings.TrimSpace(settingsPath) != "" && samePath(candidate, settingsPath):
			status.Source = "settings"
		case samePath(candidate, os.Getenv("MULTISITE_BROWSER")):
			status.Source = "env"
		default:
			status.Source = "common-dir"
		}
		break
	}

	if !status.Found {
		status.Note = "No Chrome/Edge/Chromium found. Install one, or set the browser path in Settings."
		return status
	}

	status.Version = browserVersion(status.Path)
	if status.Version == "" {
		status.Note = "Binary found; version could not be read (headless probing may still work)."
	}
	return status
}

func browserName(path string) string {
	lower := strings.ToLower(path)
	switch {
	case strings.Contains(lower, "msedge") || strings.Contains(lower, "edge"):
		return "Microsoft Edge"
	case strings.Contains(lower, "brave"):
		return "Brave"
	case strings.Contains(lower, "chromium"):
		return "Chromium"
	case strings.Contains(lower, "chrome"):
		return "Google Chrome"
	}
	return filepath.Base(path)
}

func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

// browserVersion asks the binary for its version (fast, and works headless).
func browserVersion(path string) string {
	cmd := exec.Command(path, "--version")
	done := make(chan string, 1)
	go func() {
		out, err := cmd.CombinedOutput()
		if err != nil {
			done <- ""
			return
		}
		done <- strings.TrimSpace(string(out))
	}()
	select {
	case version := <-done:
		if len(version) > 120 {
			version = version[:120]
		}
		return version
	case <-time.After(4 * time.Second):
		_ = cmd.Process.Kill()
		return ""
	}
}

// LaunchOptions describes one browser instance.
type LaunchOptions struct {
	ExecPath   string
	ProfileDir string
	StartURL   string
	Headless   bool
	KeepAlive  bool
	ExtraArgs  []string
	Timeout    time.Duration
}

// browserProcess is one running Chromium-family process plus its page client.
type browserProcess struct {
	cmd        *exec.Cmd
	profileDir string
	headless   bool
	port       int
	cdp        *cdpClient
	pageURL    string
	started    time.Time
}

// browserStarter is a seam for tests: the default really spawns a process.
var browserStarter = defaultBrowserStarter

func defaultBrowserStarter(opts LaunchOptions, args []string) (*exec.Cmd, error) {
	cmd := exec.Command(opts.ExecPath, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start browser: %w", err)
	}
	return cmd, nil
}

// LaunchBrowser starts Chrome/Edge against a dedicated profile folder and, when
// attach is true, connects to the first page target over CDP.
func LaunchBrowser(opts LaunchOptions, attach bool) (*browserProcess, error) {
	if opts.ExecPath == "" {
		return nil, fmt.Errorf("no browser binary configured")
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 25 * time.Second
	}
	if opts.ProfileDir == "" {
		opts.ProfileDir = filepath.Join(os.TempDir(), "multisite-console-profile")
	}
	if err := os.MkdirAll(opts.ProfileDir, 0o755); err != nil {
		return nil, fmt.Errorf("create profile dir: %w", err)
	}

	port, err := freePort()
	if err != nil {
		return nil, err
	}

	args := []string{
		fmt.Sprintf("--remote-debugging-port=%d", port),
		"--user-data-dir=" + opts.ProfileDir,
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-blink-features=AutomationControlled",
		"--disable-features=Translate,BackForwardCache,AcceptCHFrame",
		"--disable-popup-blocking",
		"--disable-notifications",
		"--disable-infobars",
		"--window-size=1280,900",
	}
	if opts.Headless {
		args = append(args, "--headless=new", "--disable-gpu", "--mute-audio")
	}
	args = append(args, opts.ExtraArgs...)
	if opts.StartURL != "" {
		args = append(args, opts.StartURL)
	} else {
		args = append(args, "about:blank")
	}

	cmd, err := browserStarter(opts, args)
	if err != nil {
		return nil, err
	}

	proc := &browserProcess{cmd: cmd, profileDir: opts.ProfileDir, headless: opts.Headless, port: port, started: time.Now()}

	cleanup := func(err error) (*browserProcess, error) {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return nil, err
	}

	deadline := time.Now().Add(opts.Timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		targets, err := pageTargets(port)
		if err == nil && len(targets) > 0 {
			proc.pageURL = targets[0].URL
			if !attach {
				return proc, nil
			}
			client, dialErr := dialCDP(targets[0].WebSocketDebuggerURL, 10*time.Second)
			if dialErr != nil {
				lastErr = dialErr
				time.Sleep(300 * time.Millisecond)
				continue
			}
			proc.cdp = client
			return proc, nil
		}
		if err != nil {
			lastErr = err
		}
		time.Sleep(400 * time.Millisecond)
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("browser did not expose a page target within %s", opts.Timeout)
	}
	return cleanup(lastErr)
}

type devToolsTarget struct {
	ID                   string `json:"id"`
	Type                 string `json:"type"`
	Title                string `json:"title"`
	URL                  string `json:"url"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

func pageTargets(port int) ([]devToolsTarget, error) {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/list", port))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var targets []devToolsTarget
	if err := json.NewDecoder(resp.Body).Decode(&targets); err != nil {
		return nil, err
	}

	pages := make([]devToolsTarget, 0, len(targets))
	for _, target := range targets {
		if target.Type == "page" && target.WebSocketDebuggerURL != "" {
			pages = append(pages, target)
		}
	}
	// prefer a real page over chrome://newtab / about:blank
	sort.SliceStable(pages, func(i, j int) bool {
		skip := func(url string) bool {
			return strings.HasPrefix(url, "about:") || strings.HasPrefix(url, "chrome://")
		}
		return !skip(pages[i].URL) && skip(pages[j].URL)
	})
	return pages, nil
}

func freePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("find free port: %w", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

// Navigate loads a URL in the attached page and waits for the load event.
func (b *browserProcess) Navigate(url string, timeout time.Duration) error {
	if b.cdp == nil {
		return fmt.Errorf("no page attached")
	}
	if timeout <= 0 {
		timeout = 45 * time.Second
	}
	if _, err := b.cdp.call("Page.enable", nil, 10*time.Second); err != nil {
		return err
	}
	if _, err := b.cdp.call("Page.navigate", map[string]any{"url": url}, timeout); err != nil {
		return err
	}
	b.pageURL = url
	return nil
}

// WaitFor polls a JS expression until it returns true.
func (b *browserProcess) WaitFor(expression string, timeout time.Duration) bool {
	if b.cdp == nil {
		return false
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var ready bool
		if err := b.cdp.evalValue(expression, 5*time.Second, &ready); err == nil && ready {
			return true
		}
		time.Sleep(300 * time.Millisecond)
	}
	return false
}

// Close detaches and terminates the browser (profile stays on disk on purpose).
func (b *browserProcess) Close() {
	if b == nil {
		return
	}
	if b.cdp != nil {
		_ = b.cdp.close()
		b.cdp = nil
	}
	if b.cmd != nil && b.cmd.Process != nil {
		_ = b.cmd.Process.Kill()
		_, _ = b.cmd.Process.Wait()
	}
}
