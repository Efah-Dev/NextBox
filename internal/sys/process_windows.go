//go:build windows

package sys

import (
	"path/filepath"
	"sort"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// RunningProgram is a program that is running now, for picking one in a
// routing rule.
type RunningProgram struct {
	Name string `json:"name"` // e.g. "chrome.exe", what process rules match
	Path string `json:"path"` // full path when it can be read, else ""
}

// RunningPrograms lists the programs running now, one entry per program
// name, sorted by name. Windows' own idle and system processes are left
// out.
func RunningPrograms() ([]RunningProgram, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snap)
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	byName := map[string]RunningProgram{}
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		name := windows.UTF16ToString(e.ExeFile[:])
		if e.ProcessID <= 4 || name == "" || !strings.EqualFold(filepath.Ext(name), ".exe") {
			continue
		}
		key := strings.ToLower(name)
		if p, ok := byName[key]; ok && p.Path != "" {
			continue
		}
		byName[key] = RunningProgram{Name: name, Path: processPath(e.ProcessID)}
	}
	out := make([]RunningProgram, 0, len(byName))
	for _, p := range byName {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out, nil
}

// processPath reads the full path of a process's program; "" for the
// processes of other users or of the system that cannot be opened.
func processPath(pid uint32) string {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, windows.MAX_LONG_PATH)
	size := uint32(len(buf))
	if windows.QueryFullProcessImageName(h, 0, &buf[0], &size) != nil {
		return ""
	}
	return windows.UTF16ToString(buf[:size])
}
