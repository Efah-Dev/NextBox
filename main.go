package main

import (
	"embed"
	"log"
	"os"
	"strconv"
	"strings"

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

const Version = "1.0.2"

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
		}
	}

	if err := i18n.Init(i18nEN, i18nTW); err != nil {
		log.Println("i18n:", err)
	}

	app := NewApp()

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
