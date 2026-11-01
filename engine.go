package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36"

type Segment struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"` // inclusive
	Done  int64 `json:"done"`
}

type State struct {
	URL      string    `json:"url"`    // the link the user pasted
	Direct   string    `json:"direct"` // the real file link (refreshed on every run)
	Size     int64     `json:"size"`
	Ranges   bool      `json:"ranges"`
	Segments []Segment `json:"segments"`
}

// Job is one download. Run can be called again with the same URL to resume.
type Job struct {
	URL        string
	Dir        string // folder to save into
	Name       string // optional file name chosen by the user
	Out        string // full path, filled in by Run
	Conns      int
	Header     http.Header       // extra headers (Referer, Cookie) from the browser; kept in memory only
	OnName     func(name string) // optional: called once the file name is known
	OnSegments func([]Segment)   // optional: receives a snapshot of live segment progress
}

type hdrKey struct{}

var client = func() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Jar: jar,
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			MaxIdleConns:          256,
			MaxIdleConnsPerHost:   64,
			MaxConnsPerHost:       0, // no artificial per-host connection ceiling
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			ForceAttemptHTTP2:     true,
		},
	}
}()

type fileInfo struct {
	Size   int64
	Ranges bool
	Name   string
}

func newRequest(ctx context.Context, link string, from, to int64, ranged bool) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", link, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	if h, ok := ctx.Value(hdrKey{}).(http.Header); ok {
		for k, vs := range h {
			for _, v := range vs {
				req.Header.Add(k, v)
			}
		}
	}
	if ranged {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", from, to))
	}
	return req, nil
}

func nameFromURL(link string) string {
	u, err := url.Parse(link)
	if err != nil {
		return ""
	}
	name := path.Base(u.Path)
	if un, err := url.PathUnescape(name); err == nil {
		name = un
	}
	if name == "." || name == "/" {
		return ""
	}
	return name
}

func sanitizeName(name string) string {
	name = strings.ToValidUTF8(name, "_") // never show broken characters
	name = filepath.Base(strings.TrimSpace(name))
	name = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, r) || r < 32 {
			return '_'
		}
		return r
	}, name)
	name = strings.Trim(name, " .")
	if name == "" {
		return "download.bin"
	}
	return name
}

func probe(ctx context.Context, link string) (fileInfo, error) {
	var info fileInfo
	req, err := newRequest(ctx, link, 0, 0, true)
	if err != nil {
		return info, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return info, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 206 {
		return info, fmt.Errorf("server returned HTTP %d - not a downloadable file link", resp.StatusCode)
	}
	contentType := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	if strings.HasPrefix(strings.ToLower(contentType), "text/html") {
		return info, fmt.Errorf("this link is a web page (HTML), not a direct file link, and this site is not supported yet")
	}
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if _, params, e := mime.ParseMediaType(cd); e == nil {
			info.Name = params["filename"]
			if info.Name == "" {
				info.Name = params["filename*"]
			}
		}
	}
	if info.Name == "" {
		info.Name = nameFromURL(link)
	}
	if filepath.Ext(info.Name) == "" && contentType != "" {
		if exts, _ := mime.ExtensionsByType(contentType); len(exts) > 0 {
			priority := map[string]int{".pdf": 1, ".zip": 1, ".7z": 1, ".rar": 1, ".mp3": 1, ".mp4": 1, ".jpg": 1, ".jpeg": 1, ".png": 1, ".docx": 1, ".xlsx": 1, ".exe": 1}
			sort.SliceStable(exts, func(i, j int) bool { return priority[strings.ToLower(exts[i])] > priority[strings.ToLower(exts[j])] })
			info.Name += exts[0]
		}
	}
	if resp.StatusCode == 206 {
		var start, end, total int64
		n, _ := fmt.Sscanf(resp.Header.Get("Content-Range"), "bytes %d-%d/%d", &start, &end, &total)
		if n == 3 && total > 0 {
			info.Size, info.Ranges = total, true
			return info, nil
		}
		return info, fmt.Errorf("server did not report the file size")
	}
	if resp.ContentLength <= 0 {
		return info, fmt.Errorf("server did not report the file size")
	}
	info.Size = resp.ContentLength
	return info, nil
}

func (j *Job) prepare(ctx context.Context) (*State, error) {
	direct, resolvedName, err := Resolve(ctx, j.URL)
	if err != nil {
		return nil, err
	}
	info, err := probe(ctx, direct)
	if err != nil {
		return nil, err
	}
	if j.Out == "" {
		name := j.Name
		if name == "" {
			name = resolvedName
		}
		if name == "" {
			name = info.Name
		}
		if name == "" {
			name = nameFromURL(direct)
		}
		j.Out = filepath.Join(j.Dir, sanitizeName(name))
	}
	if j.OnName != nil {
		j.OnName(filepath.Base(j.Out))
	}

	// Resume if a matching, unfinished download exists.
	if b, rerr := os.ReadFile(j.Out + ".gdm"); rerr == nil {
		var st State
		if fileInfo, serr := os.Stat(j.Out); serr == nil && fileInfo.Size() == info.Size &&
			json.Unmarshal(b, &st) == nil && st.URL == j.URL && st.Size == info.Size &&
			validResumeState(&st, info.Size, info.Ranges) {
			st.Direct = direct // direct links expire, so always use the fresh one
			if !info.Ranges {  // server can't resume: start over
				for i := range st.Segments {
					st.Segments[i].Done = 0
				}
			}
			st.Ranges = info.Ranges
			return &st, nil
		}
	}

	maxSegments := j.Conns
	if maxSegments < 1 {
		maxSegments = 16
	}
	if maxSegments > 32 {
		maxSegments = 32
	}
	n := adaptiveSegmentCount(info.Size, info.Ranges, maxSegments)
	st := &State{URL: j.URL, Direct: direct, Size: info.Size, Ranges: info.Ranges}
	st.Segments = buildSegments(info.Size, n)
	return st, nil
}

// adaptiveSegmentCount chooses enough parallel ranges to keep segments near
// an 8 MiB target while respecting the user's configured 1..32 upper bound.
// Servers without byte-range support and small files always use one stream.
func adaptiveSegmentCount(size int64, ranges bool, maxSegments int) int {
	if maxSegments < 1 {
		maxSegments = 1
	}
	if maxSegments > 32 {
		maxSegments = 32
	}
	if !ranges || size < 1<<20 {
		return 1
	}
	if size <= 0 {
		return 1
	}
	const targetSegmentSize = int64(8 << 20)
	n := int(1 + (size-1)/targetSegmentSize)
	if n < 1 {
		n = 1
	}
	if n > maxSegments {
		n = maxSegments
	}
	return n
}

// validResumeState prevents corrupt or stale sidecar metadata from creating
// overlapping ranges, out-of-bounds writes, or impossible resume offsets.
func validResumeState(st *State, size int64, ranges bool) bool {
	if st == nil || size <= 0 || st.Size != size || len(st.Segments) < 1 || len(st.Segments) > 32 {
		return false
	}
	if !ranges && len(st.Segments) != 1 {
		return false
	}
	expectedStart := int64(0)
	for _, seg := range st.Segments {
		if seg.Start != expectedStart || seg.End < seg.Start || seg.End >= size {
			return false
		}
		length := seg.End - seg.Start + 1
		if seg.Done < 0 || seg.Done > length {
			return false
		}
		expectedStart = seg.End + 1
	}
	return expectedStart == size
}

// buildSegments divides [0,size) into n non-overlapping, contiguous byte ranges.
func buildSegments(size int64, n int) []Segment {
	if size <= 0 {
		return nil
	}
	if n < 1 {
		n = 1
	}
	if n > 32 {
		n = 32
	}
	if int64(n) > size {
		n = int(size)
	}
	segments := make([]Segment, 0, n)
	base, remainder := size/int64(n), size%int64(n)
	start := int64(0)
	for i := 0; i < n; i++ {
		length := base
		if int64(i) < remainder {
			length++
		}
		end := start + length - 1
		segments = append(segments, Segment{Start: start, End: end})
		start = end + 1
	}
	return segments
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func worker(ctx context.Context, st *State, i int, f *os.File, mu *sync.Mutex,
	wg *sync.WaitGroup, errCh chan<- error, stop context.CancelFunc) {
	defer wg.Done()
	seg := &st.Segments[i]
	buf := make([]byte, 256*1024) // larger sequential reads reduce syscall overhead on fast links
	var lastErr error
	fails := 0
	for fails < 10 {
		if ctx.Err() != nil {
			return
		}
		pos := seg.Start + seg.Done
		if pos > seg.End {
			return // segment complete
		}
		startDone := seg.Done
		req, err := newRequest(ctx, st.Direct, pos, seg.End, st.Ranges)
		if err != nil {
			errCh <- err
			stop()
			return
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			fails++
			sleepCtx(ctx, time.Duration(fails)*time.Second)
			continue
		}
		if !(resp.StatusCode == 206 || (resp.StatusCode == 200 && !st.Ranges)) {
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			fails++
			sleepCtx(ctx, time.Duration(fails)*time.Second)
			continue
		}
		for {
			n, rerr := resp.Body.Read(buf)
			if n > 0 {
				remaining := seg.End - (seg.Start + seg.Done) + 1
				if int64(n) > remaining {
					n = int(remaining)
				}
				if n > 0 {
					if _, werr := f.WriteAt(buf[:n], seg.Start+seg.Done); werr != nil {
						resp.Body.Close()
						errCh <- werr
						stop()
						return
					}
					mu.Lock()
					seg.Done += int64(n)
					mu.Unlock()
					throttle(ctx, n)
				}
			}
			if rerr != nil {
				if rerr != io.EOF {
					lastErr = rerr
				}
				break
			}
		}
		resp.Body.Close()
		if seg.Done == startDone {
			fails++ // no progress this round
			sleepCtx(ctx, time.Duration(fails)*time.Second)
		} else {
			fails = 0
		}
		debugf("segment %d at %d/%d", i, seg.Done, seg.End-seg.Start+1)
	}
	if ctx.Err() == nil {
		errCh <- fmt.Errorf("segment %d failed: %v", i, lastErr)
		stop()
	}
}

// Run downloads until finished. It returns nil when complete,
// context.Canceled when paused, or another error on failure.
func (j *Job) Run(ctx context.Context, progress func(done, total int64)) error {
	if j.Header != nil {
		ctx = context.WithValue(ctx, hdrKey{}, j.Header)
	}
	st, err := j.prepare(ctx)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(j.Out, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	if err := f.Truncate(st.Size); err != nil {
		f.Close()
		return err
	}
	statePath := j.Out + ".gdm"

	var mu sync.Mutex
	report := func() {
		// Flush file data before its resume map. This makes a pause/restart safe
		// even after a long wait or an unexpected process termination.
		_ = f.Sync()
		mu.Lock()
		var d int64
		segments := append([]Segment(nil), st.Segments...)
		for _, seg := range segments {
			d += seg.Done
		}
		b, _ := json.Marshal(st)
		mu.Unlock()
		if j.OnSegments != nil {
			j.OnSegments(segments)
		}
		// Replace the resume metadata atomically so a crash cannot leave a
		// half-written JSON state file that prevents a later resume.
		tmpState := statePath + ".tmp"
		if err := os.WriteFile(tmpState, b, 0644); err == nil {
			_ = os.Rename(tmpState, statePath)
		}
		if progress != nil {
			progress(d, st.Size)
		}
	}

	wctx, stop := context.WithCancel(ctx)
	defer stop()
	errCh := make(chan error, len(st.Segments))
	var wg sync.WaitGroup
	for i := range st.Segments {
		wg.Add(1)
		go worker(wctx, st, i, f, &mu, &wg, errCh, stop)
	}

	tickStop := make(chan struct{})
	var tw sync.WaitGroup
	tw.Add(1)
	go func() {
		defer tw.Done()
		t := time.NewTicker(500 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-tickStop:
				return
			case <-t.C:
				report()
			}
		}
	}()

	wg.Wait()
	close(tickStop)
	tw.Wait()
	report()
	f.Close()

	if ctx.Err() != nil {
		return ctx.Err() // paused
	}
	select {
	case err := <-errCh:
		return err
	default:
	}
	var total int64
	for _, s := range st.Segments {
		total += s.Done
	}
	if total != st.Size {
		return fmt.Errorf("download incomplete (%d of %d bytes)", total, st.Size)
	}
	os.Remove(statePath)
	return nil
}

// ---- global speed limit (shared by all downloads) ----
// A shared token bucket caps aggregate throughput across all file workers.
// It re-reads the setting while waiting, so switching to Unlimited takes effect
// immediately instead of leaving workers sleeping on an old rate reservation.
var speedLimit atomic.Int64 // bytes per second, 0 = unlimited

func setSpeedLimit(bytesPerSecond int64) {
	if bytesPerSecond < 0 {
		bytesPerSecond = 0
	}
	speedLimit.Store(bytesPerSecond)
	limiter.Lock()
	limiter.tokens = 256 * 1024
	limiter.last = time.Now()
	limiter.Unlock()
}

var limiter = struct {
	sync.Mutex
	tokens float64
	last   time.Time
}{tokens: 256 * 1024, last: time.Now()}

func throttle(ctx context.Context, n int) {
	if n <= 0 {
		return
	}
	for {
		rate := speedLimit.Load()
		if rate <= 0 {
			return
		}
		limiter.Lock()
		now := time.Now()
		if limiter.last.IsZero() {
			limiter.last = now
		}
		elapsed := now.Sub(limiter.last).Seconds()
		if elapsed > 0 {
			limiter.tokens += elapsed * float64(rate)
			if limiter.tokens > 256*1024 {
				limiter.tokens = 256 * 1024
			}
			limiter.last = now
		}
		if limiter.tokens >= float64(n) {
			limiter.tokens -= float64(n)
			limiter.Unlock()
			return
		}
		missing := float64(n) - limiter.tokens
		limiter.tokens = 0
		wait := time.Duration(missing / float64(rate) * float64(time.Second))
		limiter.Unlock()
		if wait > 100*time.Millisecond {
			wait = 100 * time.Millisecond
		}
		if wait < time.Millisecond {
			wait = time.Millisecond
		}
		if !sleepCtx(ctx, wait) {
			return
		}
	}
}

// checkDirect verifies that a link can really be fetched. The browser extension
// uses it so the browser's own download is only cancelled when we can take over.
func checkDirect(ctx context.Context, link string, hdr http.Header) error {
	if hdr != nil {
		ctx = context.WithValue(ctx, hdrKey{}, hdr)
	}
	direct, _, err := Resolve(ctx, link)
	if err != nil {
		return err
	}
	_, err = probe(ctx, direct)
	return err
}
