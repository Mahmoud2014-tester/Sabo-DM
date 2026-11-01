package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Torrents and magnet links use aria2c (downloaded once, about 6 MB, and
// checked against a known SHA-256), so this app stays small. One aria2c
// process runs per torrent and is controlled over a private local RPC port.

const (
	aria2URL    = "https://github.com/aria2/aria2/releases/download/release-1.37.0/aria2-1.37.0-win-64bit-build1.zip"
	aria2SHA256 = "67d015301eef0b612191212d564c5bb0a14b5b9c4796b76454276a4d28d9b288"
)

var errNoAria2 = errors.New("aria2c.exe not found")

// isTorrentLink is true for magnet links and http(s) links to .torrent files.
func isTorrentLink(l string) bool {
	low := strings.ToLower(strings.TrimSpace(l))
	if strings.HasPrefix(low, "magnet:") {
		return true
	}
	if u, err := url.Parse(strings.TrimSpace(l)); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
		return strings.HasSuffix(strings.ToLower(u.Path), ".torrent")
	}
	return false
}

// ---- installing aria2 ----

func fetchFile(ctx context.Context, link, dst string, minSize int64, progress func(done, total int64)) error {
	req, err := http.NewRequestWithContext(ctx, "GET", link, nil)
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
	f, err := os.Create(dst)
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
				os.Remove(dst)
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
			os.Remove(dst)
			return rerr
		}
	}
	if err := f.Close(); err != nil {
		os.Remove(dst)
		return err
	}
	if done < minSize || (resp.ContentLength > 0 && done != resp.ContentLength) {
		os.Remove(dst)
		return fmt.Errorf("incomplete download")
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func downloadAria2(ctx context.Context, progress func(done, total int64)) error {
	dst := filepath.Join(toolsDir(), "aria2c.exe")
	tmp := filepath.Join(toolsDir(), "aria2.zip.part")
	if err := fetchFile(ctx, aria2URL, tmp, 1<<20, progress); err != nil {
		return err
	}
	defer os.Remove(tmp)
	sum, err := fileSHA256(tmp)
	if err != nil {
		return err
	}
	if sum != aria2SHA256 {
		return fmt.Errorf("the downloaded file failed its security check (checksum mismatch)")
	}
	zr, err := zip.OpenReader(tmp)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, f := range zr.File {
		if !strings.EqualFold(filepath.Base(f.Name), "aria2c.exe") {
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
	return errors.New("aria2c.exe was not found inside the download")
}

// ---- controlling aria2c ----

type rpcClient struct {
	url, secret string
	hc          *http.Client
}

func (c *rpcClient) call(ctx context.Context, method string, params ...interface{}) (json.RawMessage, error) {
	ps := append([]interface{}{"token:" + c.secret}, params...)
	body, err := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": "saber", "method": method, "params": ps})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&out); err != nil {
		return nil, err
	}
	if out.Error != nil {
		return nil, errors.New(out.Error.Message)
	}
	return out.Result, nil
}

type ariaTask struct {
	TotalLength     string `json:"totalLength"`
	CompletedLength string `json:"completedLength"`
	DownloadSpeed   string `json:"downloadSpeed"`
	Connections     string `json:"connections"`
	NumSeeders      string `json:"numSeeders"`
	BitTorrent      struct {
		Info struct {
			Name string `json:"name"`
		} `json:"info"`
	} `json:"bittorrent"`
	Files []struct {
		Path string `json:"path"`
	} `json:"files"`
}

func atoi64(s string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func randHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// runTorrent downloads a magnet link, .torrent URL or local .torrent file.
// Cancelling ctx stops aria2c; running again resumes from the saved progress.
// It does not seed after finishing.
func runTorrent(ctx context.Context, src, dir string, limitKB int64, sel string,
	onUpdate func(done, total int64, speed float64, status string), onName func(string)) error {

	ar := findTool("aria2c.exe")
	if ar == "" {
		return errNoAria2
	}
	port, err := freePort()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	secret := randHex(8)
	args := []string{
		"--enable-rpc", fmt.Sprintf("--rpc-listen-port=%d", port), "--rpc-secret=" + secret,
		"--rpc-listen-all=false", "--dir=" + dir, "--continue=true", "--seed-time=0",
		"--file-allocation=none", "--summary-interval=0", "--console-log-level=error",
		"--enable-color=false", "--follow-torrent=true", "--max-concurrent-downloads=1",
		"--bt-max-peers=100",
	}
	if limitKB > 0 {
		args = append(args, fmt.Sprintf("--max-overall-download-limit=%dK", limitKB))
	}
	if sel != "" {
		args = append(args, "--select-file="+sel) // only the files the user ticked
	}
	args = append(args, src)

	cmd := hidden(exec.CommandContext(ctx, ar, args...))
	var logBuf bytes.Buffer
	cmd.Stdout = &logBuf
	cmd.Stderr = &logBuf
	if err := cmd.Start(); err != nil {
		return err
	}
	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()

	rc := &rpcClient{
		url:    fmt.Sprintf("http://127.0.0.1:%d/jsonrpc", port),
		secret: secret,
		hc:     &http.Client{Timeout: 3 * time.Second},
	}
	keys := []string{"totalLength", "completedLength", "downloadSpeed", "connections", "numSeeders", "files", "bittorrent"}
	lastName := ""
	tick := time.NewTicker(time.Second)
	defer tick.Stop()

	for {
		select {
		case werr := <-waitCh:
			if ctx.Err() != nil {
				return ctx.Err() // paused
			}
			if werr != nil {
				msg := lastLine(logBuf.String())
				if msg == "" {
					msg = werr.Error()
				}
				return errors.New(msg)
			}
			return nil // finished
		case <-tick.C:
			res, err := rc.call(ctx, "aria2.tellActive", keys)
			if err != nil {
				continue // aria2 is still starting up
			}
			var tasks []ariaTask
			if json.Unmarshal(res, &tasks) != nil || len(tasks) == 0 {
				continue
			}
			t := tasks[0]
			done, total, speed := atoi64(t.CompletedLength), atoi64(t.TotalLength), float64(atoi64(t.DownloadSpeed))
			name := t.BitTorrent.Info.Name
			if name == "" && len(t.Files) > 0 {
				name = filepath.Base(t.Files[0].Path)
			}
			meta := len(t.Files) > 0 && strings.Contains(t.Files[0].Path, "[METADATA]")
			status := "Connecting to peers..."
			switch {
			case meta:
				status = "Fetching torrent info..."
				done, total, speed = 0, 0, 0
			case total > 0:
				status = fmt.Sprintf("Downloading (seeds %d, connections %d)", atoi64(t.NumSeeders), atoi64(t.Connections))
			}
			if !meta && name != "" && name != lastName {
				lastName = name
				if onName != nil {
					onName(name)
				}
			}
			if onUpdate != nil {
				onUpdate(done, total, speed, status)
			}
		}
	}
}

// ---- reading .torrent files, so the user can choose which files to download ----

type torrentFile struct {
	Path string
	Size int64
	Pad  bool // BEP 47 padding file (hidden, always kept)
}

type torrentMeta struct {
	Name  string
	Hash  string
	Files []torrentFile
	Path  string // our saved copy of the .torrent file
}

// visible counts the files the user can actually choose between.
func (m *torrentMeta) visible() int {
	n := 0
	for _, f := range m.Files {
		if !f.Pad {
			n++
		}
	}
	return n
}

var errBencode = errors.New("not a valid .torrent file")

type bdec struct {
	b    []byte
	i    int
	info []byte // raw bytes of the top-level "info" value (its SHA-1 is the torrent's id)
}

func (d *bdec) value(depth int) (interface{}, error) {
	if depth > 32 || d.i >= len(d.b) {
		return nil, errBencode
	}
	c := d.b[d.i]
	switch {
	case c == 'i':
		d.i++
		j := bytes.IndexByte(d.b[d.i:], 'e')
		if j < 0 {
			return nil, errBencode
		}
		n, err := strconv.ParseInt(string(d.b[d.i:d.i+j]), 10, 64)
		if err != nil {
			return nil, errBencode
		}
		d.i += j + 1
		return n, nil
	case c == 'l':
		d.i++
		var list []interface{}
		for {
			if d.i >= len(d.b) {
				return nil, errBencode
			}
			if d.b[d.i] == 'e' {
				d.i++
				return list, nil
			}
			v, err := d.value(depth + 1)
			if err != nil {
				return nil, err
			}
			list = append(list, v)
		}
	case c == 'd':
		d.i++
		m := map[string]interface{}{}
		for {
			if d.i >= len(d.b) {
				return nil, errBencode
			}
			if d.b[d.i] == 'e' {
				d.i++
				return m, nil
			}
			k, err := d.value(depth + 1)
			ks, isStr := k.(string)
			if err != nil || !isStr {
				return nil, errBencode
			}
			start := d.i
			v, err := d.value(depth + 1)
			if err != nil {
				return nil, err
			}
			if depth == 0 && ks == "info" {
				d.info = d.b[start:d.i]
			}
			m[ks] = v
		}
	case c >= '0' && c <= '9':
		j := bytes.IndexByte(d.b[d.i:], ':')
		if j < 0 {
			return nil, errBencode
		}
		n, err := strconv.Atoi(string(d.b[d.i : d.i+j]))
		if err != nil || n < 0 || d.i+j+1+n > len(d.b) {
			return nil, errBencode
		}
		s := string(d.b[d.i+j+1 : d.i+j+1+n])
		d.i += j + 1 + n
		return s, nil
	}
	return nil, errBencode
}

func parseTorrent(data []byte) (*torrentMeta, error) {
	d := &bdec{b: data}
	top, err := d.value(0)
	if err != nil {
		return nil, err
	}
	root, ok := top.(map[string]interface{})
	if !ok || d.info == nil {
		return nil, errBencode
	}
	info, ok := root["info"].(map[string]interface{})
	if !ok {
		return nil, errBencode
	}
	sum := sha1.Sum(d.info)
	m := &torrentMeta{Hash: hex.EncodeToString(sum[:])}
	m.Name, _ = info["name.utf-8"].(string)
	if m.Name == "" {
		m.Name, _ = info["name"].(string)
	}
	if m.Name == "" {
		m.Name = m.Hash
	}
	if list, isList := info["files"].([]interface{}); isList {
		for _, it := range list {
			fm, ok := it.(map[string]interface{})
			if !ok {
				return nil, errBencode
			}
			size, _ := fm["length"].(int64)
			pl, _ := fm["path.utf-8"].([]interface{})
			if pl == nil {
				pl, _ = fm["path"].([]interface{})
			}
			var parts []string
			for _, p := range pl {
				if s, isStr := p.(string); isStr {
					parts = append(parts, s)
				}
			}
			if len(parts) == 0 {
				return nil, errBencode
			}
			attr, _ := fm["attr"].(string)
			m.Files = append(m.Files, torrentFile{Path: strings.Join(parts, "/"), Size: size, Pad: strings.Contains(attr, "p")})
		}
	} else if size, isInt := info["length"].(int64); isInt {
		m.Files = []torrentFile{{Path: m.Name, Size: size}}
	}
	if len(m.Files) == 0 {
		return nil, errBencode
	}
	return m, nil
}

// fetchMagnetMeta asks aria2 to download only the torrent's file list from peers.
func fetchMagnetMeta(ctx context.Context, magnet string) ([]byte, error) {
	ar := findTool("aria2c.exe")
	if ar == "" {
		return nil, errNoAria2
	}
	tmp, err := os.MkdirTemp("", "saber-meta-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	cmd := hidden(exec.CommandContext(ctx, ar, "--bt-metadata-only=true", "--bt-save-metadata=true",
		"--dir="+tmp, "--console-log-level=error", "--summary-interval=0", "--enable-color=false",
		"--seed-time=0", magnet))
	var logBuf bytes.Buffer
	cmd.Stdout = &logBuf
	cmd.Stderr = &logBuf
	runErr := cmd.Run()
	if files, _ := filepath.Glob(filepath.Join(tmp, "*.torrent")); len(files) > 0 {
		return os.ReadFile(files[0])
	}
	if ctx.Err() == context.DeadlineExceeded {
		return nil, errors.New("no peers answered within 3 minutes, so the file list could not be read (the magnet link may be dead)")
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	msg := lastLine(logBuf.String())
	if msg == "" && runErr != nil {
		msg = runErr.Error()
	}
	if msg == "" {
		msg = "could not read the magnet link"
	}
	return nil, errors.New(msg)
}

func loadTorrentBytes(ctx context.Context, src string) ([]byte, error) {
	src = strings.TrimSpace(src)
	low := strings.ToLower(src)
	switch {
	case strings.HasPrefix(low, "magnet:"):
		return fetchMagnetMeta(ctx, src)
	case strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://"):
		req, err := newRequest(ctx, src, 0, 0, false)
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("server returned HTTP %d", resp.StatusCode)
		}
		return io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	default:
		return os.ReadFile(src)
	}
}

// prepareTorrent reads the file list and keeps a copy of the .torrent file,
// so resuming later never needs the magnet link or the web address again.
func prepareTorrent(ctx context.Context, src string) (*torrentMeta, error) {
	data, err := loadTorrentBytes(ctx, src)
	if err != nil {
		return nil, err
	}
	m, err := parseTorrent(data)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(configDir(), "torrents")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	m.Path = filepath.Join(dir, m.Hash+".torrent")
	if err := os.WriteFile(m.Path, data, 0644); err != nil {
		return nil, err
	}
	return m, nil
}
