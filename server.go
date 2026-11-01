package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

// The browser extension talks to the app over this local-only address.
const apiAddr = "127.0.0.1:19345"

var appLanguage atomic.Value

func setAppLanguage(lang string) {
	if lang != "ar" && lang != "fr" {
		lang = "en"
	}
	appLanguage.Store(lang)
}

func getAppLanguage() string {
	v := appLanguage.Load()
	if lang, ok := v.(string); ok && (lang == "en" || lang == "ar" || lang == "fr") {
		return lang
	}
	return "en"
}

type addReq struct {
	URL         string      `json:"url"`
	Referrer    string      `json:"referrer"`
	Cookie      string      `json:"cookie"`
	Kind        string      `json:"kind"`    // "video", "audio", or "torrent"; empty = normal file
	Cookies     []cookieRec `json:"cookies"` // login cookies for video pages (optional)
	Action      string      `json:"action"`  // download or queue
	Name        string      `json:"name"`
	Size        int64       `json:"size"`
	AudioOnly   bool        `json:"audioOnly"`
	AudioFormat string      `json:"audioFormat"`
}

// startServer listens on 127.0.0.1 only. A web page cannot use it:
// requests must come from a browser extension (Origin check), carry a
// custom header (which forces a CORS preflight), and use our own Host.
func startServer(onAdd func(addReq) error, onShow func()) {
	guard := func(w http.ResponseWriter, r *http.Request) bool {
		if r.Host != apiAddr && r.Host != "localhost:19345" {
			http.Error(w, "bad host", http.StatusForbidden)
			return false
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			if !strings.HasPrefix(origin, "chrome-extension://") && !strings.HasPrefix(origin, "moz-extension://") {
				http.Error(w, "forbidden", http.StatusForbidden)
				return false
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Sabo-DM")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return false
		}
		return true
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		if guard(w, r) {
			w.Write([]byte("sabo-dm"))
		}
	})
	mux.HandleFunc("/capabilities", func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		_, ffErr := ffmpegReady()
		_, ytErr := os.Stat(findTool("yt-dlp.exe"))
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"app": "Sabo DM", "version": AppVersion,
			"ffmpegReady": ffErr == nil, "ytDlpReady": ytErr == nil,
			"audioFormats": []string{"original", "mp3", "m4a", "aac", "opus", "wav", "flac"},
		})
	})
	mux.HandleFunc("/settings", func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]string{"language": getAppLanguage()})
	})
	mux.HandleFunc("/show", func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		if r.Method != http.MethodPost || r.Header.Get("X-Sabo-DM") != "1" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		onShow()
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/add", func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		if r.Method != http.MethodPost || r.Header.Get("X-Sabo-DM") != "1" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		var req addReq
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		req.URL = strings.TrimSpace(req.URL)
		if len(req.Cookies) > 3000 {
			req.Cookies = req.Cookies[:3000]
		}
		if !strings.HasPrefix(req.URL, "http://") && !strings.HasPrefix(req.URL, "https://") &&
			!strings.HasPrefix(strings.ToLower(req.URL), "magnet:") {
			http.Error(w, "bad url", http.StatusBadRequest)
			return
		}
		if req.Kind == "" {
			if isTorrentLink(req.URL) {
				req.Kind = "torrent"
			} else if isVideoSite(req.URL) {
				req.Kind = "video"
			}
		}
		if req.Kind == "" {
			// Only say "ok" if we can really fetch it, so the browser keeps
			// its own download when we cannot take over.
			var hdr http.Header
			if req.Referrer != "" || req.Cookie != "" {
				hdr = http.Header{}
				if req.Referrer != "" {
					hdr.Set("Referer", req.Referrer)
				}
				if req.Cookie != "" {
					hdr.Set("Cookie", req.Cookie)
				}
			}
			ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
			defer cancel()
			if err := checkDirect(ctx, req.URL, hdr); err != nil {
				http.Error(w, err.Error(), http.StatusUnprocessableEntity)
				return
			}
		}
		if err := onAdd(req); err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"ok":true,"accepted":true}`))
	})

	srv := &http.Server{Addr: apiAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() { debugf("api server stopped: %v", srv.ListenAndServe()) }()
}

// existingInstance returns true (after asking it to show itself) when
// another copy of the app is already running.
func existingInstance(show bool) bool {
	c := &http.Client{Timeout: 800 * time.Millisecond}
	resp, err := c.Get("http://" + apiAddr + "/ping")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 32))
	if string(b) != "sabo-dm" {
		return false
	}
	if !show {
		return true
	}
	if req, err := http.NewRequest("POST", "http://"+apiAddr+"/show", nil); err == nil {
		req.Header.Set("X-Sabo-DM", "1")
		if r2, err := c.Do(req); err == nil {
			r2.Body.Close()
		}
	}
	return true
}
