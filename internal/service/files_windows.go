//go:build windows

package service

import (
	"io"
	"os"
	"path/filepath"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func unsafePointer(st *windows.SERVICE_STATUS_PROCESS) unsafe.Pointer { return unsafe.Pointer(st) }

const sizeofStatus = unsafe.Sizeof(windows.SERVICE_STATUS_PROCESS{})

// Files copies into the service's folder: the service, and the core it runs.
// libcronet.dll (NaiveProxy) is optional.
var Files = []string{ExeName, "sing-box.exe", "libcronet.dll"}

func copyFiles(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for _, name := range Files {
		from := filepath.Join(src, name)
		if _, err := os.Stat(from); err != nil {
			if name == "libcronet.dll" {
				os.Remove(filepath.Join(dst, name))
				continue
			}
			return err
		}
		if err := copyFile(from, filepath.Join(dst, name)); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := to + ".new"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	// the old file may still be in use for a moment after the service stopped
	for i := 0; ; i++ {
		err = os.Rename(tmp, to)
		if err == nil || i == 20 {
			return err
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func removeDir(dir string) error {
	var err error
	for i := 0; i < 20; i++ {
		if err = os.RemoveAll(dir); err == nil {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return err
}
