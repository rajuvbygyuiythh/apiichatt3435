// =============================================================================
//  app-methods-excerpt.go — READ-ONLY REFERENCE (not a compiling file)
// -----------------------------------------------------------------------------
//  The App struct and the emit/log plumbing from the original app.go, shown
//  without the domain logic. Copy the shape, never the behaviour.
//  Backend patterns are documented in design-kit/07-BACKEND-PATTERNS.md.
// =============================================================================

type App struct {
	ctx             context.Context
	dataDir         string
	uiStore         *uistore.Store
	mu              sync.Mutex
	localTestMu     sync.Mutex
	localTest       *bot.LocalChatSession
	chatFeed        *homeChatFeed
	showLog         *showLogFeed
	botLog          *botLogFeed
	showModeRunning bool
	running         bool
	runID           uint64
	botCancel       context.CancelFunc
	activeWorkers   map[string]bool
}

func (a *App) emitStatus(running bool) {
	a.emit("ui:running", running)
}

func (a *App) emit(name string, data any) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, name, data)
}

func (a *App) log(level string, message string) {
	// Persist first, independently of Badger/UI events; no Home visibility filter.
	disk := applog.Logger{Path: applog.Path(a.dataDir), Source: "app"}
	disk.Log(level, message)
	entry, err := a.uiStore.AppendLog(level, message)
	if err == nil {
		a.emit("ui:log", entry) // Browser status still consumes this event.
	} else {
		disk.Error(fmt.Sprintf("UI log persistence failed: %v", err))
	}
}

