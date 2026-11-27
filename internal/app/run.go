package app

// Starting the window. The Wails options live here because they hand the
// App's unexported lifecycle methods to Wails: exported ones would be bound
// to the UI as well.

import (
	"io/fs"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

// Run shows the window (or only the tray with startMinimised, when started
// by Windows) and returns when NextBox quits. assets is the built UI.
func Run(assets fs.FS, startMinimised bool) error {
	app := NewApp()
	app.autoStarted = startMinimised
	app.ui, _ = fs.Sub(assets, "frontend/dist")
	return wails.Run(&options.App{
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
		// files dragged into the window are imported (frontend DropZone.vue)
		DragAndDrop: &options.DragAndDrop{EnableFileDrop: true},
		Windows: &windows.Options{
			Theme: windows.SystemDefault,
		},
	})
}

// ServiceHelper is what NextBox.exe does when started as administrator to
// install or remove the NextBox service (see serviceHelper); it returns the
// exit code.
func ServiceHelper(args []string) int { return serviceHelper(args) }
