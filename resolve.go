package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"html"
	"io"
	"net/url"
	"regexp"
	"strings"
)

// Resolve turns a "page" link into a direct file link when it knows the site.
// For unknown sites the link is returned unchanged.
// To support another site, add a case below with its own resolveXxx function.
func Resolve(ctx context.Context, link string) (direct, name string, err error) {
	u, perr := url.Parse(link)
	if perr != nil {
		return "", "", perr
	}
	host := strings.ToLower(u.Hostname())
	if (host == "mediafire.com" || strings.HasSuffix(host, ".mediafire.com")) && !strings.HasPrefix(host, "download") {
		return resolveMediaFire(ctx, link)
	}
	return link, "", nil
}

var (
	reScrambled = regexp.MustCompile(`data-scrambled-url="([^"]+)"`)
	reButton    = regexp.MustCompile(`(?s)id="downloadButton"[^>]*?href="([^"]+)"`)
	reDirect    = regexp.MustCompile(`https?://download[0-9a-z-]*\.mediafire\.com/[^"'\s<>\\]+`)
)

func resolveMediaFire(ctx context.Context, link string) (string, string, error) {
	req, err := newRequest(ctx, link, 0, 0, false)
	if err != nil {
		return "", "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("MediaFire page returned HTTP %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	page := string(body)

	direct := ""
	// 1) newer pages hide the link in a base64 attribute
	if m := reScrambled.FindStringSubmatch(page); m != nil {
		if b, derr := base64.StdEncoding.DecodeString(html.UnescapeString(m[1])); derr == nil && strings.HasPrefix(string(b), "http") {
			direct = string(b)
		}
	}
	// 2) the download button's href
	if direct == "" {
		if m := reButton.FindStringSubmatch(page); m != nil {
			if h := html.UnescapeString(m[1]); strings.HasPrefix(h, "http") {
				direct = h
			}
		}
	}
	// 3) any download*.mediafire.com link in the page
	if direct == "" {
		if m := reDirect.FindString(page); m != "" {
			direct = html.UnescapeString(m)
		}
	}
	if direct == "" {
		return "", "", fmt.Errorf("could not find the download link on this MediaFire page (the file may be removed, private, or password-protected)")
	}
	return direct, nameFromURL(direct), nil
}
