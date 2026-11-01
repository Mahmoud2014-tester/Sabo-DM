package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lxn/walk"
)

const (
	stQueued      = "Queued"
	stDownloading = "Downloading"
	stPaused      = "Paused"
	stDone        = "Done"

	kindFile    = "file"
	kindVideo   = "video"
	kindTorrent = "torrent"
)

// Item is one row in the download list.
type Item struct {
	URL, Dir, Name string
	Path           string // final file, once known
	Queue          string // user-defined queue name
	Category       string // derived file category
	Kind           string // kindFile (default) or kindVideo
	VLabel         string // chosen video quality, for display
	VArgs          []string
	Select         string // torrent: chosen file numbers, empty = all files
	Jar            string // video login cookies (cookies.txt text); kept encrypted on disk
	Status         string
	Done, Total    int64
	Speed          float64
	Segments       []Segment // live direct-file segment progress; not persisted separately
	Referer        string    // from the browser, memory only
	Cookie         string    // from the browser, memory only (never saved to disk)

	cancel  context.CancelFunc
	running bool
}

// Queue is both the download manager and the table model for the GUI.
// All of its methods must be called on the UI thread; background work
// reports back through q.post (which wraps mw.Synchronize).
type Queue struct {
	walk.TableModelBase
	Items            []*Item
	Queues           []string
	MaxParallel      int
	MaxSegments      int // maximum direct-download range workers (1..32)
	post             func(func())
	onDone           func(*Item)
	onChange         func() // called whenever the list changes (for the summary line)
	store            *queueStore
	lastProgressSave time.Time
}

func bar(p float64) string {
	const w = 10
	n := int(p/100*w + 0.5)
	if n < 0 {
		n = 0
	}
	if n > w {
		n = w
	}
	return strings.Repeat("█", n) + strings.Repeat("░", w-n) + fmt.Sprintf(" %.0f%%", p)
}

func (q *Queue) RowCount() int { return len(q.Items) }

func (q *Queue) Value(row, col int) interface{} {
	if row < 0 || row >= len(q.Items) {
		return nil
	}
	it := q.Items[row]
	switch col {
	case 0:
		name := displayName(it)
		if it.Kind == kindVideo && it.VLabel != "" {
			return name + "  [" + it.VLabel + "]"
		}
		return name
	case 1:
		if it.Total > 0 {
			return humanBytes(float64(it.Total))
		}
		return "-"
	case 2:
		if it.Status == stDone {
			return bar(100)
		}
		if it.Total > 0 {
			return bar(float64(it.Done) * 100 / float64(it.Total))
		}
		return "-"
	case 3:
		if it.running && it.Speed > 0 {
			return humanBytes(it.Speed) + "/s"
		}
		return ""
	case 4:
		return it.Status
	}
	return nil
}

func (q *Queue) changed(it *Item) {
	for i, x := range q.Items {
		if x == it {
			q.PublishRowChanged(i)
			q.notify()
			return
		}
	}
}

// progressReporter coalesces high-frequency worker callbacks before they reach
// the Windows UI thread. This prevents a backlog of Synchronize calls and
// repeated native control invalidations while keeping progress responsive.
// Call flush once when the worker finishes so the final byte count is never lost.
func (q *Queue) progressReporter(it *Item) (report func(done, total int64, speed float64), flush func()) {
	var mu sync.Mutex
	var done, total int64
	var speed float64
	lastPost := time.Time{}
	haveValue := false
	const minInterval = 400 * time.Millisecond

	postLatest := func(force bool) {
		mu.Lock()
		now := time.Now()
		if !haveValue || (!force && !lastPost.IsZero() && now.Sub(lastPost) < minInterval) {
			mu.Unlock()
			return
		}
		d, t, sp := done, total, speed
		lastPost = now
		mu.Unlock()
		q.post(func() {
			if !it.running {
				return
			}
			it.Done, it.Total, it.Speed = d, t, sp
			it.Status = stDownloading
			q.changed(it)
			// Persist resumable progress every few seconds, not on every repaint.
			// The transfer engine flushes its segment map and file before reporting.
			if q.lastProgressSave.IsZero() || time.Since(q.lastProgressSave) >= 4*time.Second {
				q.lastProgressSave = time.Now()
				q.save()
			}
		})
	}

	report = func(d, t int64, sp float64) {
		mu.Lock()
		done, total, speed = d, t, sp
		haveValue = true
		mu.Unlock()
		postLatest(false)
	}
	flush = func() { postLatest(true) }
	return report, flush
}

func categoryFor(name, kind string) string {
	switch kind {
	case kindVideo:
		return "Videos"
	case kindTorrent:
		return "Torrents"
	}
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".mp3", ".m4a", ".aac", ".ogg", ".opus", ".wav", ".flac":
		return "Music"
	case ".mp4", ".mkv", ".webm", ".m4v", ".mov", ".avi":
		return "Videos"
	case ".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".txt", ".epub":
		return "Documents"
	default:
		return "Files"
	}
}

func (q *Queue) setQueueNames(names []string) {
	clean := make([]string, 0, len(names))
	seen := map[string]bool{}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || len(name) > 40 || seen[strings.ToLower(name)] {
			continue
		}
		seen[strings.ToLower(name)] = true
		clean = append(clean, name)
	}
	if !seen["default"] {
		clean = append([]string{"Default"}, clean...)
	}
	q.Queues = clean
	s := loadSettings()
	s.Queues = clean
	saveSettings(s)
}

func (q *Queue) assignQueueForURL(link, queue string) {
	queue = strings.TrimSpace(queue)
	if queue == "" {
		return
	}
	valid := false
	for _, name := range q.Queues {
		if name == queue {
			valid = true
			break
		}
	}
	if !valid {
		return
	}
	for _, item := range q.Items {
		if item.URL == link {
			item.Queue = queue
			q.save()
			q.notify()
			return
		}
	}
}

// displayName is what the list shows: the real name once known, otherwise a clear placeholder.
func displayName(it *Item) string {
	if it.Name != "" {
		return it.Name
	}
	if strings.HasPrefix(it.Status, "Error") {
		return it.URL
	}
	switch it.Kind {
	case kindVideo:
		return "Reading video info..."
	case kindTorrent:
		return "Torrent (reading info...)"
	}
	return "Getting file info..."
}

func (q *Queue) notify() {
	if q.onChange != nil {
		q.onChange()
	}
}

func (q *Queue) exists(link string) bool {
	for _, x := range q.Items {
		if x.URL == link && x.Status != stDone {
			return true
		}
	}
	return false
}

func (q *Queue) Add(link, dir string) { q.AddExt(link, dir, "", "") }

// AddExt also carries the browser's referrer and cookies (memory only).
func (q *Queue) AddExt(link, dir, referer, cookie string) {
	q.addExtNamed(link, dir, referer, cookie, "", true)
}

// AddExtNamed preserves the browser's decoded filename in the main list immediately.
func (q *Queue) AddExtNamed(link, dir, referer, cookie, name string) {
	q.addExtNamed(link, dir, referer, cookie, name, true)
}

// AddExtQueued inserts a browser download without starting it until the user chooses Start.
func (q *Queue) AddExtQueued(link, dir, referer, cookie string) {
	q.addExtNamed(link, dir, referer, cookie, "", false)
}

func (q *Queue) AddExtQueuedNamed(link, dir, referer, cookie, name string) {
	q.addExtNamed(link, dir, referer, cookie, name, false)
}

func (q *Queue) addExtNamed(link, dir, referer, cookie, name string, start bool) {
	for _, x := range q.Items {
		if x.URL == link && x.Status != stDone {
			// Sent again from the browser: refresh the login, and retry if it had stopped.
			if cookie != "" {
				x.Cookie = cookie
			}
			if referer != "" {
				x.Referer = referer
			}
			if x.Name == "" && name != "" {
				x.Name = name
				q.changed(x)
			}
			// A duplicate browser notification may refresh cookies/referrer, but must
			// never restart a user-paused item. Only the explicit Start button resumes it.
			q.save()
			return
		}
	}
	q.Items = append(q.Items, &Item{URL: link, Dir: dir, Name: name, Kind: kindFile, Referer: referer, Cookie: cookie, Queue: "Default", Category: categoryFor(name, kindFile), Status: stQueued})
	q.PublishRowsReset()
	q.notify()
	q.save()
	if start {
		q.schedule()
	}
}

func (q *Queue) AddVideo(link, dir, title, label string, args []string, jar string) {
	q.addVideo(link, dir, title, label, args, jar, "", true)
}
func (q *Queue) AddVideoQueued(link, dir, title, label string, args []string, jar string) {
	q.addVideo(link, dir, title, label, args, jar, "", false)
}
func (q *Queue) AddVideoWithReferrer(link, dir, title, label string, args []string, jar, referer string) {
	q.addVideo(link, dir, title, label, args, jar, referer, true)
}
func (q *Queue) AddVideoQueuedWithReferrer(link, dir, title, label string, args []string, jar, referer string) {
	q.addVideo(link, dir, title, label, args, jar, referer, false)
}
func (q *Queue) addVideo(link, dir, title, label string, args []string, jar, referer string, start bool) {
	for _, x := range q.Items {
		if x.URL == link && x.Status != stDone {
			if jar != "" {
				x.Jar = jar
			}
			if referer != "" {
				x.Referer = referer
			}
			// A duplicate browser notification may refresh cookies/referrer, but must
			// never restart a user-paused item. Only the explicit Start button resumes it.
			q.save()
			return
		}
	}
	q.Items = append(q.Items, &Item{URL: link, Dir: dir, Name: title, Kind: kindVideo, VLabel: label, VArgs: args, Jar: jar, Referer: referer, Queue: "Default", Category: categoryFor(title, kindVideo), Status: stQueued})
	q.PublishRowsReset()
	q.notify()
	q.save()
	if start {
		q.schedule()
	}
}

// AddTorrent takes a magnet link, a .torrent URL or a path to a .torrent file.
func (q *Queue) AddTorrent(src, dir, name, sel string) { q.addTorrent(src, dir, name, sel, true) }
func (q *Queue) AddTorrentQueued(src, dir, name, sel string) {
	q.addTorrent(src, dir, name, sel, false)
}
func (q *Queue) addTorrent(src, dir, name, sel string, start bool) {
	if q.exists(src) {
		return
	}
	q.Items = append(q.Items, &Item{URL: src, Dir: dir, Name: name, Select: sel, Kind: kindTorrent, Queue: "Default", Category: categoryFor(name, kindTorrent), Status: stQueued})
	q.PublishRowsReset()
	q.notify()
	q.save()
	if start {
		q.schedule()
	}
}

func (q *Queue) Start(it *Item) {
	if it.running || it.Status == stDone {
		return
	}
	it.Status = stQueued
	q.changed(it)
	q.save() // persist the user's resume request immediately
	q.schedule()
}

func (q *Queue) Pause(it *Item) {
	switch {
	case it.running:
		it.Status = "Pausing..."
		q.changed(it)
		if it.cancel != nil {
			it.cancel()
		}
	case it.Status == stQueued:
		it.Status = stPaused
		q.changed(it)
		q.save() // a queued item paused by the user must stay paused after restart
	}
}

// Remove deletes rows from the list. Downloaded files are NOT deleted.
func (q *Queue) Remove(indexes []int) {
	idx := append([]int(nil), indexes...)
	sort.Sort(sort.Reverse(sort.IntSlice(idx)))
	for _, i := range idx {
		if i < 0 || i >= len(q.Items) {
			continue
		}
		if it := q.Items[i]; it.running && it.cancel != nil {
			it.cancel()
		}
		q.Items = append(q.Items[:i], q.Items[i+1:]...)
	}
	q.PublishRowsReset()
	q.notify()
	q.save()
	q.schedule()
}

// ClearDone removes finished rows from the list (files stay on disk).
func (q *Queue) ClearDone() {
	kept := q.Items[:0]
	for _, it := range q.Items {
		if it.Status != stDone {
			kept = append(kept, it)
		}
	}
	q.Items = kept
	q.PublishRowsReset()
	q.notify()
	q.save()
}

// CancelAll stops every running download (progress is kept for next time).
func (q *Queue) CancelAll() {
	for _, it := range q.Items {
		if it.running && it.cancel != nil {
			it.cancel()
		}
	}
}

// schedule starts queued items until MaxParallel downloads are running.
func (q *Queue) schedule() {
	max := q.MaxParallel
	if max < 1 {
		max = 1
	}
	running := 0
	for _, it := range q.Items {
		if it.running {
			running++
		}
	}
	for _, it := range q.Items {
		if running >= max {
			break
		}
		if !it.running && it.Status == stQueued {
			q.run(it)
			running++
		}
	}
}

func (q *Queue) run(it *Item) {
	ctx, cancel := context.WithCancel(context.Background())
	it.cancel = cancel
	it.running = true
	it.Status = stDownloading
	it.Speed = 0
	q.changed(it)
	q.save() // persist active state before starting the worker

	snap := *it // background goroutines only read this copy
	go func() {
		var err error
		switch snap.Kind {
		case kindVideo:
			err = q.execVideo(ctx, it, &snap)
		case kindTorrent:
			err = q.execTorrent(ctx, it, &snap)
		default:
			err = q.execFile(ctx, it, &snap)
		}
		q.post(func() {
			it.running = false
			it.Speed = 0
			it.cancel = nil
			switch {
			case err == nil:
				it.Segments = nil
				it.Status = stDone
				it.Cookie, it.Jar = "", "" // login data is wiped once the download is finished
				if it.Path != "" {
					if st, serr := os.Stat(it.Path); serr == nil && !st.IsDir() {
						it.Total = st.Size()
					}
				}
				it.Done = it.Total
				if q.onDone != nil {
					q.onDone(it)
				}
			case errors.Is(err, context.Canceled):
				it.Status = stPaused
			default:
				msg := err.Error()
				if it.Kind == kindVideo {
					msg = loginHint(msg)
				}
				it.Status = "Error: " + msg
			}
			q.changed(it)
			q.save()
			q.schedule()
		})
	}()
}

func (q *Queue) execFile(ctx context.Context, it, s *Item) error {
	var hdr http.Header
	if s.Referer != "" || s.Cookie != "" {
		hdr = http.Header{}
		if s.Referer != "" {
			hdr.Set("Referer", s.Referer)
		}
		if s.Cookie != "" {
			hdr.Set("Cookie", s.Cookie)
		}
	}
	job := &Job{
		URL: s.URL, Dir: s.Dir, Name: s.Name, Conns: q.MaxSegments, Header: hdr,
		OnSegments: func(segments []Segment) {
			snapshot := append([]Segment(nil), segments...)
			q.post(func() {
				if it.running {
					it.Segments = snapshot
					q.changed(it)
				}
			})
		},
		OnName: func(n string) {
			q.post(func() {
				if it.Name == "" {
					it.Name = n
				}
				it.Path = filepath.Join(it.Dir, n)
				q.changed(it)
				q.save()
			})
		},
	}
	lastT := time.Now()
	lastD := int64(-1)
	report, flush := q.progressReporter(it)
	err := job.Run(ctx, func(done, total int64) {
		now := time.Now()
		speed := 0.0
		if el := now.Sub(lastT).Seconds(); lastD >= 0 && el > 0 {
			speed = float64(done-lastD) / el
		}
		lastT, lastD = now, done
		report(done, total, speed)
	})
	flush()
	return err
}

func (q *Queue) execVideo(ctx context.Context, it, s *Item) error {
	report, flush := q.progressReporter(it)
	path, err := runVideo(ctx, s.URL, s.Dir, s.VArgs, speedLimit.Load()/1024, s.Jar, s.Referer,
		func(done, total int64, speed float64) {
			report(done, total, speed)
		},
		func(st string) {
			q.post(func() {
				it.Status = st
				q.changed(it)
			})
		},
		func(title string) {
			q.post(func() {
				if it.Name == "" {
					it.Name = title
					q.changed(it)
					q.save()
				}
			})
		})
	flush()
	if path != "" {
		q.post(func() {
			it.Path = path
			it.Name = filepath.Base(path)
			q.changed(it)
			q.save()
		})
	}
	return err
}

// ---- settings and the saved list ----

func configDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	dir = filepath.Join(dir, "Sabo DM")
	os.MkdirAll(dir, 0755)
	return dir
}

type Settings struct {
	Dir           string
	Parallel      int
	MaxSegments   int // maximum byte-range workers per direct file (1..32)
	LimitKB       int
	CookieBrowser string // yt-dlp --cookies-from-browser, "" = none
	Language      string // en, ar, fr

	AutoStart      bool     // start with Windows (tray only)
	AutoStartAsked bool     // the first-run question was already answered
	AutoResume     bool     // resume unfinished downloads when the app starts
	Queues         []string `json:"queues,omitempty"`
}

func settingsPath() string { return filepath.Join(configDir(), "settings.json") }

func loadSettings() Settings {
	s := Settings{Parallel: 3, MaxSegments: 16, Language: "en", AutoStart: true, Queues: []string{"Default"}}
	if b, err := os.ReadFile(settingsPath()); err == nil {
		json.Unmarshal(b, &s)
	}
	if s.Parallel < 1 || s.Parallel > 8 {
		s.Parallel = 3
	}
	if s.MaxSegments < 1 || s.MaxSegments > 32 {
		s.MaxSegments = 16
	}
	if s.Language != "en" && s.Language != "ar" && s.Language != "fr" {
		s.Language = "en"
	}
	if s.LimitKB < 0 {
		s.LimitKB = 0
	}
	if len(s.Queues) == 0 {
		s.Queues = []string{"Default"}
	}
	return s
}

func saveSettings(s Settings) {
	if b, err := json.MarshalIndent(s, "", "  "); err == nil {
		os.WriteFile(settingsPath(), b, 0644)
	}
}

type savedItem struct {
	URL, Dir, Name, Path string
	Queue, Category      string
	Kind, VLabel         string
	VArgs                []string
	Done                 bool
	DoneBytes            int64
	Total                int64
	Status               string
	Select               string
	Referer              string
	CookieEnc            string // DPAPI-sealed, base64
	JarEnc               string // DPAPI-sealed, base64
}

func queuePath() string   { return filepath.Join(configDir(), "queue.json") }
func queueDBPath() string { return filepath.Join(configDir(), "queue.db") }

func (q *Queue) savedItems() []savedItem {
	out := make([]savedItem, 0, len(q.Items))
	for _, it := range q.Items {
		si := savedItem{URL: it.URL, Dir: it.Dir, Name: it.Name, Path: it.Path, Queue: it.Queue, Category: it.Category, Kind: it.Kind, VLabel: it.VLabel,
			VArgs: it.VArgs, Done: it.Status == stDone, DoneBytes: it.Done, Total: it.Total, Status: it.Status,
			Select: it.Select, Referer: it.Referer}
		if it.Status != stDone { // login data is only kept while unfinished, and only encrypted
			si.CookieEnc = sealString(it.Cookie)
			si.JarEnc = sealString(it.Jar)
		}
		out = append(out, si)
	}
	return out
}

// writeLegacyQueue is a compatibility fallback for systems where SQLite cannot
// be opened. It writes a complete temporary file before replacing the old file.
func writeLegacyQueue(items []savedItem) error {
	b, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(configDir(), "queue-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		_ = tmp.Close()
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err = tmp.Write(b); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	target, backup := queuePath(), queuePath()+".bak"
	_ = os.Remove(backup)
	if _, statErr := os.Stat(target); statErr == nil {
		if err := os.Rename(target, backup); err != nil {
			return err
		}
	}
	if err := os.Rename(tmpName, target); err != nil {
		if _, statErr := os.Stat(backup); statErr == nil {
			_ = os.Rename(backup, target)
		}
		return err
	}
	ok = true
	return nil
}

func (q *Queue) save() {
	items := q.savedItems()
	if q.store != nil {
		if err := q.store.replace(items); err == nil {
			return
		} else {
			debugf("SQLite queue save failed; using JSON compatibility fallback: %v", err)
		}
	}
	if err := writeLegacyQueue(items); err != nil {
		debugf("queue fallback save failed: %v", err)
	}
}

// load prefers SQLite. Existing queue.json installations are imported once.
// Active transfers are restored as Paused; the existing Auto Resume setting
// can restart them. Direct-file segment maps remain in the adjacent .gdm files.
func (q *Queue) load() {
	store, err := openQueueStore(queueDBPath())
	if err == nil {
		q.store = store
	} else {
		debugf("could not initialize SQLite queue store: %v", err)
	}

	var in []savedItem
	if q.store != nil {
		stored, loadErr := q.store.load()
		if loadErr == nil {
			in = stored
		} else {
			debugf("could not read SQLite queue: %v", loadErr)
		}
	}

	// Import the old JSON queue only when the database has no saved rows.
	if len(in) == 0 {
		if b, readErr := os.ReadFile(queuePath()); readErr == nil {
			var legacy []savedItem
			if json.Unmarshal(b, &legacy) == nil {
				in = legacy
				if q.store != nil {
					if saveErr := q.store.replace(legacy); saveErr == nil {
						_ = os.Rename(queuePath(), queuePath()+".migrated")
					} else {
						debugf("legacy queue import failed: %v", saveErr)
					}
				}
			}
		}
	}

	for _, s := range in {
		status := stPaused
		if s.Done {
			status = stDone
		} else if strings.HasPrefix(s.Status, "Error:") {
			status = s.Status
		}
		done := s.DoneBytes
		if s.Done {
			done = s.Total
		}
		it := &Item{URL: s.URL, Dir: s.Dir, Name: s.Name, Path: s.Path, Queue: s.Queue, Category: s.Category, Kind: s.Kind,
			VLabel: s.VLabel, VArgs: s.VArgs, Done: done, Total: s.Total, Status: status,
			Select: s.Select, Referer: s.Referer, Cookie: openString(s.CookieEnc), Jar: openString(s.JarEnc)}
		if it.Kind == "" {
			it.Kind = kindFile
		}
		if it.Queue == "" {
			it.Queue = "Default"
		}
		if it.Category == "" {
			it.Category = categoryFor(it.Name, it.Kind)
		}
		q.Items = append(q.Items, it)
	}
}

func (q *Queue) close() {
	if q.store != nil {
		if err := q.store.close(); err != nil {
			debugf("close SQLite queue store: %v", err)
		}
		q.store = nil
	}
}

func (q *Queue) execTorrent(ctx context.Context, it, s *Item) error {
	return runTorrent(ctx, s.URL, s.Dir, speedLimit.Load()/1024, s.Select,
		func(done, total int64, speed float64, status string) {
			q.post(func() {
				it.Done, it.Total, it.Speed = done, total, speed
				if status != "" {
					it.Status = status
				}
				q.changed(it)
			})
		},
		func(name string) {
			q.post(func() {
				it.Name = name
				it.Path = filepath.Join(it.Dir, name)
				q.changed(it)
				q.save()
			})
		})
}
