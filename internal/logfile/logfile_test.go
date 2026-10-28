package logfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRotatePruneRemove(t *testing.T) {
	dir := t.TempDir()
	w := New(dir, 1)
	line := strings.Repeat("x", 1000)
	for i := 0; i < 1500; i++ { // ~1.5 MB -> two files at a 1 MB limit
		w.WriteLine(line)
	}
	w.Close()
	if n, size := Stats(dir); n != 2 || size < 1_400_000 {
		t.Fatalf("files=%d size=%d, want 2 files ~1.5MB", n, size)
	}

	old := filepath.Join(dir, prefix+"2000-01-01_00-00-00"+suffix)
	os.WriteFile(old, []byte("old"), 0o644)
	past := time.Now().AddDate(0, 0, -30)
	os.Chtimes(old, past, past)
	if removed := Prune(dir, 7); removed != 1 {
		t.Fatalf("pruned %d, want 1", removed)
	}
	if removed := RemoveAll(dir); removed != 2 {
		t.Fatalf("removed %d, want 2", removed)
	}
}
