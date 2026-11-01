package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// startDashboardServer exposes a loopback-only JSON API for the Sabo DM dashboard.
// Queue access is always marshalled onto the Windows UI thread by dispatch.
const dashboardAddr = "127.0.0.1:19346"

//go:embed ui-preview
var dashboardFiles embed.FS

type dashboardItem struct {
	Index    int                `json:"index"`
	Name     string             `json:"name"`
	URL      string             `json:"url"`
	Kind     string             `json:"kind"`
	Status   string             `json:"status"`
	Done     int64              `json:"done"`
	Total    int64              `json:"total"`
	Speed    float64            `json:"speed"`
	Percent  float64            `json:"percent"`
	Segments []dashboardSegment `json:"segments"`
}

type dashboardSegment struct {
	Done  int64   `json:"done"`
	Total int64   `json:"total"`
	Pct   float64 `json:"pct"`
}

type dashboardAction struct {
	Action string `json:"action"`
	Index  int    `json:"index"`
	URL    string `json:"url"`
}

func startDashboardServer(q *Queue, saveDir string, dispatch func(func())) {
	mux := http.NewServeMux()
	guard := func(w http.ResponseWriter, r *http.Request) bool {
		if r.Host != dashboardAddr && r.Host != "localhost:19346" {
			http.Error(w, "invalid host", http.StatusForbidden)
			return false
		}
		origin := r.Header.Get("Origin")
		if origin != "" && origin != "http://127.0.0.1:19346" && origin != "http://localhost:19346" {
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return false
		}
		w.Header().Set("Access-Control-Allow-Origin", "http://127.0.0.1:19346")
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		return true
	}
	mux.HandleFunc("/api/downloads", func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", 405)
			return
		}
		ch := make(chan []dashboardItem, 1)
		dispatch(func() {
			items := make([]dashboardItem, 0, len(q.Items))
			for i, it := range q.Items {
				name := displayName(it)
				pct := 0.0
				if it.Total > 0 {
					pct = float64(it.Done) * 100 / float64(it.Total)
				}
				segments := make([]dashboardSegment, 0, len(it.Segments))
				for _, seg := range it.Segments {
					total := seg.End - seg.Start + 1
					segmentPct := 0.0
					if total > 0 {
						segmentPct = float64(seg.Done) * 100 / float64(total)
					}
					segments = append(segments, dashboardSegment{Done: seg.Done, Total: total, Pct: segmentPct})
				}
				items = append(items, dashboardItem{Index: i, Name: name, URL: it.URL, Kind: it.Kind, Status: it.Status, Done: it.Done, Total: it.Total, Speed: it.Speed, Percent: pct, Segments: segments})
			}
			ch <- items
		})
		select {
		case items := <-ch:
			_ = json.NewEncoder(w).Encode(items)
		case <-time.After(3 * time.Second):
			http.Error(w, "UI thread timeout", 503)
		}
	})
	mux.HandleFunc("/api/action", func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", 405)
			return
		}
		var a dashboardAction
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&a); err != nil {
			http.Error(w, "invalid JSON", 400)
			return
		}
		if a.Action != "add" && a.Action != "start" && a.Action != "pause" && a.Action != "remove" && a.Action != "clear-done" && a.Action != "set-speed" {
			http.Error(w, "unsupported action", 400)
			return
		}
		done := make(chan error, 1)
		dispatch(func() {
			var err error
			switch a.Action {
			case "add":
				u := strings.TrimSpace(a.URL)
				if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") && !strings.HasPrefix(strings.ToLower(u), "magnet:") {
					err = fmt.Errorf("enter a valid HTTP, HTTPS, or magnet URL")
				} else {
					q.AddExt(u, saveDir, "", "")
				}
			case "start", "pause":
				if a.Index < 0 || a.Index >= len(q.Items) {
					err = fmt.Errorf("download no longer exists")
				} else if a.Action == "start" {
					q.Start(q.Items[a.Index])
				} else {
					q.Pause(q.Items[a.Index])
				}
			case "remove":
				q.Remove([]int{a.Index})
			case "clear-done":
				q.ClearDone()
			case "set-speed":
				if a.Index < 0 {
					err = fmt.Errorf("speed limit must be zero or greater")
				} else {
					setSpeedLimit(int64(a.Index) * 1024)
				}
			}
			done <- err
		})
		select {
		case err := <-done:
			if err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		case <-time.After(3 * time.Second):
			http.Error(w, "UI thread timeout", 503)
		}
	})
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		_, _ = w.Write([]byte(`{"status":"connected","app":"Sabo DM"}`))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/dashboard/", http.StatusTemporaryRedirect)
	})
	staticFiles, _ := fs.Sub(dashboardFiles, "ui-preview")
	mux.Handle("/dashboard/", http.StripPrefix("/dashboard/", http.FileServer(http.FS(staticFiles))))
	srv := &http.Server{Addr: dashboardAddr, Handler: mux, ReadHeaderTimeout: 4 * time.Second}
	go func() { debugf("dashboard API stopped: %v", srv.ListenAndServe()) }()
}

// parseDashboardIndex accepts only non-negative decimal row indexes.
func parseDashboardIndex(s string) (int, error) {
	i, err := strconv.Atoi(s)
	if err != nil || i < 0 {
		return 0, fmt.Errorf("invalid index")
	}
	return i, nil
}
