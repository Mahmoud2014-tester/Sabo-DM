package main

import (
	"syscall"
	"unsafe"
)

// Colours for the "current download" bar. A standard Windows progress bar follows
// the system theme, so the theme is switched off for this one bar to allow red/green.

var (
	modUxtheme       = syscall.NewLazyDLL("uxtheme.dll")
	modUser32        = syscall.NewLazyDLL("user32.dll")
	procSetWinTheme  = modUxtheme.NewProc("SetWindowTheme")
	procSendMessage  = modUser32.NewProc("SendMessageW")
	procGetWLP       = modUser32.NewProc("GetWindowLongPtrW")
	procSetWLP       = modUser32.NewProc("SetWindowLongPtrW")
	procSetWindowPos = modUser32.NewProc("SetWindowPos")
)

func rgbRef(r, g, b uint8) uint32 { return uint32(r) | uint32(g)<<8 | uint32(b)<<16 }

var (
	colRed    = rgbRef(220, 38, 38)
	colGreen  = rgbRef(22, 163, 74)
	colGray   = rgbRef(148, 163, 184)
	colOrange = rgbRef(234, 88, 12)
	colTrack  = rgbRef(226, 232, 240)
)

// initBar prepares a progress bar for custom colours (smooth bar, no theme, light track).
func initBar(hwnd uintptr) {
	defer func() { recover() }()
	empty, _ := syscall.UTF16PtrFromString("")
	procSetWinTheme.Call(hwnd, uintptr(unsafe.Pointer(empty)), uintptr(unsafe.Pointer(empty)))
	gwlStyle := ^uintptr(15) // GWL_STYLE (-16)
	style, _, _ := procGetWLP.Call(hwnd, gwlStyle)
	procSetWLP.Call(hwnd, gwlStyle, style|0x01)              // PBS_SMOOTH
	procSendMessage.Call(hwnd, 0x2001, 0, uintptr(colTrack)) // PBM_SETBKCOLOR
}

func setBarColor(hwnd uintptr, c uint32) {
	defer func() { recover() }()
	procSendMessage.Call(hwnd, 0x0409, 0, uintptr(c)) // PBM_SETBARCOLOR
}

// ensureStandardWindowFrame restores the normal resizable Windows frame.
// This guarantees the native title bar and its minimize, maximize/restore,
// and close buttons remain available even if a prior build altered the style.
func ensureStandardWindowFrame(hwnd uintptr) {
	defer func() { _ = recover() }()
	const (
		gwlStyle        = ^uintptr(15) // -16
		wsCaption       = uintptr(0x00C00000)
		wsSysMenu       = uintptr(0x00080000)
		wsMinimizeBox   = uintptr(0x00020000)
		wsMaximizeBox   = uintptr(0x00010000)
		wsThickFrame    = uintptr(0x00040000)
		swpNoSize       = uintptr(0x0001)
		swpNoMove       = uintptr(0x0002)
		swpNoZOrder     = uintptr(0x0004)
		swpNoActivate   = uintptr(0x0010)
		swpFrameChanged = uintptr(0x0020)
	)
	style, _, _ := procGetWLP.Call(hwnd, gwlStyle)
	// A popup/child style suppresses the standard non-client title bar.
	style &^= uintptr(0x80000000 | 0x40000000) // WS_POPUP | WS_CHILD
	style |= wsCaption | wsSysMenu | wsMinimizeBox | wsMaximizeBox | wsThickFrame
	procSetWLP.Call(hwnd, gwlStyle, style)
	procSetWindowPos.Call(hwnd, 0, 0, 0, 0, 0, swpNoSize|swpNoMove|swpNoZOrder|swpNoActivate|swpFrameChanged)
}
