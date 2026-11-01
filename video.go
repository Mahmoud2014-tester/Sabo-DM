package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"debug/pe"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

// Video downloads use yt-dlp (downloaded on demand, so this app stays small).
// If ffmpeg.exe is also present, high qualities (video+audio merged) and MP3 work.

const ytDlpURL = "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp.exe"

var errNoYtDlp = errors.New("yt-dlp.exe not found")

var videoHosts = []string{
	"youtube.com", "youtu.be", "vimeo.com", "dailymotion.com", "twitch.tv", "tiktok.com",
	"facebook.com", "fb.watch", "instagram.com", "twitter.com", "x.com", "reddit.com",
	"soundcloud.com", "bilibili.com", "streamable.com",
	// Learning and course platforms (actual access still requires valid account permissions).
	"udemy.com", "coursera.org", "linkedin.com", "skillshare.com", "pluralsight.com",
	"edx.org", "khanacademy.org", "masterclass.com", "futurelearn.com", "domestika.org",
}

func isVideoSite(link string) bool {
	u, err := url.Parse(link)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, h := range videoHosts {
		if host == h || strings.HasSuffix(host, "."+h) {
			return true
		}
	}
	return false
}

func hidden(cmd *exec.Cmd) *exec.Cmd {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8", "PYTHONUTF8=1")
	return cmd
}

func toolsDir() string {
	dir := filepath.Join(configDir(), "tools")
	os.MkdirAll(dir, 0755)
	return dir
}

// findTool looks in the app tools folder, next to the app, then in PATH.
func findTool(name string) string {
	candidates := []string{filepath.Join(toolsDir(), name)}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), name))
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return ""
}

// findFFmpegDir returns a directory containing BOTH executables. yt-dlp's
// --ffmpeg-location is deliberately given a directory, not ffmpeg.exe itself:
// yt-dlp must locate ffprobe.exe beside ffmpeg.exe during extraction.
func findFFmpegDir() string {
	var dirs []string
	addDir := func(dir string) {
		if dir == "" {
			return
		}
		dir, err := filepath.Abs(dir)
		if err != nil {
			return
		}
		for _, seen := range dirs {
			if strings.EqualFold(seen, dir) {
				return
			}
		}
		dirs = append(dirs, dir)
	}
	addDir(toolsDir())
	if exe, err := os.Executable(); err == nil {
		addDir(filepath.Dir(exe))
	}
	for _, name := range []string{"ffmpeg.exe", "ffprobe.exe"} {
		if p, err := exec.LookPath(name); err == nil {
			addDir(filepath.Dir(p))
		}
	}
	for _, dir := range dirs {
		ff, e1 := os.Stat(filepath.Join(dir, "ffmpeg.exe"))
		fp, e2 := os.Stat(filepath.Join(dir, "ffprobe.exe"))
		if e1 == nil && e2 == nil && !ff.IsDir() && !fp.IsDir() {
			return dir
		}
	}
	return ""
}

// ffmpegReady performs a real execution check, not just an existence check.
// A damaged, blocked, wrong-architecture, or incomplete binary must never
// unlock conversion formats in the UI.
func ffmpegReady() (string, error) {
	dir := findFFmpegDir()
	if dir == "" {
		return "", errors.New("FFmpeg and FFprobe are not installed together")
	}
	for _, name := range []string{"ffmpeg.exe", "ffprobe.exe"} {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		cmd := hidden(exec.CommandContext(ctx, filepath.Join(dir, name), "-version"))
		out, err := cmd.CombinedOutput()
		cancel()
		if err != nil {
			msg := strings.TrimSpace(string(out))
			if len(msg) > 240 {
				msg = msg[:240]
			}
			return "", fmt.Errorf("%s exists but cannot run: %v %s", name, err, msg)
		}
		if !strings.Contains(strings.ToLower(string(out)), strings.TrimSuffix(name, ".exe")) {
			return "", fmt.Errorf("%s did not return a valid version response", name)
		}
	}
	return dir, nil
}

func lastLine(s string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\r", ""), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if l := strings.TrimSpace(lines[i]); l != "" {
			return strings.TrimPrefix(l, "ERROR: ")
		}
	}
	return ""
}

func atof(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}

// ---- video information and quality choices ----

type ytInfo struct {
	Title        string `json:"title"`
	Extractor    string `json:"extractor"`
	WebpageURL   string `json:"webpage_url"`
	Availability string `json:"availability"`
	IsLive       bool   `json:"is_live"`
	Formats      []struct {
		Height int    `json:"height"`
		VCodec string `json:"vcodec"`
	} `json:"formats"`
}

func (i *ytInfo) heights() []int {
	seen := map[int]bool{}
	var hs []int
	for _, f := range i.Formats {
		if f.Height > 0 && f.VCodec != "none" && !seen[f.Height] {
			seen[f.Height] = true
			hs = append(hs, f.Height)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(hs)))
	return hs
}

// normalizeSocialURL unwraps common login/redirect URLs used by social sites.
// yt-dlp needs the original post/reel permalink rather than /login.php?next=...
func normalizeSocialURL(raw string) string {
	current := strings.TrimSpace(raw)
	for i := 0; i < 4; i++ {
		u, err := url.Parse(current)
		if err != nil || u.Hostname() == "" {
			return current
		}
		host, path := strings.ToLower(u.Hostname()), strings.ToLower(u.Path)
		social := strings.Contains(host, "facebook.com") || strings.Contains(host, "instagram.com") || strings.Contains(host, "x.com") || strings.Contains(host, "twitter.com")
		if !social || !(strings.Contains(path, "login") || strings.Contains(path, "redirect") || strings.Contains(path, "checkpoint") || strings.Contains(path, "oauth")) {
			return current
		}
		q := u.Query()
		next := ""
		for _, key := range []string{"next", "continue", "redirect", "redirect_uri", "url", "u", "return_url"} {
			if v := strings.TrimSpace(q.Get(key)); v != "" {
				next = v
				break
			}
		}
		if next == "" || next == current {
			return current
		}
		if !strings.HasPrefix(strings.ToLower(next), "http://") && !strings.HasPrefix(strings.ToLower(next), "https://") {
			if decoded, e := url.QueryUnescape(next); e == nil {
				next = decoded
			}
		}
		current = next
	}
	return current
}

func normalizeLearningURL(raw string) string {
	// Course pages often arrive with tracking parameters or a deep lecture path.
	// yt-dlp's Udemy extractor identifies the course from /course/<slug>/;
	// canonicalizing that part avoids passing login/marketing query clutter.
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Hostname() == "" {
		return raw
	}
	host := strings.ToLower(u.Hostname())
	if host == "udemy.com" || strings.HasSuffix(host, ".udemy.com") {
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) >= 2 && parts[0] == "course" && parts[1] != "" {
			u.Path = "/course/" + parts[1] + "/"
			u.RawQuery = ""
			u.Fragment = ""
			return u.String()
		}
	}
	return raw
}

func isLinkedInLearningURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	h := strings.ToLower(u.Hostname())
	return h == "linkedin.com" || strings.HasSuffix(h, ".linkedin.com") || h == "linkedinlearning.com" || strings.HasSuffix(h, ".linkedinlearning.com")
}

func isSoundCloudURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	h := strings.ToLower(u.Hostname())
	return h == "soundcloud.com" || strings.HasSuffix(h, ".soundcloud.com") || h == "on.soundcloud.com"
}

func isSoundCloudMetadataError(err error) bool {
	if err == nil {
		return false
	}
	m := strings.ToLower(err.Error())
	return strings.Contains(m, "unable to download json metadata") ||
		strings.Contains(m, "http error 404") ||
		strings.Contains(m, "http error 403") ||
		strings.Contains(m, "soundcloud")
}

func isExtractorCompatibilityError(err error) bool {
	if err == nil {
		return false
	}
	m := strings.ToLower(err.Error())
	for _, marker := range []string{"unable to extract course id", "unsupported url", "no suitable extractor", "extractor error", "no video formats found", "no playable video formats", "requested format is not available", "unable to download json metadata", "http error 404"} {
		if strings.Contains(m, marker) {
			return true
		}
	}
	return false
}

// isBrowserCookieDatabaseError identifies Chromium/Firefox cookie database
// copy failures. These are local profile-access problems, not extractor errors.
func isBrowserCookieDatabaseError(err error) bool {
	if err == nil {
		return false
	}
	m := strings.ToLower(err.Error())
	for _, marker := range []string{
		"could not copy chrome cookie database",
		"could not copy chromium cookie database",
		"could not copy edge cookie database",
		"could not copy brave cookie database",
		"could not copy firefox cookie database",
		"could not copy cookie database",
		"failed to copy cookie database",
	} {
		if strings.Contains(m, marker) {
			return true
		}
	}
	return false
}

func fetchInfo(ctx context.Context, link, jar, referer string) (*ytInfo, error) {
	link = normalizeLearningURL(normalizeSocialURL(link))
	// A feed/homepage is not a video URL. In particular, yt-dlp correctly
	// rejects x.com/home; ask for the specific post/reel permalink instead.
	if u, err := url.Parse(link); err == nil {
		host := strings.ToLower(u.Hostname())
		path := strings.Trim(strings.ToLower(u.Path), "/")
		if (host == "x.com" || host == "www.x.com" || host == "twitter.com" || host == "www.twitter.com") && (path == "home" || path == "explore" || path == "notifications" || path == "messages" || path == "") {
			return nil, errors.New("This is an X feed page, not a specific video. Open the post containing the video, then use the post URL (for example x.com/username/status/123456789). If you are on the feed, use Sabo DM's on-video download button.")
		}
		if (host == "facebook.com" || host == "www.facebook.com" || host == "m.facebook.com") && (path == "" || path == "watch" || path == "home") && u.Query().Get("v") == "" {
			return nil, errors.New("This is a Facebook feed/page, not a specific video. Open the video or reel itself and copy its permalink, or use Sabo DM's on-video download button.")
		}
	}
	yt := findTool("yt-dlp.exe")
	if yt == "" {
		return nil, errNoYtDlp
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	attempt := func() (*ytInfo, error) {
		args := []string{"--encoding", "utf-8", "--ignore-config", "--no-playlist", "--no-warnings", "--dump-single-json", "--extractor-retries", "3", "--retries", "3", "--fragment-retries", "10"}
		extra, cleanup, err := extraArgs(jar)
		if err != nil {
			return nil, err
		}
		defer cleanup()
		args = append(args, extra...)
		if referer != "" {
			args = append(args, "--referer", referer)
		}
		args = append(args, link)
		cmd := hidden(exec.CommandContext(ctx, yt, args...))
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, runErr := cmd.Output()
		if runErr != nil {
			if msg := lastLine(stderr.String()); msg != "" {
				return nil, errors.New(msg)
			}
			return nil, runErr
		}
		var info ytInfo
		if err := json.Unmarshal(out, &info); err != nil {
			return nil, err
		}
		if len(info.Formats) == 0 {
			return &info, errors.New("extractor returned no playable video formats")
		}
		return &info, nil
	}
	info, err := attempt()
	if err == nil {
		return info, nil
	}
	// Chromium may refuse yt-dlp's attempt to copy its live cookie database
	// (profile lock / browser encryption / file permissions). Do not fail a
	// public video for that local credential-store problem: disable direct
	// browser-cookie reading for this app session and retry without cookies.
	// Extension-supplied Netscape cookies still take precedence in extraArgs.
	if jar == "" && getBrowser() != "" && isBrowserCookieDatabaseError(err) {
		browserName := getBrowser()
		browserSetting.Store("")
		info, retryErr := attempt()
		if retryErr == nil {
			return info, nil
		}
		return nil, fmt.Errorf("Sabo DM could not read the %s cookie database (%v). It retried without browser cookies, but the site still did not provide video access: %v. For signed-in Facebook, Instagram, X or other protected videos, open the Sabo DM browser extension, enable 'Use my login for videos', refresh the page and try again. For a public video, retry without requiring a browser sign-in. You can also fully exit the browser and restart Sabo DM before retrying", browserName, err, retryErr)
	}
	// Retry once with the current yt-dlp build when an extractor reports a
	// compatibility/metadata failure. SoundCloud's API changes can surface as
	// HTTP 404 while fetching JSON, not only as a conventional extractor error.
	shouldRefreshExtractor := isExtractorCompatibilityError(err) || (isSoundCloudURL(link) && isSoundCloudMetadataError(err))
	if shouldRefreshExtractor {
		if _, updateErr := updateYtDlp(); updateErr == nil {
			info, retryErr := attempt()
			if retryErr == nil {
				return info, nil
			}
			err = retryErr
		} else if isLinkedInLearningURL(link) || isSoundCloudURL(link) {
			return nil, fmt.Errorf("%v. Sabo DM could not refresh yt-dlp automatically: %v. Check your internet connection and use Update yt-dlp, then retry. No account credentials are sent by the update", err, updateErr)
		}
	}
	if isSoundCloudURL(link) && isSoundCloudMetadataError(err) {
		return nil, fmt.Errorf("SoundCloud did not return track metadata after refreshing yt-dlp. Check that the selected track is still available and that Sabo DM is using the actual currently playing track URL, then retry. Private, deleted, region-restricted or otherwise unavailable tracks may return this response. Details: %v", err)
	}
	if isLinkedInLearningURL(link) && (strings.Contains(strings.ToLower(err.Error()), "format") || strings.Contains(strings.ToLower(err.Error()), "linkedin")) {
		return nil, fmt.Errorf("LinkedIn Learning did not expose a downloadable video format for this page. Sign in to LinkedIn Learning in the same browser, refresh the lesson or preview page, enable Use my login for videos, and retry the specific lesson URL. Some previews are only playable inside LinkedIn's player or are restricted by the course's access/DRM policy; Sabo DM will not bypass those restrictions. Details: %v", err)
	}
	if strings.Contains(strings.ToLower(link), "udemy.com") && (strings.Contains(strings.ToLower(err.Error()), "login") || strings.Contains(strings.ToLower(err.Error()), "sign in") || strings.Contains(strings.ToLower(err.Error()), "course")) {
		return nil, fmt.Errorf("%v. For Udemy course videos, sign in to Udemy in your browser and enable 'Use my login for videos' in the Sabo DM extension, or select the signed-in browser under Sign in with", err)
	}
	return nil, err
}

type VideoOpt struct {
	Label string
	Args  []string
}

func bestArgs(haveFFmpeg bool) []string {
	if haveFFmpeg {
		// Prefer MP4 video + M4A audio streams, but retain a fallback for
		// sources that only expose WebM/other codecs. --remux-video makes
		// the final container MP4 without re-encoding compatible streams.
		return []string{"-f", "bv*[ext=mp4]+ba[ext=m4a]/bv*+ba/b", "--merge-output-format", "mp4", "--remux-video", "mp4"}
	}
	// Without FFmpeg we cannot promise an MP4 container. Preserve the
	// source container rather than renaming WebM data to .mp4.
	return []string{"-f", "b"}
}

func audioOptions(haveFFmpeg bool) []VideoOpt {
	// Always expose the common formats. MP3 and AAC conversion require
	// FFmpeg; without it the caller shows an installation hint and only
	// offers source-compatible choices for immediate downloading.
	if haveFFmpeg {
		return []VideoOpt{
			{"MP3", []string{"-f", "bestaudio/b", "-x", "--audio-format", "mp3", "--audio-quality", "0"}},
			{"M4A", []string{"-f", "bestaudio/b", "-x", "--audio-format", "m4a", "--audio-quality", "0"}},
			{"AAC", []string{"-f", "bestaudio/b", "-x", "--audio-format", "aac", "--audio-quality", "0"}},
			{"Best available audio (original format)", []string{"-f", "bestaudio/b"}},
			{"Opus", []string{"-f", "bestaudio/b", "-x", "--audio-format", "opus", "--audio-quality", "0"}},
			{"WAV", []string{"-f", "bestaudio/b", "-x", "--audio-format", "wav"}},
			{"FLAC", []string{"-f", "bestaudio/b", "-x", "--audio-format", "flac"}},
		}
	}
	// Never offer a conversion format when the required executables are absent.
	// This prevents a seemingly successful download from failing only afterward.
	return []VideoOpt{
		{"M4A (source format, when available)", []string{"-f", "bestaudio[ext=m4a]/bestaudio/b"}},
		{"AAC (source format, when available)", []string{"-f", "bestaudio[ext=aac]/bestaudio/b"}},
		{"Best available audio (original format)", []string{"-f", "bestaudio/b"}},
	}
}

func videoOptions(info *ytInfo, haveFFmpeg bool) []VideoOpt {
	bestLabel := "Best available (original format)"
	if haveFFmpeg {
		bestLabel = "Best available (MP4)"
	}
	opts := []VideoOpt{{bestLabel, bestArgs(haveFFmpeg)}}
	for _, h := range info.heights() {
		f := fmt.Sprintf("b[height<=%d]/b", h)
		args := []string{"-f", f}
		if haveFFmpeg {
			f = fmt.Sprintf("bv*[height<=%d][ext=mp4]+ba[ext=m4a]/bv*[height<=%d]+ba/b[height<=%d]/b", h, h, h)
			args = []string{"-f", f, "--merge-output-format", "mp4", "--remux-video", "mp4"}
		}
		label := fmt.Sprintf("%dp", h)
		if haveFFmpeg {
			label += " (MP4)"
		}
		opts = append(opts, VideoOpt{label, args})
	}
	if haveFFmpeg {
		opts = append(opts, VideoOpt{"Audio only (MP3)", []string{"-f", "bestaudio/b", "-x", "--audio-format", "mp3", "--audio-quality", "0"}})
	} else {
		opts = append(opts, VideoOpt{"Audio only (M4A, no conversion)", []string{"-f", "bestaudio[ext=m4a]/bestaudio/b"}})
	}
	return opts
}

// ---- running a video download ----

// runVideo downloads one video. It returns the final file path (when known).
// Cancelling ctx stops yt-dlp; running again continues from the partial file.
func runVideo(ctx context.Context, link, dir string, args []string, limitKB int64, jar, referer string,
	onProgress func(done, total int64, speed float64), onStatus func(string), onName func(string)) (string, error) {

	yt := findTool("yt-dlp.exe")
	if yt == "" {
		return "", errNoYtDlp
	}
	// Fail before downloading if the selected operation requires FFmpeg.
	needsFFmpeg := false
	for _, arg := range args {
		if arg == "-x" || arg == "--audio-format" || arg == "--audio-quality" {
			needsFFmpeg = true
			break
		}
		if arg == "--remux-video" || arg == "--recode-video" {
			needsFFmpeg = true
			break
		}
	}
	if needsFFmpeg {
		if _, ffErr := ffmpegReady(); ffErr != nil {
			return "", fmt.Errorf("audio conversion was not started because FFmpeg/FFprobe are not ready: %w. Click Install FFmpeg and wait for its success confirmation, then retry", ffErr)
		}
	}
	link = normalizeLearningURL(normalizeSocialURL(link))
	status := func(s string) {
		if onStatus != nil {
			onStatus(s)
		}
	}
	// The title and the final file name are passed through small UTF-8 files, because the
	// console encoding of a pipe mangles Arabic and other non-English characters.
	tmpDir := filepath.Join(configDir(), "tmp")
	os.MkdirAll(tmpDir, 0700)
	tag := randHex(6)
	pathFile := filepath.Join(tmpDir, "pf-"+tag+".txt")
	titleFile := filepath.Join(tmpDir, "tf-"+tag+".txt")
	defer os.Remove(pathFile)
	defer os.Remove(titleFile)
	a := []string{
		"--encoding", "utf-8", "--progress",
		"--print-to-file", "after_move:filepath", pathFile,
		"--print-to-file", "before_dl:%(title)s", titleFile,
		"--ignore-config", "--no-playlist", "--newline", "--no-colors", "--no-mtime", "--windows-filenames",
		"--extractor-retries", "3", "--retries", "3", "--fragment-retries", "10",
		"--progress-template", "download:SABER|%(progress.downloaded_bytes)s|%(progress.total_bytes)s|%(progress.total_bytes_estimate)s|%(progress.speed)s",
		"-o", filepath.Join(dir, "%(title).150s [%(id)s].%(ext)s"),
	}
	if ffDir := findFFmpegDir(); ffDir != "" {
		a = append(a, "--ffmpeg-location", ffDir)
	}
	if limitKB > 0 {
		a = append(a, "--limit-rate", fmt.Sprintf("%dK", limitKB))
	}
	extra, cleanup, err := extraArgs(jar)
	if err != nil {
		return "", err
	}
	defer cleanup()
	a = append(a, extra...)
	if referer != "" {
		a = append(a, "--referer", referer)
	}
	a = append(a, args...)
	a = append(a, link)

	cmd := hidden(exec.CommandContext(ctx, yt, a...))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}

	path := ""
	titleSent := false
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case strings.HasPrefix(line, "SABER|"):
			if !titleSent { // show the real video name as soon as the download starts
				if b, err := os.ReadFile(titleFile); err == nil {
					t := strings.TrimSpace(strings.SplitN(strings.ReplaceAll(string(b), "\r", ""), "\n", 2)[0])
					if t != "" {
						titleSent = true
						if onName != nil {
							onName(t)
						}
					}
				}
			}
			p := strings.Split(line, "|")
			if len(p) >= 5 && onProgress != nil {
				total := atof(p[2])
				if total == 0 {
					total = atof(p[3])
				}
				onProgress(int64(atof(p[1])), int64(total), atof(p[4]))
			}
		case strings.Contains(line, "Merging formats into"):
			if i, j := strings.Index(line, `"`), strings.LastIndex(line, `"`); i >= 0 && j > i {
				path = line[i+1 : j]
			}
			status("Merging audio and video...")
		case strings.HasPrefix(line, "[ExtractAudio] Destination:"):
			path = strings.TrimSpace(strings.TrimPrefix(line, "[ExtractAudio] Destination:"))
			status("Converting...")
		case strings.HasPrefix(line, "[download] Destination:"):
			path = strings.TrimSpace(strings.TrimPrefix(line, "[download] Destination:"))
		case strings.HasPrefix(line, "[download] ") && strings.HasSuffix(line, " has already been downloaded"):
			path = strings.TrimSuffix(strings.TrimPrefix(line, "[download] "), " has already been downloaded")
		}
	}
	werr := cmd.Wait()
	if ctx.Err() != nil {
		return "", ctx.Err() // paused
	}
	if werr != nil {
		msg := lastLine(stderr.String())
		if msg == "" {
			msg = werr.Error()
		}
		return "", errors.New(msg)
	}
	if b, err := os.ReadFile(pathFile); err == nil {
		for _, ln := range strings.Split(strings.ReplaceAll(string(b), "\r", ""), "\n") {
			if ln = strings.TrimSpace(ln); ln != "" {
				path = ln // the last line is the final file
			}
		}
	}
	return path, nil
}

// ---- installing / updating yt-dlp ----

func downloadYtDlp(ctx context.Context, progress func(done, total int64)) error {
	dst := filepath.Join(toolsDir(), "yt-dlp.exe")
	req, err := http.NewRequestWithContext(ctx, "GET", ytDlpURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}
	tmp := dst + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	var done int64
	buf := make([]byte, 64*1024)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				f.Close()
				os.Remove(tmp)
				return werr
			}
			done += int64(n)
			if progress != nil {
				progress(done, resp.ContentLength)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			f.Close()
			os.Remove(tmp)
			return rerr
		}
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if done < 1<<20 || (resp.ContentLength > 0 && done != resp.ContentLength) {
		os.Remove(tmp)
		return fmt.Errorf("incomplete download")
	}
	return os.Rename(tmp, dst)
}

func updateYtDlp() (string, error) {
	yt := findTool("yt-dlp.exe")
	if yt == "" {
		return "", errNoYtDlp
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	out, err := hidden(exec.CommandContext(ctx, yt, "-U")).CombinedOutput()
	text := lastLine(string(out))
	if err != nil {
		if text == "" {
			text = err.Error()
		}
		return "", errors.New(text)
	}
	return text, nil
}

// ---- opening files ----

func openFile(path string) {
	cmd := exec.Command("cmd", "/c", "start", "", path)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Start()
}

func showInFolder(path string) {
	cmd := exec.Command("explorer.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `explorer.exe /select,"` + path + `"`}
	cmd.Start()
}

// ---- signing in (cookies) and the JavaScript engine YouTube needs ----

// Which browser yt-dlp may read cookies from ("" = none). Firefox is the most
// reliable; recent Chrome/Edge/Brave versions often block this, which is why
// the browser extension can send the cookies instead.
var browserSetting atomic.Value // string

func getBrowser() string {
	if v, ok := browserSetting.Load().(string); ok {
		return v
	}
	return ""
}

type cookieRec struct {
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Secure   bool    `json:"secure"`
	HTTPOnly bool    `json:"httpOnly"`
	Expires  float64 `json:"expires"`
}

// netscapeJar converts the extension's cookies to the cookies.txt format yt-dlp reads.
func netscapeJar(cs []cookieRec) string {
	if len(cs) == 0 {
		return ""
	}
	clean := strings.NewReplacer("\t", " ", "\r", "", "\n", "").Replace
	var b strings.Builder
	b.WriteString("# Netscape HTTP Cookie File\n")
	for _, c := range cs {
		if c.Domain == "" || c.Name == "" {
			continue
		}
		domain := clean(c.Domain)
		sub, sec := "FALSE", "FALSE"
		if strings.HasPrefix(domain, ".") {
			sub = "TRUE"
		}
		if c.Secure {
			sec = "TRUE"
		}
		path := clean(c.Path)
		if path == "" {
			path = "/"
		}
		if c.HTTPOnly {
			b.WriteString("#HttpOnly_")
		}
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\t%d\t%s\t%s\n", domain, sub, path, sec, int64(c.Expires), clean(c.Name), clean(c.Value))
	}
	return b.String()
}

// writeJar puts the cookies in a private temporary file that is deleted right after use.
func writeJar(jar string) (string, func(), error) {
	dir := filepath.Join(configDir(), "tmp")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", nil, err
	}
	p := filepath.Join(dir, "ck-"+randHex(6)+".txt")
	if err := os.WriteFile(p, []byte(jar), 0600); err != nil {
		return "", nil, err
	}
	return p, func() { os.Remove(p) }, nil
}

// extraArgs are the yt-dlp options for sign-in and for the JavaScript engine.
func extraArgs(jar string) ([]string, func(), error) {
	var a []string
	cleanup := func() {}
	if dn := findTool("deno.exe"); dn != "" {
		a = append(a, "--js-runtimes", "deno:"+dn)
	}
	switch {
	case jar != "":
		p, c, err := writeJar(jar)
		if err != nil {
			return nil, cleanup, err
		}
		a = append(a, "--cookies", p)
		cleanup = c
	case getBrowser() != "":
		a = append(a, "--cookies-from-browser", getBrowser())
	}
	return a, cleanup, nil
}

// loginHint explains what to do when a video needs a signed-in account.
func loginHint(msg string) string {
	low := strings.ToLower(msg)
	if isBrowserCookieDatabaseError(errors.New(msg)) {
		return msg + "  >> Chrome/Edge/Brave may keep the cookie database locked or protected while the browser is running. Sabo DM can retry public videos without browser cookies. For signed-in videos, enable 'Use my login for videos' in the Sabo DM extension; do not share cookie files or passwords."
	}
	for _, k := range []string{"sign in", "log in", "login", "cookies", "private video",
		"members-only", "members only", "age-restricted", "confirm your age", "not a bot",
		"forbidden", "http error 403", "403: forbidden", "access denied"} {
		if strings.Contains(low, k) {
			return msg + "  >> The learning platform did not authorize this request. Sign in to the same site in this browser, refresh the course page, keep 'Use my login for videos' enabled, and submit the course link again. Do not share passwords, session tokens, or cookie files. If access is still denied, use the platform's official offline viewing option or contact its support. DRM/encrypted streams are not bypassed."
		}
	}
	return msg
}

func anyYouTube(links []string) bool {
	for _, l := range links {
		u, err := url.Parse(l)
		if err != nil {
			continue
		}
		h := strings.ToLower(u.Hostname())
		if h == "youtu.be" || h == "youtube.com" || strings.HasSuffix(h, ".youtube.com") {
			return true
		}
	}
	return false
}

const denoURL = "https://github.com/denoland/deno/releases/latest/download/deno-x86_64-pc-windows-msvc.zip"

// downloadDeno installs the JavaScript engine yt-dlp uses to solve YouTube's checks.
func downloadDeno(ctx context.Context, progress func(done, total int64)) error {
	dst := filepath.Join(toolsDir(), "deno.exe")
	tmp := filepath.Join(toolsDir(), "deno.zip.part")
	if err := fetchFile(ctx, denoURL, tmp, 10<<20, progress); err != nil {
		return err
	}
	defer os.Remove(tmp)
	zr, err := zip.OpenReader(tmp)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, f := range zr.File {
		if !strings.EqualFold(filepath.Base(f.Name), "deno.exe") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(dst + ".part")
		if err != nil {
			rc.Close()
			return err
		}
		_, cerr := io.Copy(out, rc)
		rc.Close()
		if err := out.Close(); cerr == nil {
			cerr = err
		}
		if cerr != nil {
			os.Remove(dst + ".part")
			return cerr
		}
		return os.Rename(dst+".part", dst)
	}
	return errors.New("deno.exe was not found inside the download")
}

// validateFFmpegPE checks the actual PE/COFF structure and architecture. A two-byte
// "MZ" signature alone is not enough: HTML/error payloads and truncated files can
// begin with those bytes yet still fail with Windows error "%1 is not a valid Win32 application".
func validateFFmpegPE(path string) error {
	f, err := pe.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	machine := f.FileHeader.Machine
	switch runtime.GOARCH {
	case "amd64":
		if machine != pe.IMAGE_FILE_MACHINE_AMD64 {
			return fmt.Errorf("wrong executable architecture: PE machine 0x%04x, expected AMD64", machine)
		}
	case "386":
		if machine != pe.IMAGE_FILE_MACHINE_I386 {
			return fmt.Errorf("wrong executable architecture: PE machine 0x%04x, expected x86", machine)
		}
	case "arm64":
		if machine != pe.IMAGE_FILE_MACHINE_ARM64 {
			return fmt.Errorf("wrong executable architecture: PE machine 0x%04x, expected ARM64", machine)
		}
	default:
		return fmt.Errorf("unsupported application architecture: %s", runtime.GOARCH)
	}
	return nil
}

// FFmpeg Windows essentials build. The installer downloads the archive over HTTPS,
// extracts only the two required executables, and stores them in Sabo DM's tools folder.
const ffmpegWindowsURL = "https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip"

func downloadFFmpeg(ctx context.Context, progress func(done, total int64)) error {
	// Never treat file existence as a successful installation. Older builds could
	// leave truncated/non-Windows executables in the tools folder; those files
	// shadow a valid copy elsewhere and prevent a clean reinstall.
	if _, readyErr := ffmpegReady(); readyErr == nil {
		return nil
	}
	if err := os.MkdirAll(toolsDir(), 0755); err != nil {
		return err
	}
	// Remove only the app-managed copies after they have failed execution checks.
	// A fresh download must be able to replace corrupt files from previous runs.
	for _, name := range []string{"ffmpeg.exe", "ffprobe.exe", "ffmpeg.exe.part", "ffprobe.exe.part"} {
		if err := os.Remove(filepath.Join(toolsDir(), name)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("cannot remove unusable %s: %w", name, err)
		}
	}
	// A valid copy may exist beside the application or on PATH; re-check after
	// removing the broken higher-priority tools-folder copy.
	if _, readyErr := ffmpegReady(); readyErr == nil {
		return nil
	}
	archivePath := filepath.Join(toolsDir(), "ffmpeg-essentials.zip.part")
	const maxArchive = int64(250 << 20)

	// The ZIP build is about 109 MB. Keep partial data and resume with HTTP
	// Range after interruptions. The old 8-minute whole-request timeout caused
	// slow links to fail even when bytes were still arriving.
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment, MaxIdleConns: 8, MaxIdleConnsPerHost: 4, IdleConnTimeout: 90 * time.Second, TLSHandshakeTimeout: 30 * time.Second, ResponseHeaderTimeout: 60 * time.Second}
	client := &http.Client{Transport: transport} // no short total-body timeout
	defer transport.CloseIdleConnections()
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		offset := int64(0)
		if st, e := os.Stat(archivePath); e == nil {
			offset = st.Size()
			if offset > maxArchive {
				_ = os.Remove(archivePath)
				offset = 0
			}
		} else if !os.IsNotExist(e) {
			return e
		}
		attemptCtx, cancel := context.WithTimeout(ctx, 45*time.Minute)
		req, e := http.NewRequestWithContext(attemptCtx, http.MethodGet, ffmpegWindowsURL, nil)
		if e != nil {
			cancel()
			return e
		}
		req.Header.Set("User-Agent", "Sabo DM/1.27 FFmpeg installer")
		req.Header.Set("Accept-Encoding", "identity")
		if offset > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
		}
		resp, e := client.Do(req)
		if e != nil {
			cancel()
			lastErr = e
		} else if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
			lastErr = fmt.Errorf("FFmpeg download failed: HTTP %d", resp.StatusCode)
			status := resp.StatusCode
			resp.Body.Close()
			cancel()
			if status == http.StatusRequestedRangeNotSatisfiable || status < 500 {
				_ = os.Remove(archivePath)
			}
		} else {
			appendMode := offset > 0 && resp.StatusCode == http.StatusPartialContent
			if !appendMode {
				offset = 0
			}
			flags := os.O_CREATE | os.O_WRONLY
			if appendMode {
				flags |= os.O_APPEND
			} else {
				flags |= os.O_TRUNC
			}
			f, e := os.OpenFile(archivePath, flags, 0600)
			if e != nil {
				resp.Body.Close()
				cancel()
				return e
			}
			expected := resp.ContentLength
			if expected >= 0 {
				expected += offset
			}
			written := offset
			buf := make([]byte, 1024*1024)
			var readErr error
			for {
				n, re := resp.Body.Read(buf)
				if n > 0 {
					written += int64(n)
					if written > maxArchive {
						readErr = errors.New("FFmpeg archive exceeded the 250 MB safety limit")
						break
					}
					if _, we := f.Write(buf[:n]); we != nil {
						readErr = we
						break
					}
					if progress != nil {
						progress(written, expected)
					}
				}
				if re == io.EOF {
					break
				}
				if re != nil {
					readErr = re
					break
				}
			}
			closeErr := f.Close()
			resp.Body.Close()
			cancel()
			if readErr == nil {
				readErr = closeErr
			}
			if readErr == nil && expected > 0 && written != expected {
				readErr = fmt.Errorf("FFmpeg download incomplete (%d of %d bytes)", written, expected)
			}
			if readErr == nil && written < 1<<20 {
				readErr = errors.New("downloaded FFmpeg archive is too small to be valid")
			}
			lastErr = readErr
			if lastErr == nil {
				break
			}
		}
		if attempt < 4 {
			time.Sleep(time.Duration(1<<attempt) * time.Second)
		}
	}
	if lastErr != nil {
		return fmt.Errorf("FFmpeg download did not finish after 5 attempts; partial data was kept so the next attempt can resume: %w", lastErr)
	}
	if st, e := os.Stat(archivePath); e != nil {
		return e
	} else if st.Size() > maxArchive {
		_ = os.Remove(archivePath)
		return errors.New("FFmpeg archive exceeded the 250 MB safety limit")
	}

	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		_ = os.Remove(archivePath)
		return fmt.Errorf("FFmpeg archive is invalid: %w", err)
	}
	defer zr.Close()
	wanted := map[string]bool{"ffmpeg.exe": false, "ffprobe.exe": false}
	for _, entry := range zr.File {
		base := strings.ToLower(filepath.Base(filepath.FromSlash(entry.Name)))
		if _, ok := wanted[base]; !ok || entry.FileInfo().IsDir() {
			continue
		}
		// Accept only binaries from the archive's bin directory.
		clean := strings.ToLower(filepath.ToSlash(filepath.Clean(entry.Name)))
		if !strings.Contains(clean, "/bin/") {
			continue
		}
		// Do not impose a 100 MiB extraction limit here. Current FFmpeg builds can
		// exceed that size. The old LimitReader silently truncated larger EXEs;
		// PE headers still parsed, but Windows rejected the incomplete image with
		// ERROR_BAD_EXE_FORMAT ("%1 is not a valid Win32 application").
		const maxExecutableSize = uint64(300 << 20)
		if entry.UncompressedSize64 < 1<<20 || entry.UncompressedSize64 > maxExecutableSize {
			_ = os.Remove(archivePath)
			return fmt.Errorf("%s has an invalid uncompressed size (%d bytes)", base, entry.UncompressedSize64)
		}
		rc, openErr := entry.Open()
		if openErr != nil {
			return openErr
		}
		tmpPath := filepath.Join(toolsDir(), base+".part")
		out, createErr := os.Create(tmpPath)
		if createErr != nil {
			rc.Close()
			return createErr
		}
		written, copyErr := io.Copy(out, rc) // read to EOF so ZIP CRC is checked
		rcCloseErr := rc.Close()
		closeErr := out.Close()
		if copyErr == nil {
			copyErr = rcCloseErr
		}
		if copyErr != nil {
			_ = os.Remove(tmpPath)
			_ = os.Remove(archivePath)
			return fmt.Errorf("extracting %s failed (archive data/CRC): %w", base, copyErr)
		}
		if closeErr != nil {
			_ = os.Remove(tmpPath)
			return closeErr
		}
		if uint64(written) != entry.UncompressedSize64 {
			_ = os.Remove(tmpPath)
			_ = os.Remove(archivePath)
			return fmt.Errorf("%s extraction was incomplete: wrote %d of %d bytes", base, written, entry.UncompressedSize64)
		}
		if written < 1<<20 {
			_ = os.Remove(tmpPath)
			return fmt.Errorf("%s failed executable-size validation", base)
		}
		if peErr := validateFFmpegPE(tmpPath); peErr != nil {
			os.Remove(tmpPath)
			_ = os.Remove(archivePath)
			return fmt.Errorf("downloaded %s is not a compatible Windows executable: %w", base, peErr)
		}
		dst := filepath.Join(toolsDir(), base)
		// Windows does not reliably replace an existing executable with Rename.
		// The old invalid file was already execution-tested and removed above.
		if removeErr := os.Remove(dst); removeErr != nil && !os.IsNotExist(removeErr) {
			os.Remove(tmpPath)
			return fmt.Errorf("cannot replace old %s: %w", base, removeErr)
		}
		if renameErr := os.Rename(tmpPath, dst); renameErr != nil {
			os.Remove(tmpPath)
			return renameErr
		}
		wanted[base] = true
	}
	if !wanted["ffmpeg.exe"] || !wanted["ffprobe.exe"] {
		missing := make([]string, 0, 2)
		for _, name := range []string{"ffmpeg.exe", "ffprobe.exe"} {
			if !wanted[name] {
				missing = append(missing, name)
			}
		}
		return fmt.Errorf("FFmpeg archive is incomplete; missing %s", strings.Join(missing, " and "))
	}
	if _, verifyErr := ffmpegReady(); verifyErr != nil {
		for _, name := range []string{"ffmpeg.exe", "ffprobe.exe"} {
			_ = os.Remove(filepath.Join(toolsDir(), name))
		}
		_ = os.Remove(archivePath)
		return fmt.Errorf("FFmpeg files were extracted but failed the execution test: %w", verifyErr)
	}
	_ = os.Remove(archivePath)
	return nil
}

// openFolder opens a folder in Explorer (creating it first if it does not exist yet).
func openFolder(dir string) error {
	if dir == "" {
		return fmt.Errorf("no folder is set")
	}
	dir = filepath.Clean(dir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return exec.Command("explorer.exe", dir).Start()
}

// revealPath opens the folder that holds a file and highlights the file.
// If the file is gone it simply opens the folder.
func revealPath(path string) error {
	path = filepath.Clean(path)
	if _, err := os.Stat(path); err == nil {
		cmd := exec.Command("explorer.exe")
		cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `explorer.exe /select,"` + path + `"`}
		return cmd.Start()
	}
	return openFolder(filepath.Dir(path))
}
