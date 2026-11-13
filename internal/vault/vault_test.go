package vault

import (
	"encoding/base64"
	"runtime"
	"testing"
)

func TestPasswordRoundTrip(t *testing.T) {
	plain := []byte(`{"app":"NextBox","version":2}`)
	b, err := Encrypt(plain, "correct horse")
	if err != nil {
		t.Fatal(err)
	}
	e, ok := Parse(b)
	if !ok || e.KDF != KDFPBKDF2 || e.Iterations != Iterations {
		t.Fatalf("envelope: %s", b)
	}
	if _, _, err := Decrypt(e, "wrong"); err != ErrPassword {
		t.Errorf("wrong password: %v", err)
	}
	got, key, err := Decrypt(e, "correct horse")
	if err != nil || string(got) != string(plain) {
		t.Fatalf("decrypt: %q %v", got, err)
	}
	// written again with the kept key: same salt, new IV, still opens
	again, _ := key.Seal(plain)
	e2, _ := Parse(again)
	if e2.Salt != e.Salt || e2.IV == e.IV {
		t.Error("reseal should keep the salt and change the IV")
	}
	if got, _, err := Decrypt(e2, "correct horse"); err != nil || string(got) != string(plain) {
		t.Errorf("reseal: %v", err)
	}
}

// A backup made by NextBox for Android (BackupCrypto.kt) with the password
// "nextbox" holding {"app":"NextBox"}: produced with the same algorithm in
// Kotlin's terms (PBKDF2WithHmacSHA256, AES/GCM/NoPadding, 128-bit tag).
func TestAndroidEnvelope(t *testing.T) {
	plain := []byte(`{"app":"NextBox"}`)
	salt := []byte("0123456789abcdef")
	iv := []byte("abcdefghijkl")
	key, _ := Key("nextbox", salt, 1000)
	data, _ := gcmSeal(key, iv, plain)
	e := Envelope{App: "NextBox", Encrypted: true, KDF: KDFPBKDF2, Iterations: 1000,
		Salt: "MDEyMzQ1Njc4OWFiY2RlZg==", IV: "YWJjZGVmZ2hpamts"}
	e.Data = b64(data)
	got, _, err := Decrypt(e, "nextbox")
	if err != nil || string(got) != string(plain) {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip()
	}
	b, err := SealWindows([]byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	e, ok := Parse(b)
	if !ok || e.KDF != KDFWindows {
		t.Fatalf("%s", b)
	}
	got, err := OpenWindows(e)
	if err != nil || string(got) != "secret" {
		t.Fatalf("%q %v", got, err)
	}
}

func b64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }
