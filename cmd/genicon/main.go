// Command genicon writes the application icons used by `wails build`
// (build/appicon.png, build/windows/icon.ico) and the README logo
// (docs/logo.png). The outputs are committed, so wails build never falls back
// to its default icon; rerun this only after changing internal/icon. From the
// project root:
//
//	go run ./cmd/genicon
package main

import (
	"log"
	"os"
	"path/filepath"

	"nextbox/internal/icon"
)

func main() {
	files := map[string][]byte{
		filepath.Join("build", "appicon.png"):         icon.PNG(1024, nil),
		filepath.Join("build", "windows", "icon.ico"): icon.ICO([]int{16, 20, 24, 32, 40, 48, 64, 128, 256}, nil),
		filepath.Join("docs", "logo.png"):             icon.PNG(256, nil),
	}
	for path, data := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			log.Fatal(err)
		}
		log.Println("wrote", path)
	}
}
