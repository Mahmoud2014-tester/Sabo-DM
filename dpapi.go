package main

import (
	"encoding/base64"
	"errors"
	"runtime"
	"syscall"
	"unsafe"
)

// Login data (cookies) is never written to disk in plain text. It is sealed
// with Windows DPAPI, which ties it to your Windows user account: copying the
// file to another account or PC makes it unreadable.

var (
	modCrypt32    = syscall.NewLazyDLL("crypt32.dll")
	modKernel32   = syscall.NewLazyDLL("kernel32.dll")
	procProtect   = modCrypt32.NewProc("CryptProtectData")
	procUnprotect = modCrypt32.NewProc("CryptUnprotectData")
	procLocalFree = modKernel32.NewProc("LocalFree")
)

type dataBlob struct {
	cbData uint32
	pbData *byte
}

func blobOf(b []byte) *dataBlob {
	if len(b) == 0 {
		return &dataBlob{}
	}
	return &dataBlob{cbData: uint32(len(b)), pbData: &b[0]}
}

func dpapi(proc *syscall.LazyProc, data []byte) ([]byte, error) {
	var out dataBlob
	in := blobOf(data)
	// last-but-one argument 1 = CRYPTPROTECT_UI_FORBIDDEN
	r, _, callErr := proc.Call(uintptr(unsafe.Pointer(in)), 0, 0, 0, 0, 1, uintptr(unsafe.Pointer(&out)))
	runtime.KeepAlive(data)
	if r == 0 {
		if callErr == nil || callErr == syscall.Errno(0) {
			callErr = errors.New("Windows data protection failed")
		}
		return nil, callErr
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	return append([]byte(nil), unsafe.Slice(out.pbData, int(out.cbData))...), nil
}

// sealString returns "" when sealing fails, so nothing is ever stored unprotected.
func sealString(s string) string {
	if s == "" {
		return ""
	}
	b, err := dpapi(procProtect, []byte(s))
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(b)
}

func openString(enc string) string {
	if enc == "" {
		return ""
	}
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return ""
	}
	b, err := dpapi(procUnprotect, raw)
	if err != nil {
		return ""
	}
	return string(b)
}
