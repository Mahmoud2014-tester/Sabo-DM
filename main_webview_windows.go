//go:build windows

package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	webview2 "github.com/jchv/go-webview2"
)

func main() {
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
		cfg.Parallel = 3
	}
	if cfg.MaxSegments < 1 || cfg.MaxSegments > 32 {
		cfg.MaxSegments = 16
	}

	wv := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     false,
		DataPath:  filepath.Join(configDir(), "WebView2"),
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  "Sabo DM",
			Width:  1440,
			Height: 900,
			Center: true,
		},
	})
	if wv == nil {
		fmt.Println("Could not initialize Microsoft Edge WebView2. Install the Microsoft Edge WebView2 Runtime and try again.")
		return
	}
	defer wv.Destroy()

	// Expose real Windows window-management actions to the dashboard.
	hwnd := uintptr(wv.Window())
	user32 := syscall.NewLazyDLL("user32.dll")
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
	_ = wv.Bind("saboClose", func() {
		wv.Dispatch(func() { wv.Destroy() })
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

	startDashboardServer(q, saveDir, wv.Dispatch, func() { wv.Dispatch(func() { wv.Destroy() }) })
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
	})

	if cfg.AutoStart {
		go setAutostart(true)
	}

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
	wv.Run()

	q.CancelAll()
	q.save()
	q.close()
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
