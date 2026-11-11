// Package update looks up the latest release of a GitHub repository.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// API is the GitHub API root; tests point it at a local server.
var API = "https://api.github.com"

type Release struct {
	Version string // tag without the leading "v"
	URL     string // release page
}

// Latest returns the newest published (non-draft, non-prerelease) release of
// repo ("owner/name").
func Latest(ctx context.Context, client *http.Client, repo string) (Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, API+"/repos/"+repo+"/releases/latest", nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "NextBox")
	resp, err := client.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Release{}, err
	}
	if resp.StatusCode != http.StatusOK {
		// e.g. 403 when the unauthenticated rate limit is used up
		return Release{}, fmt.Errorf("GitHub: HTTP %d", resp.StatusCode)
	}
	var v struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return Release{}, err
	}
	if v.TagName == "" {
		return Release{}, fmt.Errorf("GitHub: no release found for %s", repo)
	}
	return Release{Version: strings.TrimPrefix(v.TagName, "v"), URL: v.HTMLURL}, nil
}

// Newer reports whether version latest is newer than current. Versions look
// like "1.14.4" or "v1.15.0-beta.2"; a pre-release is older than the release
// it leads up to.
func Newer(latest, current string) bool {
	l, lPre := parse(latest)
	c, cPre := parse(current)
	for i := 0; i < len(l) || i < len(c); i++ {
		var a, b int
		if i < len(l) {
			a = l[i]
		}
		if i < len(c) {
			b = c[i]
		}
		if a != b {
			return a > b
		}
	}
	return cPre != "" && lPre == ""
}

func parse(v string) (nums []int, pre string) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	v, pre, _ = strings.Cut(v, "-")
	for _, part := range strings.Split(v, ".") {
		n, _ := strconv.Atoi(part)
		nums = append(nums, n)
	}
	return nums, pre
}
