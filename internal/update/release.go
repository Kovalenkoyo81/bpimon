// Package update checks GitHub for newer bpimon releases.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Release struct {
	Tag   string
	URL   string
	Notes string
}

// Source returns the latest published release.
type Source interface {
	Latest(ctx context.Context) (Release, error)
}

// GitHub fetches the latest non-draft, non-prerelease release of a repo.
// Results are cached for TTL to stay well under the unauthenticated
// API limit (60 requests/hour).
type GitHub struct {
	Repo    string // "owner/name"
	BaseURL string // default https://api.github.com
	Client  *http.Client
	TTL     time.Duration

	mu       sync.Mutex
	cached   Release
	cachedAt time.Time
}

func NewGitHub(repo string) *GitHub {
	return &GitHub{
		Repo:    repo,
		BaseURL: "https://api.github.com",
		Client:  &http.Client{Timeout: 10 * time.Second},
		TTL:     time.Hour,
	}
}

func (g *GitHub) Latest(ctx context.Context) (Release, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.cachedAt.IsZero() && time.Since(g.cachedAt) < g.TTL {
		return g.cached, nil
	}

	url := strings.TrimRight(g.BaseURL, "/") + "/repos/" + g.Repo + "/releases/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "bpimon")

	resp, err := g.Client.Do(req)
	if err != nil {
		return Release{}, fmt.Errorf("fetch latest release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("fetch latest release: HTTP %d", resp.StatusCode)
	}

	var body struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
		Body    string `json:"body"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Release{}, fmt.Errorf("decode latest release: %w", err)
	}
	if body.TagName == "" {
		return Release{}, fmt.Errorf("latest release has no tag")
	}

	g.cached = Release{Tag: body.TagName, URL: body.HTMLURL, Notes: body.Body}
	g.cachedAt = time.Now()
	return g.cached, nil
}
