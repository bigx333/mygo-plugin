package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

// GitHub reaches GitHub's API, raw files and archives. The zero value uses
// github.com; a Token raises the rate limit, and reads private repositories
// when the token may.
type GitHub struct {
	Client *http.Client
	Token  string
	// API, Raw and Web override the three hosts, for tests.
	API string
	Raw string
	Web string
}

var repoPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

// ValidRepo reports whether s is owner/name.
func ValidRepo(s string) bool { return repoPattern.MatchString(s) }

func (g *GitHub) client() *http.Client {
	if g.Client != nil {
		return g.Client
	}
	return &http.Client{Timeout: 10 * time.Minute}
}

func (g *GitHub) api() string { return strings.TrimSuffix(or(g.API, "https://api.github.com"), "/") }
func (g *GitHub) raw() string {
	return strings.TrimSuffix(or(g.Raw, "https://raw.githubusercontent.com"), "/")
}
func (g *GitHub) web() string { return strings.TrimSuffix(or(g.Web, "https://github.com"), "/") }

func or(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func (g *GitHub) request(ctx context.Context, url string) (*http.Response, error) {
	return g.requestAccept(ctx, url, "application/vnd.github+json")
}

func (g *GitHub) requestAccept(ctx context.Context, url, accept string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "mygo-plugin")
	if g.Token != "" {
		req.Header.Set("Authorization", "Bearer "+g.Token)
	}
	req.Header.Set("Accept", accept)
	return g.client().Do(req)
}

// Manifest reads plugin.json from the repository's default branch.
func (g *GitHub) Manifest(ctx context.Context, repo string) (*Manifest, error) {
	if !ValidRepo(repo) {
		return nil, fmt.Errorf("not a repository, owner/name: %q", repo)
	}
	resp, err := g.request(ctx, g.raw()+"/"+repo+"/HEAD/plugin.json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: plugin.json: %s", repo, resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	m, err := ParseManifest(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", repo, err)
	}
	return m, nil
}

// Repos lists the public repositories with a topic, newest first.
func (g *GitHub) Repos(ctx context.Context, topic string) ([]string, error) {
	if topic == "" {
		return nil, errors.New("no topic")
	}
	var out []string
	seen := map[string]bool{}
	for page := 1; page <= 10; page++ {
		url := fmt.Sprintf("%s/search/repositories?q=topic:%s&per_page=100&page=%d&sort=updated",
			g.api(), topic, page)
		resp, err := g.request(ctx, url)
		if err != nil {
			return nil, err
		}
		var body struct {
			Items []struct {
				FullName string `json:"full_name"`
			} `json:"items"`
		}
		err = json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("GitHub search for %s: %s", topic, resp.Status)
		}
		if err != nil {
			return nil, err
		}
		for _, item := range body.Items {
			if !seen[item.FullName] {
				seen[item.FullName] = true
				out = append(out, item.FullName)
			}
		}
		if len(body.Items) < 100 {
			break
		}
	}
	return out, nil
}

// Install downloads the latest release of repo, or its source at the default
// branch, and installs it into pluginsDir.
func (g *GitHub) Install(ctx context.Context, repo, pluginsDir string) (*Manifest, error) {
	if !ValidRepo(repo) {
		return nil, fmt.Errorf("not a repository, owner/name: %q", repo)
	}
	url, accept, err := g.releaseZip(ctx, repo)
	if err != nil {
		return nil, err
	}
	if url == "" {
		url, accept = g.web()+"/"+repo+"/archive/HEAD.zip", ""
	}
	zipPath, err := g.download(ctx, url, accept)
	if err != nil {
		return nil, err
	}
	defer os.Remove(zipPath)
	return InstallZip(zipPath, pluginsDir)
}

// releaseZip is the zip of the latest release, or "" when there is none.
func (g *GitHub) releaseZip(ctx context.Context, repo string) (url, accept string, err error) {
	resp, err := g.request(ctx, g.api()+"/repos/"+repo+"/releases/latest")
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "", "", nil
	}
	if resp.StatusCode != http.StatusOK {
		// A rate limit must not stop an install that the archive can do.
		return "", "", nil
	}
	var body struct {
		Assets []struct {
			Name string `json:"name"`
			URL  string `json:"url"` // the API asset, which works for private repositories
			Web  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return "", "", err
	}
	for _, a := range body.Assets {
		if !strings.HasSuffix(strings.ToLower(a.Name), ".zip") {
			continue
		}
		// A private repository's browser URL is a 404; its API asset is not.
		if g.Token != "" && a.URL != "" {
			return a.URL, "application/octet-stream", nil
		}
		return a.Web, "", nil
	}
	return "", "", nil
}

func (g *GitHub) download(ctx context.Context, url, accept string) (string, error) {
	if accept == "" {
		accept = "application/vnd.github+json"
	}
	resp, err := g.requestAccept(ctx, url, accept)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %s", url, resp.Status)
	}
	f, err := os.CreateTemp("", "mygo-plugin-*.zip")
	if err != nil {
		return "", err
	}
	n, err := io.Copy(f, io.LimitReader(resp.Body, MaxArchive+1))
	cerr := f.Close()
	if err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(f.Name())
		return "", err
	}
	if n > MaxArchive {
		os.Remove(f.Name())
		return "", fmt.Errorf("%s is larger than %d bytes", url, int64(MaxArchive))
	}
	return f.Name(), nil
}
