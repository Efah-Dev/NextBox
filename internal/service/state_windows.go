//go:build windows

package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// Dir is where the service is installed: "%ProgramFiles%\NextBox Service".
func Dir() string {
	pf, err := windows.KnownFolderPath(windows.FOLDERID_ProgramFiles, 0)
	if err != nil || pf == "" {
		pf = os.Getenv("ProgramFiles")
	}
	if pf == "" {
		pf = `C:\Program Files`
	}
	return filepath.Join(pf, DirName)
}

// state is what the service remembers, in Dir()\service.json: the users who
// installed it (they may use it) and, per user, the program to start at
// logon.
type state struct {
	Version string           `json:"version"`
	Users   map[string]*user `json:"users"` // by SID
}

type user struct {
	Autostart string `json:"autostart,omitempty"`
}

func statePath(dir string) string { return filepath.Join(dir, "service.json") }

func loadState(dir string) *state {
	st := &state{Users: map[string]*user{}}
	if b, err := os.ReadFile(statePath(dir)); err == nil {
		json.Unmarshal(b, st)
	}
	if st.Users == nil {
		st.Users = map[string]*user{}
	}
	return st
}

func (st *state) save(dir string) error {
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := statePath(dir) + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, statePath(dir))
}

// pipeSDDL lets the system, administrators and the given users use the pipe,
// and nobody over the network.
func pipeSDDL(sids []string) string {
	var b strings.Builder
	b.WriteString("D:P(D;;GA;;;NU)(A;;GA;;;SY)(A;;GA;;;BA)")
	for _, s := range sids {
		if _, err := windows.StringToSid(s); err == nil {
			b.WriteString("(A;;GRGW;;;" + s + ")")
		}
	}
	return b.String()
}

// FileHash is the SHA-256 of a file, "" when it cannot be read.
func FileHash(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

// tokenSID is the user a token belongs to.
func tokenSID(t windows.Token) (string, error) {
	u, err := t.GetTokenUser()
	if err != nil {
		return "", err
	}
	return u.User.Sid.String(), nil
}

// CurrentUserSID is the user this program runs as.
func CurrentUserSID() (string, error) {
	return tokenSID(windows.GetCurrentProcessToken())
}
