// NextBoxService.exe: the NextBox service (see internal/service). It is
// installed and started by NextBox ("设置 → TUN 模式 → 服务模式"); run by hand,
// it only says so.
package main

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows/svc"

	"nextbox/internal/service"
)

// Version is set by the build (-ldflags "-X main.Version=…") to NextBox's.
var Version = "dev"

func main() {
	if ok, err := svc.IsWindowsService(); err != nil || !ok {
		fmt.Fprintln(os.Stderr, "NextBoxService is the Windows service of NextBox; install it from NextBox (Settings → TUN mode).")
		os.Exit(2)
	}
	if err := service.Run(Version); err != nil {
		os.Exit(1)
	}
}
