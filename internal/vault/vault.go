// Package vault encrypts NextBox data: backups that leave the computer, and
// optionally the node list on disk.
//
// The envelope is the one NextBox for Android uses for its backups
// (BackupCrypto.kt), so a backup made on either side opens on the other:
// AES-256-GCM with a key derived from the password by PBKDF2-HMAC-SHA256,
// fields in standard Base64. At rest the key may instead be protected by
// Windows for the current user (DPAPI), which needs no password.
package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"

	"nextbox/internal/i18n"
	"nextbox/internal/sys"
)

const (
	KDFPBKDF2  = "PBKDF2-HMAC-SHA256"
	KDFWindows = "DPAPI" // NextBox for Windows only: bound to the Windows account
	Iterations = 210_000
)

// Envelope is an encrypted document.
type Envelope struct {
	App        string `json:"app"`
	Encrypted  bool   `json:"encrypted"`
	KDF        string `json:"kdf"`
	Iterations int    `json:"iterations,omitempty"`
	Salt       string `json:"salt,omitempty"`
	IV         string `json:"iv,omitempty"`
	Data       string `json:"data"`
}

// ErrPassword is returned for a wrong password (or a damaged file: GCM
// cannot tell the two apart).
var ErrPassword = errors.New("wrong password")

// Parse returns the envelope when b is an encrypted document.
func Parse(b []byte) (Envelope, bool) {
	var e Envelope
	if !strings.Contains(string(b[:min(len(b), 4096)]), `"encrypted"`) {
		return e, false
	}
	if json.Unmarshal(b, &e) != nil || !e.Encrypted || e.Data == "" {
		return e, false
	}
	return e, true
}

// Key derives the AES key for a password.
func Key(password string, salt []byte, iterations int) ([]byte, error) {
	if iterations < 1 || iterations > 5_000_000 {
		return nil, errors.New(i18n.T("文件已损坏"))
	}
	return pbkdf2.Key(sha256.New, password, salt, iterations, 32)
}

// PasswordKey is a derived key with its salt, so that a file can be written
// again without the slow derivation.
type PasswordKey struct {
	key, salt  []byte
	iterations int
}

// NewPasswordKey derives a key for a new salt.
func NewPasswordKey(password string) (*PasswordKey, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	key, err := Key(password, salt, Iterations)
	if err != nil {
		return nil, err
	}
	return &PasswordKey{key: key, salt: salt, iterations: Iterations}, nil
}

// Seal encrypts plain with the key.
func (k *PasswordKey) Seal(plain []byte) ([]byte, error) {
	iv := make([]byte, 12)
	if _, err := rand.Read(iv); err != nil {
		return nil, err
	}
	data, err := gcmSeal(k.key, iv, plain)
	if err != nil {
		return nil, err
	}
	b64 := base64.StdEncoding.EncodeToString
	return json.MarshalIndent(Envelope{
		App: "NextBox", Encrypted: true, KDF: KDFPBKDF2, Iterations: k.iterations,
		Salt: b64(k.salt), IV: b64(iv), Data: b64(data),
	}, "", "  ")
}

// Encrypt seals plain with a password (a fresh salt every time).
func Encrypt(plain []byte, password string) ([]byte, error) {
	k, err := NewPasswordKey(password)
	if err != nil {
		return nil, err
	}
	return k.Seal(plain)
}

// Decrypt opens a password envelope. It also returns the key, so that the
// caller can write the document back (see PasswordKey).
func Decrypt(e Envelope, password string) ([]byte, *PasswordKey, error) {
	if e.KDF != KDFPBKDF2 {
		return nil, nil, errors.New(i18n.T("不支持的加密方式：") + e.KDF)
	}
	dec := base64.StdEncoding.DecodeString
	salt, err1 := dec(e.Salt)
	iv, err2 := dec(e.IV)
	data, err3 := dec(e.Data)
	if err := errors.Join(err1, err2, err3); err != nil {
		return nil, nil, errors.New(i18n.T("文件已损坏"))
	}
	key, err := Key(password, salt, e.Iterations)
	if err != nil {
		return nil, nil, err
	}
	plain, err := gcmOpen(key, iv, data)
	if err != nil {
		return nil, nil, ErrPassword
	}
	return plain, &PasswordKey{key: key, salt: salt, iterations: e.Iterations}, nil
}

// SealWindows protects plain for the current Windows user.
func SealWindows(plain []byte) ([]byte, error) {
	data, err := sys.Protect(plain)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(Envelope{App: "NextBox", Encrypted: true, KDF: KDFWindows, Data: base64.StdEncoding.EncodeToString(data)}, "", "  ")
}

// OpenWindows opens a document sealed by SealWindows.
func OpenWindows(e Envelope) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(e.Data)
	if err != nil {
		return nil, errors.New(i18n.T("文件已损坏"))
	}
	plain, err := sys.Unprotect(data)
	if err != nil {
		return nil, errors.New(i18n.T("无法解密：配置是在另一台电脑或另一个 Windows 账户下加密的，请在原来的电脑上导出备份再导入"))
	}
	return plain, nil
}

func gcmSeal(key, iv, plain []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Seal(nil, iv, plain, nil), nil
}

func gcmOpen(key, iv, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(iv) != gcm.NonceSize() {
		return nil, ErrPassword
	}
	return gcm.Open(nil, iv, data, nil)
}
