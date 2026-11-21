# fyne.io/systray v1.12.2, modified for NextBox

Copy of https://github.com/fyne-io/systray at v1.12.2 (Apache License 2.0,
see LICENSE), used through a `replace` in NextBox's go.mod.

Changed: `systray_windows.go`, `initInstance`. When `Shell_NotifyIcon(NIM_ADD)`
fails (it can time out right after logon although the icon was added), the
icon is checked with `NIM_MODIFY` and adding is retried for about a minute
instead of giving up, which left a blank tray icon that did nothing.

The `example` folder and the tests were left out.
