// NextBox for Windows: the program starts here; everything else is in
// internal/app (the window, tray and the functions the UI calls) and the
// other packages under internal/. This file keeps what must sit next to the
// frontend folder: the built UI and the files shared with it, embedded.
package main

import (
	"embed"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"nextbox/internal/app"
	"nextbox/internal/geo"
	"nextbox/internal/i18n"
	"nextbox/internal/sys"
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

func main() {
	// run as administrator by NextBox to install or remove its service
	for _, arg := range os.Args[1:] {
		if arg == "--service-install" || arg == "--service-uninstall" {
			os.Exit(app.ServiceHelper(os.Args[1:]))
		}
	}
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
	if err := geo.LoadPlaces(placesJSON); err != nil {
		log.Println("places:", err)
	}
	if err := app.Run(assets, startMinimised); err != nil {
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
