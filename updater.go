package main

import (
	"context"
	"crypto/sha256"
	"debug/pe"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const githubRepository = "Mahmoud2014-tester/Sabo-DM"
const githubLatestReleaseAPI = "https://api.github.com/repos/" + githubRepository + "/releases/latest"

type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
	Digest             string `json:"digest"`
}

type githubRelease struct {
	TagName     string         `json:"tag_name"`
	HTMLURL     string         `json:"html_url"`
	Body        string         `json:"body"`
	PublishedAt string         `json:"published_at"`
	Prerelease  bool           `json:"prerelease"`
	Draft       bool           `json:"draft"`
	Assets      []releaseAsset `json:"assets"`
}

type UpdateCheck struct {
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion"`
	UpdateAvailable bool   `json:"updateAvailable"`
	ReleaseURL      string `json:"releaseUrl"`
	PublishedAt     string `json:"publishedAt"`
	Notes           string `json:"notes"`
	HasWindowsAsset bool   `json:"hasWindowsAsset"`
}

func fetchLatestRelease(ctx context.Context) (*githubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubLatestReleaseAPI, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Sabo-DM-Updater/"+AppVersion)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	client := &http.Client{Timeout: 25 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not contact GitHub: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("no published Sabo DM release exists yet; publish a version tag such as v1.3.1 on GitHub first")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("GitHub release check failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var release githubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&release); err != nil {
		return nil, fmt.Errorf("invalid GitHub release response: %w", err)
	}
	if release.Draft || release.Prerelease || strings.TrimSpace(release.TagName) == "" {
		return nil, fmt.Errorf("GitHub did not return a stable published release")
	}
	return &release, nil
}

func compareVersions(a, b string) int {
	clean := func(v string) []int {
		v = strings.TrimPrefix(strings.TrimSpace(v), "v")
		if i := strings.IndexAny(v, "-+"); i >= 0 {
			v = v[:i]
		}
		parts := strings.Split(v, ".")
		out := make([]int, 3)
		for i := 0; i < len(parts) && i < 3; i++ {
			out[i], _ = strconv.Atoi(parts[i])
		}
		return out
	}
	aa, bb := clean(a), clean(b)
	for i := 0; i < 3; i++ {
		if aa[i] < bb[i] {
			return -1
		}
		if aa[i] > bb[i] {
			return 1
		}
	}
	return 0
}

func checkForUpdate(ctx context.Context) (UpdateCheck, error) {
	release, err := fetchLatestRelease(ctx)
	if err != nil {
		return UpdateCheck{}, err
	}
	hasExe := false
	for _, a := range release.Assets {
		if strings.EqualFold(a.Name, "Sabo-DM.exe") && a.BrowserDownloadURL != "" {
			hasExe = true
			break
		}
	}
	return UpdateCheck{CurrentVersion: AppVersion, LatestVersion: release.TagName, UpdateAvailable: compareVersions(release.TagName, AppVersion) > 0,
		ReleaseURL: release.HTMLURL, PublishedAt: release.PublishedAt, Notes: release.Body, HasWindowsAsset: hasExe}, nil
}

func stageLatestUpdate(ctx context.Context) (string, string, error) {
	release, err := fetchLatestRelease(ctx)
	if err != nil {
		return "", "", err
	}
	if compareVersions(release.TagName, AppVersion) <= 0 {
		return "", release.TagName, fmt.Errorf("Sabo DM is already up to date (%s)", AppVersion)
	}
	var asset *releaseAsset
	for i := range release.Assets {
		if strings.EqualFold(release.Assets[i].Name, "Sabo-DM.exe") {
			asset = &release.Assets[i]
			break
		}
	}
	if asset == nil || asset.BrowserDownloadURL == "" {
		return "", release.TagName, fmt.Errorf("release %s does not contain Sabo-DM.exe", release.TagName)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.BrowserDownloadURL, nil)
	if err != nil {
		return "", release.TagName, err
	}
	req.Header.Set("User-Agent", "Sabo-DM-Updater/"+AppVersion)
	resp, err := (&http.Client{Timeout: 10 * time.Minute}).Do(req)
	if err != nil {
		return "", release.TagName, fmt.Errorf("could not download update: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", release.TagName, fmt.Errorf("update download failed (HTTP %d)", resp.StatusCode)
	}
	const maxUpdateBytes = int64(250 << 20)
	if resp.ContentLength > maxUpdateBytes {
		return "", release.TagName, fmt.Errorf("update package exceeds the 250 MB safety limit")
	}
	dir := filepath.Join(configDir(), "updates")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", release.TagName, err
	}
	tmp, err := os.CreateTemp(dir, "Sabo-DM-*.exe.part")
	if err != nil {
		return "", release.TagName, err
	}
	tmpPath := tmp.Name()
	ok := false
	defer func() {
		_ = tmp.Close()
		if !ok {
			_ = os.Remove(tmpPath)
		}
	}()
	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmp, hash), io.LimitReader(resp.Body, maxUpdateBytes+1))
	if err != nil {
		return "", release.TagName, err
	}
	if n < 1<<20 || n > maxUpdateBytes {
		return "", release.TagName, fmt.Errorf("downloaded update has an invalid size (%d bytes)", n)
	}
	if asset.Size > 0 && n != asset.Size {
		return "", release.TagName, fmt.Errorf("update download is incomplete (%d of %d bytes)", n, asset.Size)
	}
	if err := tmp.Sync(); err != nil {
		return "", release.TagName, err
	}
	if err := tmp.Close(); err != nil {
		return "", release.TagName, err
	}
	if !strings.HasPrefix(strings.ToLower(asset.Digest), "sha256:") {
		return "", release.TagName, fmt.Errorf("GitHub release did not provide a SHA-256 digest; refusing an unverifiable update")
	}
	expected := strings.TrimPrefix(strings.ToLower(asset.Digest), "sha256:")
	actual := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(expected, actual) {
		return "", release.TagName, fmt.Errorf("update integrity verification failed (SHA-256 mismatch)")
	}
	peFile, err := pe.Open(tmpPath)
	if err != nil {
		return "", release.TagName, fmt.Errorf("downloaded update is not a valid Windows executable: %w", err)
	}
	machine := peFile.FileHeader.Machine
	_ = peFile.Close()
	if machine != pe.IMAGE_FILE_MACHINE_AMD64 {
		return "", release.TagName, fmt.Errorf("downloaded update has the wrong architecture (expected Windows x64)")
	}
	staged := filepath.Join(dir, "Sabo-DM-update.exe")
	_ = os.Remove(staged)
	if err := os.Rename(tmpPath, staged); err != nil {
		return "", release.TagName, err
	}
	ok = true
	return staged, release.TagName, nil
}

// scheduleSelfUpdate starts a detached helper that waits for this process to
// exit, replaces the current EXE, and launches the updated application.
func scheduleSelfUpdate(staged string) error { return scheduleWindowsSelfUpdate(staged) }
