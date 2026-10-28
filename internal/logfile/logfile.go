// Package logfile writes the application log to timestamped files, starting a
// new file when the current one reaches its size limit, and prunes old files.
package logfile

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const prefix, suffix = "NextBox_", ".log"

// Writer appends lines to <dir>/NextBox_2006-01-02_15-04-05.log. The file is
// created on the first write, so enabling logging alone leaves no empty file.
type Writer struct {
	mu       sync.Mutex
	dir      string
	maxBytes int64
	f        *os.File
	w        *bufio.Writer
	size     int64
	flushing bool
}

func New(dir string, maxMB int) *Writer {
	return &Writer{dir: dir, maxBytes: int64(max(1, maxMB)) << 20}
}

func (w *Writer) SetMaxMB(mb int) {
	w.mu.Lock()
	w.maxBytes = int64(max(1, mb)) << 20
	w.mu.Unlock()
}

// WriteLine appends one line. Writes are buffered and flushed within a second.
func (w *Writer) WriteLine(line string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n := int64(len(line) + 2)
	if w.f != nil && w.size+n > w.maxBytes {
		w.closeLocked()
	}
	if w.f == nil && !w.openLocked() {
		return
	}
	w.w.WriteString(line)
	w.w.WriteString("\r\n")
	w.size += n
	if !w.flushing {
		w.flushing = true
		time.AfterFunc(time.Second, w.flush)
	}
}

func (w *Writer) flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.flushing = false
	if w.w != nil {
		w.w.Flush()
	}
}

func (w *Writer) openLocked() bool {
	if err := os.MkdirAll(w.dir, 0o755); err != nil {
		return false
	}
	name := prefix + time.Now().Format("2006-01-02_15-04-05")
	path := filepath.Join(w.dir, name+suffix)
	for i := 2; fileExists(path); i++ { // several rotations within one second
		path = filepath.Join(w.dir, name+"_"+strconv.Itoa(i)+suffix)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return false
	}
	w.f, w.w, w.size = f, bufio.NewWriterSize(f, 32<<10), 0
	return true
}

func (w *Writer) closeLocked() {
	if w.f == nil {
		return
	}
	w.w.Flush()
	w.f.Close()
	w.f, w.w = nil, nil
}

// Close flushes and closes the current file; the next write starts a new one.
func (w *Writer) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closeLocked()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func logFiles(dir string) []os.DirEntry {
	entries, _ := os.ReadDir(dir)
	var out []os.DirEntry
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), prefix) && strings.HasSuffix(e.Name(), suffix) {
			out = append(out, e)
		}
	}
	return out
}

// Prune deletes log files last modified more than keepDays ago (0 keeps all).
func Prune(dir string, keepDays int) int {
	if keepDays <= 0 {
		return 0
	}
	cutoff := time.Now().AddDate(0, 0, -keepDays)
	removed := 0
	for _, e := range logFiles(dir) {
		if info, err := e.Info(); err == nil && info.ModTime().Before(cutoff) {
			if os.Remove(filepath.Join(dir, e.Name())) == nil {
				removed++
			}
		}
	}
	return removed
}

// Stats reports how many log files exist and their total size.
func Stats(dir string) (count int, bytes int64) {
	for _, e := range logFiles(dir) {
		if info, err := e.Info(); err == nil {
			count++
			bytes += info.Size()
		}
	}
	return
}

// RemoveAll deletes every log file in dir. Close the Writer first.
func RemoveAll(dir string) int {
	removed := 0
	for _, e := range logFiles(dir) {
		if os.Remove(filepath.Join(dir, e.Name())) == nil {
			removed++
		}
	}
	return removed
}
