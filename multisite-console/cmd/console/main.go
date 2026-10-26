// Command console runs the Multi-Site Console in web mode: it serves the
// dashboard and the /api JSON surface from one standard-library HTTP server.
//
// The desktop build (Wails) binds the very same core.App methods; see
// desktop/ and README.md. Web mode exists so the app can be developed, demoed
// and tested without any toolchain beyond Go.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"multisiteconsole/internal/core"
)

func main() {
	dataDir := flag.String("data", "data", "runtime data folder (settings, logs, sequences)")
	sitesDir := flag.String("sites", "sites", "site registry folder (sites/<id>/site.json)")
	uiDir := flag.String("ui", filepath.Join("cmd", "console", "ui"), "dashboard assets folder")
	addr := flag.String("addr", "0.0.0.0:8090", "listen address")
	flag.Parse()

	if err := core.EnsureUIDir(*uiDir); err != nil {
		log.Fatalf("dashboard assets not found at %s: %v", *uiDir, err)
	}

	app, err := core.New(*dataDir, *sitesDir, "http")
	if err != nil {
		log.Fatalf("start console: %v", err)
	}

	server := &http.Server{
		Addr:              *addr,
		Handler:           app.Handler(*uiDir),
		ReadHeaderTimeout: 5 * time.Second,
	}

	abs, _ := filepath.Abs(*dataDir)
	fmt.Printf("Multi-Site Console %s\n", core.Version)
	fmt.Printf("  sites   : %s\n", *sitesDir)
	fmt.Printf("  data    : %s\n", abs)
	fmt.Printf("  ui      : %s\n", *uiDir)
	fmt.Printf("  listen  : http://%s\n", *addr)
	fmt.Println("  API     : /api/GetSnapshot, /api/SaveSettings, /api/GetLogs, ...")

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server: %v", err)
	}
	_ = os.Stdout.Sync()
}
