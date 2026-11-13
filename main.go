package main

import (
	"embed"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"nextbox/internal/geo"
	"nextbox/internal/i18n"
	"nextbox/internal/sys"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:generate go run ./cmd/genicon

//go:embed all:frontend/dist
var assets embed.FS

// Translations shared with the UI (see internal/i18n).
//
//go:embed frontend/src/i18n/en.json
var i18nEN []byte

//go:embed frontend/src/i18n/zh-TW.json
var i18nTW []byte

// Country names in node names, shared with the UI (see internal/geo).
//
//go:embed frontend/src/places.json
var placesJSON []byte

const Version = "2.0.0"

func main() {
	startMinimised := false
	for _, arg := range os.Args[1:] {
		switch {
		case strings.HasPrefix(arg, "--wait-parent="):
			// Restarted as administrator: wait for the old instance to release the single instance lock.
			if pid, err := strconv.Atoi(strings.TrimPrefix(arg, "--wait-parent=")); err == nil {
				sys.WaitPid(pid)
			}
		case arg == "--minimized":
			startMinimised = true
		case strings.HasPrefix(arg, "--restore-proxy="):
			// RunOnce safety net (see sys.SetProxyCleanup): the last session
			// ended without NextBox removing its system proxy.
			restoreProxy(strings.TrimPrefix(arg, "--restore-proxy="))
			return
		}
	}

	if err := i18n.Init(i18nEN, i18nTW); err != nil {
		log.Println("i18n:", err)
	}

	app := NewApp()
	app.autoStarted = startMinimised
	if err := geo.LoadPlaces(placesJSON); err != nil {
		log.Println("places:", err)
	}

	err := wails.Run(&options.App{
		Title:            "NextBox",
		Width:            1080,
		Height:           680,
		MinWidth:         920,
		MinHeight:        620,
		Frameless:        true,
		StartHidden:      startMinimised, // autostart: stay in the tray
		BackgroundColour: &options.RGBA{R: 250, G: 241, B: 246, A: 255},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:     app.startup,
		OnBeforeClose: app.beforeClose,
		OnShutdown:    app.shutdown,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               "nextbox-7d0f3c4e-single-instance",
			OnSecondInstanceLaunch: app.onSecondInstance,
		},
		Bind: []interface{}{app},
		Windows: &windows.Options{
			Theme: windows.SystemDefault,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}

// restoreProxy removes a system proxy pointing at addr, unless something
// listens there again (NextBox may already be running and connected).
func restoreProxy(addr string) {
	if conn, err := net.DialTimeout("tcp", addr, 300*time.Millisecond); err == nil {
		conn.Close()
		return
	}
	sys.ClearSystemProxy(addr)
}
