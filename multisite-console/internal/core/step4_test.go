package core

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Step 4 · real browser driver (WebSocket + CDP), tested without a browser
// ---------------------------------------------------------------------------

// wsFakeServer is a tiny WebSocket server used by the client tests.
type wsFakeServer struct {
	t        *testing.T
	onText   func(payload string) (reply string, closeAfter bool)
	server   *httptest.Server
	mu       sync.Mutex
	got      []string
	upgraded bool
}

func newWSFakeServer(t *testing.T, onText func(string) (string, bool)) *wsFakeServer {
	t.Helper()
	fake := &wsFakeServer{t: t, onText: onText}
	fake.server = httptest.NewServer(http.HandlerFunc(fake.handle))
	t.Cleanup(fake.server.Close)
	return fake
}

func (f *wsFakeServer) handle(w http.ResponseWriter, r *http.Request) {
	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		http.Error(w, "not a websocket request", http.StatusBadRequest)
		return
	}
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "no hijacker", http.StatusInternalServerError)
		return
	}
	conn, buf, err := hijacker.Hijack()
	if err != nil {
		return
	}
	defer conn.Close()

	sum := sha1.Sum([]byte(key + wsGUID))
	accept := base64.StdEncoding.EncodeToString(sum[:])
	_, _ = buf.WriteString("HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\nConnection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + accept + "\r\n\r\n")
	_ = buf.Flush()

	f.mu.Lock()
	f.upgraded = true
	f.mu.Unlock()

	reader := bufio.NewReader(conn)
	for {
		payload, opcode, err := readServerFrame(reader)
		if err != nil {
			return
		}
		if opcode == 0x8 {
			return
		}
		if opcode != 0x1 {
			continue
		}
		f.mu.Lock()
		f.got = append(f.got, string(payload))
		f.mu.Unlock()

		reply, closeAfter := f.onText(string(payload))
		if reply != "" {
			if err := writeServerFrame(conn, 0x1, []byte(reply)); err != nil {
				return
			}
		}
		if closeAfter {
			_ = writeServerFrame(conn, 0x8, []byte{0x03, 0xE8})
			return
		}
	}
}

func readServerFrame(reader *bufio.Reader) ([]byte, byte, error) {
	var header [2]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return nil, 0, err
	}
	opcode := header[0] & 0x0F
	masked := header[1]&0x80 != 0
	length := int64(header[1] & 0x7F)
	switch length {
	case 126:
		var ext [2]byte
		if _, err := io.ReadFull(reader, ext[:]); err != nil {
			return nil, 0, err
		}
		length = int64(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		if _, err := io.ReadFull(reader, ext[:]); err != nil {
			return nil, 0, err
		}
		length = int64(binary.BigEndian.Uint64(ext[:]))
	}
	var mask [4]byte
	if masked {
		if _, err := io.ReadFull(reader, mask[:]); err != nil {
			return nil, 0, err
		}
	}
	payload := make([]byte, length)
	if length > 0 {
		if _, err := io.ReadFull(reader, payload); err != nil {
			return nil, 0, err
		}
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}
	return payload, opcode, nil
}

func writeServerFrame(conn net.Conn, opcode byte, payload []byte) error {
	frame := []byte{0x80 | opcode}
	length := len(payload)
	switch {
	case length < 126:
		frame = append(frame, byte(length))
	case length <= 0xFFFF:
		frame = append(frame, 126, 0, 0)
		binary.BigEndian.PutUint16(frame[len(frame)-2:], uint16(length))
	default:
		frame = append(frame, 127, 0, 0, 0, 0, 0, 0, 0, 0)
		binary.BigEndian.PutUint64(frame[len(frame)-8:], uint64(length))
	}
	_, err := conn.Write(append(frame, payload...))
	return err
}

func TestWebSocketClientEcho(t *testing.T) {
	fake := newWSFakeServer(t, func(payload string) (string, bool) {
		return "echo:" + payload, false
	})
	wsURL := "ws" + strings.TrimPrefix(fake.server.URL, "http")

	conn, err := dialWS(wsURL, 5*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.close()

	if err := conn.writeText([]byte("hello")); err != nil {
		t.Fatalf("write: %v", err)
	}
	reply, err := conn.readMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(reply) != "echo:hello" {
		t.Fatalf("unexpected echo: %q", reply)
	}

	// a large frame exercises the 64-bit length path
	big := strings.Repeat("x", 70000)
	if err := conn.writeText([]byte(big)); err != nil {
		t.Fatalf("write big: %v", err)
	}
	reply, err = conn.readMessage()
	if err != nil {
		t.Fatalf("read big: %v", err)
	}
	if len(reply) != len(big)+len("echo:") {
		t.Fatalf("big frame length mismatch: %d", len(reply))
	}
}

func TestWebSocketRejectsNonUpgrade(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusForbidden)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	if _, err := dialWS(wsURL, 3*time.Second); err == nil {
		t.Fatal("expected handshake failure")
	}
}

// fakeChrome is a scripted DevTools endpoint: HTTP /json/list + one page socket.
type fakeChrome struct {
	t *testing.T

	mu          sync.Mutex
	snapshot    string
	probe       string
	readInput   string
	navigations []string
	typed       []string
	clicks      []string
	methods     []string
	listeners   map[int]net.Listener
	hasInput    bool
}

func newFakeChrome(t *testing.T) *fakeChrome {
	return &fakeChrome{
		t:         t,
		probe:     "family=family_a phase=chatting input=true msgs=1 prot=clean",
		readInput: "",
		hasInput:  true,
		snapshot: `{"controls":{"phase":"chatting","hasInput":true,"canSend":true,"protection":"clean","observerReady":true},` +
			`"messages":[{"author":"partner","text":"hey, how's it going?","kind":"text","order":1}],"revision":3}`,
	}
}

// start serves on the port the browser would have been given.
func (f *fakeChrome) start(port int) error {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return err
	}
	f.mu.Lock()
	if f.listeners == nil {
		f.listeners = map[int]net.Listener{}
	}
	f.listeners[port] = listener
	f.mu.Unlock()

	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/json/list"):
			f.mu.Lock()
			snapshot := f.snapshot
			f.mu.Unlock()
			_ = snapshot
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `[{"id":"page-1","type":"page","title":"fake","url":"https://example.com/","webSocketDebuggerUrl":"ws://127.0.0.1:%d/devtools/page/page-1"}]`, port)
		case strings.HasPrefix(r.URL.Path, "/devtools/page/"):
			f.serveWS(w, r)
		default:
			http.NotFound(w, r)
		}
	})}
	go func() { _ = server.Serve(listener) }()
	f.t.Cleanup(func() { _ = server.Close() })
	return nil
}

func (f *fakeChrome) serveWS(w http.ResponseWriter, r *http.Request) {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return
	}
	conn, buf, err := hijacker.Hijack()
	if err != nil {
		return
	}
	defer conn.Close()

	sum := sha1.Sum([]byte(r.Header.Get("Sec-WebSocket-Key") + wsGUID))
	_, _ = buf.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + base64.StdEncoding.EncodeToString(sum[:]) + "\r\n\r\n")
	_ = buf.Flush()

	reader := bufio.NewReader(conn)
	for {
		payload, opcode, err := readServerFrame(reader)
		if err != nil {
			return
		}
		if opcode == 0x8 {
			return
		}
		if opcode != 0x1 {
			continue
		}

		var call struct {
			ID     int            `json:"id"`
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
		}
		if err := json.Unmarshal(payload, &call); err != nil {
			continue
		}
		reply := f.handleCall(call.ID, call.Method, call.Params)
		if err := writeServerFrame(conn, 0x1, reply); err != nil {
			return
		}
	}
}

func (f *fakeChrome) handleCall(id int, method string, params map[string]any) []byte {
	f.mu.Lock()
	f.methods = append(f.methods, method)
	f.mu.Unlock()

	result := map[string]any{}

	switch method {
	case "Page.navigate":
		url, _ := params["url"].(string)
		f.mu.Lock()
		f.navigations = append(f.navigations, url)
		f.mu.Unlock()
		result = map[string]any{"frameId": "frame-1"}
	case "Input.insertText":
		text, _ := params["text"].(string)
		f.mu.Lock()
		f.typed = append(f.typed, text)
		f.mu.Unlock()
		result = map[string]any{}
	case "Page.captureScreenshot":
		result = map[string]any{"data": base64.StdEncoding.EncodeToString([]byte("\x89PNG\r\n\x1a\nfake"))}
	case "Page.enable", "Runtime.enable":
		result = map[string]any{}
	case "Runtime.evaluate":
		expression, _ := params["expression"].(string)
		value := f.eval(expression)
		result = map[string]any{"result": map[string]any{"type": "string", "value": value}}
	default:
		result = map[string]any{}
	}

	envelope, _ := json.Marshal(map[string]any{"id": id, "result": result})
	return envelope
}

func (f *fakeChrome) eval(expression string) any {
	f.mu.Lock()
	defer f.mu.Unlock()

	switch {
	case strings.Contains(expression, ".probe()"):
		return f.probe
	case strings.Contains(expression, "JSON.stringify") && strings.Contains(expression, ".snapshot()"):
		return f.snapshot
	case strings.Contains(expression, "document.title"):
		return "Fake Page"
	case strings.Contains(expression, ".focusInput()"):
		return f.hasInput
	case strings.Contains(expression, ".readInput()"):
		return f.readInput
	case strings.Contains(expression, ".clearInput()"):
		f.readInput = ""
		return true
	case strings.Contains(expression, "readystate") || strings.Contains(expression, "readyState"):
		return true
	case strings.Contains(expression, ".snapshot().controls.hasInput"):
		return f.hasInput
	case strings.Contains(expression, ".click("):
		labels := expression[strings.Index(expression, ".click("):]
		f.clicks = append(f.clicks, labels)
		if strings.Contains(labels, "SEND") {
			return "SEND"
		}
		if strings.Contains(labels, "SKIP") {
			return "SKIP"
		}
		return ""
	}
	return true
}

// setupFakeChrome makes DiscoverBrowser succeed and routes launches to the fake.
func setupFakeChrome(t *testing.T) *fakeChrome {
	t.Helper()

	browserPath := filepath.Join(t.TempDir(), "fake-chrome")
	script := "#!/bin/sh\nif [ \"$1\" = \"--version\" ]; then echo 'Fake Chrome 1.0.0'; fi\n"
	if err := os.WriteFile(browserPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake browser: %v", err)
	}
	t.Setenv("MULTISITE_BROWSER", browserPath)

	fake := newFakeChrome(t)
	originalStarter := browserStarter
	browserStarter = func(opts LaunchOptions, args []string) (*exec.Cmd, error) {
		port := 0
		for _, arg := range args {
			if strings.HasPrefix(arg, "--remote-debugging-port=") {
				port, _ = strconv.Atoi(strings.TrimPrefix(arg, "--remote-debugging-port="))
			}
		}
		if port == 0 {
			return nil, fmt.Errorf("test launcher: no debugging port in args")
		}
		if err := fake.start(port); err != nil {
			return nil, err
		}
		return &exec.Cmd{}, nil
	}
	t.Cleanup(func() { browserStarter = originalStarter })
	return fake
}

func (f *fakeChrome) setSnapshot(snapshot string) {
	f.mu.Lock()
	f.snapshot = snapshot
	var parsed struct {
		Controls struct {
			HasInput bool `json:"hasInput"`
		} `json:"controls"`
	}
	if err := json.Unmarshal([]byte(snapshot), &parsed); err == nil {
		f.hasInput = parsed.Controls.HasInput
	}
	f.mu.Unlock()
}

func (f *fakeChrome) methodCount(method string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	count := 0
	for _, seen := range f.methods {
		if seen == method {
			count++
		}
	}
	return count
}

func (f *fakeChrome) typedLines() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.typed))
	copy(out, f.typed)
	return out
}

// ---------------------------------------------------------------------------
// launch → attach → drive, all against the scripted Chrome above
// ---------------------------------------------------------------------------

func TestBrowseLaunchNavigateEvaluate(t *testing.T) {
	fake := setupFakeChrome(t)

	proc, err := LaunchBrowser(LaunchOptions{
		ExecPath:   os.Getenv("MULTISITE_BROWSER"),
		ProfileDir: t.TempDir(),
		StartURL:   "https://example.com/alpha",
		Headless:   true,
		Timeout:    8 * time.Second,
	}, true)
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	defer proc.Close()

	if proc.cdp == nil {
		t.Fatal("no page attached")
	}
	if err := proc.Navigate("https://example.com/alpha", 8*time.Second); err != nil {
		t.Fatalf("navigate: %v", err)
	}
	if len(fake.navigations) == 0 || fake.navigations[len(fake.navigations)-1] != "https://example.com/alpha" {
		t.Fatalf("navigation not delivered: %v", fake.navigations)
	}

	var title string
	if err := proc.cdp.evalValue("document.title", 5*time.Second, &title); err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	shot, err := (&DriverSession{Browser: proc}).Screenshot()
	if err != nil {
		t.Fatalf("screenshot: %v", err)
	}
	if !strings.HasPrefix(string(shot), "\x89PNG") {
		t.Fatalf("unexpected screenshot bytes: %q", shot[:min(len(shot), 8)])
	}
}

func TestLaunchBrowserFailsFastWithoutTarget(t *testing.T) {
	browserPath := filepath.Join(t.TempDir(), "fake-chrome")
	os.WriteFile(browserPath, []byte("#!/bin/sh\nexit 0\n"), 0o755)

	originalStarter := browserStarter
	browserStarter = func(opts LaunchOptions, args []string) (*exec.Cmd, error) {
		return &exec.Cmd{}, nil // a "browser" that never opens a debugging port
	}
	defer func() { browserStarter = originalStarter }()

	start := time.Now()
	_, err := LaunchBrowser(LaunchOptions{
		ExecPath: browserPath, ProfileDir: t.TempDir(), Timeout: 2 * time.Second,
	}, true)
	if err == nil {
		t.Fatal("expected a launch failure")
	}
	if time.Since(start) > 8*time.Second {
		t.Fatalf("launch timeout not respected: %s", time.Since(start))
	}
}

func TestDiscoverBrowserHonoursEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chrome-test")
	os.WriteFile(path, []byte("#!/bin/sh\necho 'Chrome Test 9.9'\n"), 0o755)
	t.Setenv("MULTISITE_BROWSER", path)

	status := DiscoverBrowser("")
	if !status.Found || status.Path != path {
		t.Fatalf("env browser not picked up: %+v", status)
	}
	if status.Source != "env" {
		t.Fatalf("expected source env, got %q", status.Source)
	}
	if !strings.Contains(status.Version, "Chrome Test 9.9") {
		t.Fatalf("version not read: %q", status.Version)
	}
}

func TestDiscoverBrowserMissing(t *testing.T) {
	t.Setenv("MULTISITE_BROWSER", "")
	status := DiscoverBrowser(filepath.Join(t.TempDir(), "does-not-exist"))
	if status.Found {
		t.Skip("this machine really has a browser installed; nothing to assert")
	}
	if status.Note == "" {
		t.Fatal("missing browser must carry a note")
	}
	if len(status.Cands) == 0 {
		t.Fatal("candidates should be listed even when missing")
	}
}

func TestDriverSendAndSnapshot(t *testing.T) {
	fake := setupFakeChrome(t)

	app := newTestApp(t, 1)
	proc, err := LaunchBrowser(LaunchOptions{
		ExecPath: os.Getenv("MULTISITE_BROWSER"), ProfileDir: t.TempDir(), Headless: true, Timeout: 8 * time.Second,
	}, true)
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	defer proc.Close()

	driver := &DriverSession{Site: app.sites[0], Profile: "default", Browser: proc, driverJS: "// fake"}

	snapshot, err := driver.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if !snapshot.Controls.HasInput || snapshot.Controls.Phase != "chatting" {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	if len(snapshot.Messages) != 1 || snapshot.Messages[0].Author != "partner" {
		t.Fatalf("partner message not decoded: %+v", snapshot.Messages)
	}

	if err := driver.Send("hello there"); err != nil {
		t.Fatalf("send: %v", err)
	}
	if len(fake.typedLines()) != 1 || fake.typedLines()[0] != "hello there" {
		t.Fatalf("text not inserted: %v", fake.typedLines())
	}

	if got := driver.Click("SKIP"); got != "SKIP" {
		t.Fatalf("skip click: %q", got)
	}

	if probe := driver.Probe(); !strings.Contains(probe, "input=true") {
		t.Fatalf("probe: %q", probe)
	}
}

func TestDriverSendFailsWithoutComposer(t *testing.T) {
	fake := setupFakeChrome(t)
	fake.setSnapshot(`{"controls":{"phase":"lobby","hasInput":false,"canSend":false,"protection":"clean"}}`)

	app := newTestApp(t, 1)
	proc, err := LaunchBrowser(LaunchOptions{
		ExecPath: os.Getenv("MULTISITE_BROWSER"), ProfileDir: t.TempDir(), Headless: true, Timeout: 8 * time.Second,
	}, true)
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	defer proc.Close()

	driver := &DriverSession{Site: app.sites[0], Browser: proc}
	if err := driver.Send("hello"); err == nil || !strings.Contains(err.Error(), "composer") {
		t.Fatalf("expected composer error, got %v", err)
	}
}

func TestRunRealBrowserCycle(t *testing.T) {
	fake := setupFakeChrome(t)

	app := newTestApp(t, 1)
	app.sites[0].StartURL = "https://example.com/alpha"
	app.sites[0].Selectors = Selectors{
		ComposerInput: "#msg", SendButton: "button.send", SkipButton: "button.skip",
		StartButton: "button.start", MessageHistory: "#history", MessageRow: ".row",
	}
	app.sites[0].ProtectionSummary = "" // keep the fixture quiet
	if _, err := app.store.Apply("message", json.RawMessage(`{"strategy":"single","max_lines":2,"sequence":"hello there\nwhat's up?"}`)); err != nil {
		t.Fatalf("settings: %v", err)
	}

	if _, err := app.StartRun("worker", []string{"alpha"}, "default"); err != nil {
		t.Fatalf("start: %v", err)
	}

	deadline := time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		app.mu.Lock()
		running := app.running
		app.mu.Unlock()
		if !running {
			break
		}
		time.Sleep(150 * time.Millisecond)
	}

	app.mu.Lock()
	running, completed, failed := app.running, app.stats.Completed, app.stats.Failed
	app.mu.Unlock()
	if running {
		t.Fatal("run did not finish")
	}
	if completed != 1 || failed != 0 {
		t.Fatalf("expected 1 completed / 0 failed, got %d/%d", completed, failed)
	}

	_, entries, _ := app.Log().Since(0)
	var outs, ins []string
	for _, entry := range entries {
		switch entry.Kind {
		case "out":
			outs = append(outs, entry.Text)
		case "in":
			ins = append(ins, entry.Text)
		}
	}
	if len(outs) != 2 {
		t.Fatalf("expected 2 outbound lines, got %v", outs)
	}
	if len(ins) == 0 || ins[0] != "hey, how's it going?" {
		t.Fatalf("partner reply not logged: %v", ins)
	}
	if len(fake.typedLines()) != 2 {
		t.Fatalf("browser should have received 2 lines, got %v", fake.typedLines())
	}
	if fake.methodCount("Page.navigate") == 0 {
		t.Fatal("page was never navigated")
	}

	// the run records what it did in the profile folder
	sessionPath := filepath.Join(app.SitesDir(), "alpha", "profiles", "default", "session.json")
	if _, err := os.Stat(sessionPath); err != nil {
		t.Fatalf("session file missing after a real run: %v", err)
	}

	// everything the console wrote to disk is redacted + mirrored
	raw, err := os.ReadFile(app.LogPath())
	if err != nil {
		t.Fatalf("log file: %v", err)
	}
	if !strings.Contains(string(raw), "hello there") {
		t.Fatalf("file log missed the outbound line: %s", raw)
	}
}

func TestRunSimulationWhenNoBrowser(t *testing.T) {
	t.Setenv("MULTISITE_BROWSER", "")
	app := newTestApp(t, 1)
	// force "no browser" regardless of what this machine has installed
	if _, err := app.store.Apply("browser", json.RawMessage(fmt.Sprintf(`{"path":%q}`, filepath.Join(t.TempDir(), "nope")))); err != nil {
		t.Fatalf("browser settings: %v", err)
	}

	if _, err := app.StartRun("worker", []string{"alpha"}, "default"); err != nil {
		t.Fatalf("start: %v", err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		app.mu.Lock()
		running := app.running
		app.mu.Unlock()
		if !running {
			break
		}
		time.Sleep(120 * time.Millisecond)
	}

	_, entries, _ := app.Log().Since(0)
	foundSim := false
	for _, entry := range entries {
		if entry.Meta == "simulation" || strings.Contains(entry.Text, "SIMULATION") {
			foundSim = true
		}
	}
	if !foundSim {
		t.Skip("a real browser is installed on this machine; simulation path not exercised")
	}
}
