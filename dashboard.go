package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// startDashboardServer exposes a loopback-only JSON API for the Sabo DM dashboard.
// Queue access is always marshalled onto the Windows UI thread by dispatch.
const dashboardAddr = "127.0.0.1:19346"

//go:embed ui-preview
var dashboardFiles embed.FS

type dashboardItem struct {
	Index     int                `json:"index"`
	Name      string             `json:"name"`
	URL       string             `json:"url"`
	Kind      string             `json:"kind"`
	Status    string             `json:"status"`
	Done      int64              `json:"done"`
	Total     int64              `json:"total"`
	Speed     float64            `json:"speed"`
	Percent   float64            `json:"percent"`
	Segments  []dashboardSegment `json:"segments"`
	Queue     string             `json:"queue"`
	Category  string             `json:"category"`
	Path      string             `json:"path"`
	Directory string             `json:"directory"`
}

type dashboardSegment struct {
	Done  int64   `json:"done"`
	Total int64   `json:"total"`
	Pct   float64 `json:"pct"`
}

type dashboardAction struct {
	Action      string `json:"action"`
	Index       int    `json:"index"`
	URL         string `json:"url"`
	Name        string `json:"name"`
	Queue       string `json:"queue"`
	AudioOnly   bool   `json:"audioOnly"`
	AudioFormat string `json:"audioFormat"`
}

type toolJob struct {
	Name       string    `json:"name"`
	State      string    `json:"state"`
	Error      string    `json:"error,omitempty"`
	StartedAt  time.Time `json:"startedAt,omitempty"`
	FinishedAt time.Time `json:"finishedAt,omitempty"`
}

var toolJobMu sync.Mutex
var activeToolJob toolJob

func launchToolJob(name string, work func() error) bool {
	toolJobMu.Lock()
	if activeToolJob.State == "running" {
		toolJobMu.Unlock()
		return false
	}
	activeToolJob = toolJob{Name: name, State: "running", StartedAt: time.Now()}
	toolJobMu.Unlock()
	go func() {
		err := work()
		toolJobMu.Lock()
		activeToolJob.FinishedAt = time.Now()
		if err != nil {
			activeToolJob.State = "failed"
			activeToolJob.Error = err.Error()
		} else {
			activeToolJob.State = "completed"
			activeToolJob.Error = ""
		}
		toolJobMu.Unlock()
	}()
	return true
}

func startDashboardServer(q *Queue, saveDir string, dispatch func(func()), shutdown func()) {
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
				items = append(items, dashboardItem{Index: i, Name: name, URL: it.URL, Kind: it.Kind, Status: it.Status, Done: it.Done, Total: it.Total, Speed: it.Speed, Percent: pct, Segments: segments, Queue: it.Queue, Category: it.Category, Path: it.Path, Directory: it.Dir})
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
		if a.Action != "add" && a.Action != "start" && a.Action != "pause" && a.Action != "remove" && a.Action != "clear-done" && a.Action != "set-speed" && a.Action != "add-queue" && a.Action != "delete-queue" && a.Action != "set-queue" && a.Action != "open-file" && a.Action != "open-folder" {
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
				} else if isTorrentLink(u) {
					q.AddTorrent(u, saveDir, "", "")
				} else if isVideoSite(u) {
					if findTool("yt-dlp.exe") == "" {
						err = fmt.Errorf("yt-dlp is not installed. Open Settings and install yt-dlp first")
					} else {
						label := "Best available"
						args := bestArgs(findFFmpegDir() != "")
						if a.AudioOnly {
							format := a.AudioFormat
							if format == "" {
								format = "original"
							}
							args, err = audioFormatArgs(format)
							label = "Audio (" + format + ")"
						}
						if err == nil {
							q.AddVideoWithReferrer(u, saveDir, "", label, args, "", "")
						}
					}
				} else {
					q.AddExt(u, saveDir, "", "")
				}
				if err == nil && a.Queue != "" {
					q.assignQueueForURL(u, a.Queue)
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
			case "add-queue":
				name := strings.TrimSpace(a.Name)
				if name == "" || len(name) > 40 {
					err = fmt.Errorf("queue name must contain 1–40 characters")
				} else {
					for _, existing := range q.Queues {
						if strings.EqualFold(existing, name) {
							err = fmt.Errorf("queue already exists")
							break
						}
					}
					if err == nil {
						names := append(append([]string(nil), q.Queues...), name)
						q.setQueueNames(names)
					}
				}
			case "delete-queue":
				name := strings.TrimSpace(a.Queue)
				if strings.EqualFold(name, "Default") {
					err = fmt.Errorf("the Default queue cannot be deleted")
				} else {
					for _, item := range q.Items {
						if item.Queue == name {
							item.Queue = "Default"
						}
					}
					names := make([]string, 0, len(q.Queues))
					for _, existing := range q.Queues {
						if existing != name {
							names = append(names, existing)
						}
					}
					q.setQueueNames(names)
					q.save()
					q.notify()
				}
			case "set-queue":
				if a.Index < 0 || a.Index >= len(q.Items) {
					err = fmt.Errorf("download no longer exists")
				} else {
					found := false
					for _, name := range q.Queues {
						if name == a.Queue {
							found = true
							break
						}
					}
					if !found {
						err = fmt.Errorf("queue does not exist")
					} else {
						q.Items[a.Index].Queue = a.Queue
						q.save()
						q.notify()
					}
				}
			case "open-file":
				if a.Index < 0 || a.Index >= len(q.Items) || q.Items[a.Index].Path == "" {
					err = fmt.Errorf("the file is not available yet")
				} else {
					openFile(q.Items[a.Index].Path)
				}
			case "open-folder":
				if a.Index < 0 || a.Index >= len(q.Items) {
					err = fmt.Errorf("download no longer exists")
				} else {
					err = openFolder(q.Items[a.Index].Dir)
				}
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
	mux.HandleFunc("/api/meta", func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		ch := make(chan map[string]interface{}, 1)
		dispatch(func() {
			queues := q.Queues
			if len(queues) == 0 {
				queues = []string{"Default"}
			}
			cats := []string{"Files", "Videos", "Music", "Documents", "Torrents"}
			ch <- map[string]interface{}{"queues": queues, "categories": cats, "downloadsAtOnce": q.MaxParallel, "version": AppVersion}
		})
		select {
		case meta := <-ch:
			_ = json.NewEncoder(w).Encode(meta)
		case <-time.After(3 * time.Second):
			http.Error(w, "UI thread timeout", 503)
		}
	})
	mux.HandleFunc("/api/capabilities", func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		_, ffErr := ffmpegReady()
		yt := findTool("yt-dlp.exe")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"version": AppVersion, "ffmpegReady": ffErr == nil, "ytDlpReady": yt != "", "downloadDirectory": saveDir})
	})
	mux.HandleFunc("/api/tools/status", func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		toolJobMu.Lock()
		snapshot := activeToolJob
		toolJobMu.Unlock()
		_ = json.NewEncoder(w).Encode(snapshot)
	})
	mux.HandleFunc("/api/tools/install-ffmpeg", func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", 405)
			return
		}
		if !launchToolJob("Install FFmpeg", func() error { return downloadFFmpeg(context.Background(), nil) }) {
			http.Error(w, "another tool operation is already running", http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"ok":true,"state":"running"}`))
	})
	mux.HandleFunc("/api/tools/update-ytdlp", func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", 405)
			return
		}
		if !launchToolJob("Update yt-dlp", func() error {
			if findTool("yt-dlp.exe") == "" {
				return downloadYtDlp(context.Background(), nil)
			}
			_, err := updateYtDlp()
			return err
		}) {
			http.Error(w, "another tool operation is already running", http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"ok":true,"state":"running"}`))
	})
	mux.HandleFunc("/api/update/check", func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", 405)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		info, err := checkForUpdate(ctx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		_ = json.NewEncoder(w).Encode(info)
	})
	mux.HandleFunc("/api/update/install", func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", 405)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 12*time.Minute)
		defer cancel()
		staged, tag, err := stageLatestUpdate(ctx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		if err := scheduleSelfUpdate(staged); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "latestVersion": tag, "restarting": true})
		if shutdown != nil {
			time.AfterFunc(900*time.Millisecond, shutdown)
		}
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
