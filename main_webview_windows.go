//go:build windows

package main

import (
	_ "embed"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/getlantern/systray"
	"time"

	webview2 "github.com/jchv/go-webview2"
)

//go:embed sabo.ico
var saboTrayIcon []byte

func main() {
	// Declare per-monitor DPI awareness before creating any native window.
	// This prevents Windows from scaling the client area independently of the
	// non-client title bar, which can clip the native caption buttons.
	setProcessDPIAwareness()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if existingInstance(true) {
		return
	}

	cfg := loadSettings()
	saveDir := cfg.Dir
	if st, err := os.Stat(saveDir); saveDir == "" || err != nil || !st.IsDir() {
		saveDir = defaultDownloadDir()
	}
	if cfg.Parallel < 1 || cfg.Parallel > 8 {
		cfg.Parallel = 5
	}
	if cfg.MaxSegments < 1 || cfg.MaxSegments > 32 {
		cfg.MaxSegments = 16
	}

	windowWidth, windowHeight := initialWindowSize()
	wv := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     false,
		DataPath:  filepath.Join(configDir(), "WebView2"),
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  "Sabo DM",
			Width:  windowWidth,
			Height: windowHeight,
			Center: true,
		},
	})
	if wv == nil {
		fmt.Println("Could not initialize Microsoft Edge WebView2. Install the Microsoft Edge WebView2 Runtime and try again.")
		return
	}

	// Expose real Windows window-management actions to the dashboard.
	hwnd := uintptr(wv.Window())
	user32 := syscall.NewLazyDLL("user32.dll")
	ensureNativeWindowFrame(hwnd, user32)
	showWindow := user32.NewProc("ShowWindow")
	isZoomed := user32.NewProc("IsZoomed")
	_ = wv.Bind("saboMinimize", func() {
		wv.Dispatch(func() { showWindow.Call(hwnd, 6) }) // SW_MINIMIZE
	})
	_ = wv.Bind("saboToggleMaximize", func() {
		wv.Dispatch(func() {
			zoomed, _, _ := isZoomed.Call(hwnd)
			if zoomed != 0 {
				showWindow.Call(hwnd, 9) // SW_RESTORE
			} else {
				showWindow.Call(hwnd, 3) // SW_MAXIMIZE
			}
		})
	})
	// The title-bar X hides the window but deliberately leaves the engine alive.
	installCloseToHide(hwnd, user32)
	_ = wv.Bind("saboClose", func() {
		wv.Dispatch(func() { showWindow.Call(hwnd, 0) }) // Hide, do not terminate.
	})

	q := &Queue{MaxParallel: cfg.Parallel, MaxSegments: cfg.MaxSegments, Queues: cfg.Queues}
	q.post = wv.Dispatch
	q.load()
	setSpeedLimit(int64(cfg.LimitKB) * 1024)
	if cfg.AutoResume {
		time.AfterFunc(2*time.Second, func() {
			wv.Dispatch(func() {
				for _, item := range q.Items {
					if item.Status == stPaused {
						q.Start(item)
					}
				}
			})
		})
	}
	browserSetting.Store(cfg.CookieBrowser)
	setAppLanguage(cfg.Language)

	startDashboardServer(q, saveDir, wv.Dispatch, func() { wv.Dispatch(func() { showWindow.Call(hwnd, 0) }) })
	startServer(func(r addReq) error {
		done := make(chan error, 1)
		wv.Dispatch(func() {
			var err error
			switch r.Kind {
			case "video", "audio":
				label := "Best available"
				args := bestArgs(findFFmpegDir() != "")
				if r.Kind == "audio" || r.AudioOnly {
					format := r.AudioFormat
					if format == "" {
						format = "original"
					}
					args, err = audioFormatArgs(format)
					label = "Audio (" + format + ")"
				}
				if err == nil {
					if findTool("yt-dlp.exe") == "" {
						err = fmt.Errorf("yt-dlp is not installed. Open Settings → Install / Update yt-dlp and retry")
					} else if r.Action == "queue" {
						q.AddVideoQueuedWithReferrer(r.URL, saveDir, r.Name, label, args, netscapeJar(r.Cookies), r.Referrer)
					} else {
						q.AddVideoWithReferrer(r.URL, saveDir, r.Name, label, args, netscapeJar(r.Cookies), r.Referrer)
					}
				}
			case "torrent":
				if r.Action == "queue" {
					q.AddTorrentQueued(r.URL, saveDir, r.Name, "")
				} else {
					q.AddTorrent(r.URL, saveDir, r.Name, "")
				}
			default:
				if r.Action == "queue" {
					q.AddExtQueuedNamed(r.URL, saveDir, r.Referrer, r.Cookie, r.Name)
				} else {
					q.AddExtNamed(r.URL, saveDir, r.Referrer, r.Cookie, r.Name)
				}
			}
			if err == nil {
				showAppWindow(hwnd, user32)
			}
			done <- err
		})
		select {
		case err := <-done:
			return err
		case <-time.After(10 * time.Second):
			return fmt.Errorf("Sabo DM did not finish accepting the browser request")
		}

	}, func() {
		showAppWindow(hwnd, user32)
	}, func(url string) map[string]interface{} {
		ch := make(chan map[string]interface{}, 1)
		wv.Dispatch(func() {
			for i := len(q.Items) - 1; i >= 0; i-- {
				it := q.Items[i]
				if it.URL == url {
					pct := 0.0
					if it.Total > 0 {
						pct = float64(it.Done) * 100 / float64(it.Total)
					}
					ch <- map[string]interface{}{"name": displayName(it), "status": it.Status, "done": it.Done, "total": it.Total, "speed": it.Speed, "percent": pct}
					return
				}
			}
			ch <- map[string]interface{}{"name": "Download", "status": "Queued", "done": int64(0), "total": int64(0), "speed": float64(0), "percent": float64(0)}
		})
		select {
		case result := <-ch:
			return result
		case <-time.After(2 * time.Second):
			return map[string]interface{}{"status": "Checking", "percent": float64(0)}
		}
	})

	if cfg.AutoStart {
		go setAutostart(true)
	}

	// System-tray Exit is the only full application exit. The tray's Open action
	// restores the WebView2 window; hiding the window never stops downloads.
	go runSystemTray(func() { showAppWindow(hwnd, user32) }, func() {
		wv.Dispatch(func() {
			q.CancelAll()
			q.save()
			wv.Destroy()
		})
	}, func() {
		wv.Dispatch(func() {
			for _, item := range q.Items {
				if item.running {
					q.Pause(item)
				} else if item.Status == stPaused {
					q.Start(item)
				}
			}
		})
	}, func() {
		wv.Dispatch(func() {
			for _, item := range q.Items {
				if item.Status == stPaused {
					q.Start(item)
				}
			}
		})
	})

	// Wait briefly for the loopback dashboard server before navigating WebView2.
	health := &http.Client{Timeout: 250 * time.Millisecond}
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := health.Get("http://127.0.0.1:19346/api/health")
		if err == nil {
			resp.Body.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	wv.Navigate("http://127.0.0.1:19346/dashboard/")
	for _, arg := range os.Args[1:] {
		if arg == "--background" {
			showWindow.Call(hwnd, 0)
			break
		}
	}
	wv.Run()
	q.CancelAll()
	q.save()
	q.close()
	wv.Destroy()
}

// setProcessDPIAwareness must run before WebView2 creates a window. Per-monitor
// v2 keeps Windows' native title bar and the WebView client area on the same DPI.
func setProcessDPIAwareness() {
	const dpiAwarenessContextPerMonitorAwareV2 = ^uintptr(3) // (HANDLE)-4
	user32 := syscall.NewLazyDLL("user32.dll")
	if proc := user32.NewProc("SetProcessDpiAwarenessContext"); proc.Find() == nil {
		ret, _, _ := proc.Call(dpiAwarenessContextPerMonitorAwareV2)
		if ret != 0 {
			return
		}
	}
	// Fallback for older Windows versions where the per-monitor v2 API is absent.
	if proc := user32.NewProc("SetProcessDPIAware"); proc.Find() == nil {
		proc.Call()
	}
}

// initialWindowSize leaves room for the complete native title bar and taskbar
// on smaller displays instead of creating a fixed 1440x900 window off-screen.
func initialWindowSize() (int, int) {
	user32 := syscall.NewLazyDLL("user32.dll")
	get := user32.NewProc("GetSystemMetrics")
	screenW, _, _ := get.Call(0) // SM_CXSCREEN
	screenH, _, _ := get.Call(1) // SM_CYSCREEN
	w, h := 1440, 900
	if screenW > 0 && w > int(screenW*9/10) {
		w = int(screenW * 9 / 10)
	}
	if screenH > 0 && h > int(screenH*85/100) {
		h = int(screenH * 85 / 100)
	}
	if w < 900 {
		w = 900
	}
	if h < 600 {
		h = 600
	}
	return w, h
}

// ensureNativeWindowFrame explicitly retains the standard Windows caption,
// system menu, resize border, and Minimize/Maximize buttons. It then asks
// Windows to recalculate the non-client frame after DPI awareness is applied.
func ensureNativeWindowFrame(hwnd uintptr, user32 *syscall.LazyDLL) {
	const (
		gwlStyle        = ^uintptr(15) // -16
		wsOverlapped    = uintptr(0x00CF0000)
		swpNoSize       = uintptr(0x0001)
		swpNoMove       = uintptr(0x0002)
		swpNoZOrder     = uintptr(0x0004)
		swpNoActivate   = uintptr(0x0010)
		swpFrameChanged = uintptr(0x0020)
	)
	getStyle := user32.NewProc("GetWindowLongPtrW")
	setStyle := user32.NewProc("SetWindowLongPtrW")
	style, _, _ := getStyle.Call(hwnd, gwlStyle)
	setStyle.Call(hwnd, gwlStyle, style|wsOverlapped)
	user32.NewProc("SetWindowPos").Call(
		hwnd, 0, 0, 0, 0, 0,
		swpNoSize|swpNoMove|swpNoZOrder|swpNoActivate|swpFrameChanged,
	)
}

func showAppWindow(hwnd uintptr, user32 *syscall.LazyDLL) {
	user32.NewProc("ShowWindow").Call(hwnd, 9) // SW_RESTORE
	user32.NewProc("SetForegroundWindow").Call(hwnd)
}

func defaultDownloadDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		dir := filepath.Join(home, "Downloads")
		if st, err := os.Stat(dir); err == nil && st.IsDir() {
			return dir
		}
	}
	dir, _ := os.Getwd()
	return dir
}

// installCloseToHide subclasses the native WebView2 top-level window. WM_CLOSE
// from the title-bar X hides the window; it does not terminate the engine.
var windowProcCallback uintptr

func installCloseToHide(hwnd uintptr, user32 *syscall.LazyDLL) {
	const (
		gwlpWndProc = ^uintptr(3)
		wmClose     = 0x0010
		swHide      = 0
	)
	set := user32.NewProc("SetWindowLongPtrW")
	call := user32.NewProc("CallWindowProcW")
	show := user32.NewProc("ShowWindow")
	var oldProc uintptr
	windowProcCallback = syscall.NewCallback(func(h, msg, wp, lp uintptr) uintptr {
		if msg == wmClose {
			show.Call(h, swHide)
			return 0
		}
		ret, _, _ := call.Call(oldProc, h, msg, wp, lp)
		return ret
	})
	oldProc, _, _ = set.Call(hwnd, gwlpWndProc, windowProcCallback)
}

func runSystemTray(open, exit, pause, resume func()) {
	systray.Run(func() {
		systray.SetIcon(saboTrayIcon)
		systray.SetTitle("Sabo DM")
		systray.SetTooltip("Sabo DM — Download Manager")
		show := systray.AddMenuItem("Open Sabo DM", "Show the download manager")
		systray.AddSeparator()
		p := systray.AddMenuItem("Pause all downloads", "Pause active downloads")
		r := systray.AddMenuItem("Resume all downloads", "Resume paused downloads")
		systray.AddSeparator()
		quit := systray.AddMenuItem("Exit", "Close Sabo DM and stop all downloads")
		go func() {
			for {
				select {
				case <-show.ClickedCh:
					open()
				case <-p.ClickedCh:
					pause()
				case <-r.ClickedCh:
					resume()
				case <-quit.ClickedCh:
					exit()
					systray.Quit()
					return
				}
			}
		}()
	}, func() {})
}
