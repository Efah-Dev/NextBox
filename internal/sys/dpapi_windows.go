//go:build windows

package sys

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// Protect encrypts data for the current Windows user (DPAPI): only this
// account on this computer can decrypt it, and no password is needed.
func Protect(data []byte) ([]byte, error) {
	return dpapi(data, true)
}

// Unprotect decrypts data from Protect.
func Unprotect(data []byte) ([]byte, error) {
	return dpapi(data, false)
}

func dpapi(data []byte, protect bool) ([]byte, error) {
	in := windows.DataBlob{Size: uint32(len(data))}
	if len(data) > 0 {
		in.Data = &data[0]
	}
	entropy := []byte("NextBox profiles")
	ent := windows.DataBlob{Size: uint32(len(entropy)), Data: &entropy[0]}
	var out windows.DataBlob
	const uiForbidden = 0x1
	var err error
	if protect {
		err = windows.CryptProtectData(&in, nil, &ent, 0, nil, uiForbidden, &out)
	} else {
		err = windows.CryptUnprotectData(&in, nil, &ent, 0, nil, uiForbidden, &out)
	}
	if err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return append([]byte(nil), unsafe.Slice(out.Data, out.Size)...), nil
}
