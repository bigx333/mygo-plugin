package plugin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/MoeForge/moejs"
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

// maxModule is the largest module Inspect reads.
const maxModule = 8 << 20

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

// RepoInfo is what GitHub says of a repository, for the registry's trust
// signals.
type RepoInfo struct {
	Stars    int        `json:"stargazers_count"`
	PushedAt *time.Time `json:"pushed_at"`
	Archived bool       `json:"archived"`
	License  *struct {
		SPDXID string `json:"spdx_id"`
	} `json:"license"`
}

// Repo reads a repository's metadata.
func (g *GitHub) Repo(ctx context.Context, repo string) (*RepoInfo, error) {
	if !ValidRepo(repo) {
		return nil, fmt.Errorf("not a repository, owner/name: %q", repo)
	}
	resp, err := g.request(ctx, g.api()+"/repos/"+repo)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", repo, resp.Status)
	}
	var info RepoInfo
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}

// Manifest reads plugin.json from the repository's default branch.
func (g *GitHub) Manifest(ctx context.Context, repo string) (*Manifest, error) {
	if !ValidRepo(repo) {
		return nil, fmt.Errorf("not a repository, owner/name: %q", repo)
	}
	b, err := g.rawFile(ctx, repo, "plugin.json", 1<<20)
	if err != nil {
		return nil, err
	}
	m, err := ParseManifest(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", repo, err)
	}
	return m, nil
}

// Inspect reads a repository's plugin.json and its main module from the
// default branch, and compiles the module without running it: what the
// manifest says and the exports the host can call.
func (g *GitHub) Inspect(ctx context.Context, repo string) (*Manifest, []string, error) {
	m, err := g.Manifest(ctx, repo)
	if err != nil {
		return nil, nil, err
	}
	exports, err := g.compileMain(ctx, repo, m)
	if err != nil {
		return nil, nil, err
	}
	return m, exports, nil
}

// compileMain reads the module a manifest names from the default branch and
// compiles it with the already read manifest, without running it. It is the
// second half of Inspect, so a caller that has the manifest pays one request.
func (g *GitHub) compileMain(ctx context.Context, repo string, m *Manifest) ([]string, error) {
	source, err := g.rawFile(ctx, repo, m.Main, maxModule)
	if err != nil {
		return nil, err
	}
	compile := moejs.Compile
	if strings.HasSuffix(m.Main, ".ts") {
		compile = moejs.CompileTS
	}
	mod, err := compile(m.Main, string(source))
	if err != nil {
		return nil, fmt.Errorf("%s: %s: %w", repo, m.Main, err)
	}
	exports := append([]string(nil), mod.Exports()...)
	sort.Strings(exports)
	return exports, nil
}

// rawFile reads a file of the repository's default branch.
func (g *GitHub) rawFile(ctx context.Context, repo, name string, limit int64) ([]byte, error) {
	resp, err := g.request(ctx, g.raw()+"/"+repo+"/HEAD/"+escapeSegments(name))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s: %s", repo, name, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, limit))
}

// escapeSegments escapes each element of a slash-separated path, so that a
// file name with a space or a question mark reaches the right file.
func escapeSegments(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		parts[i] = url.PathEscape(s)
	}
	return strings.Join(parts, "/")
}

// Repos lists the public repositories with a topic, newest first.
func (g *GitHub) Repos(ctx context.Context, topic string) ([]string, error) {
	if topic == "" {
		return nil, errors.New("no topic")
	}
	var out []string
	seen := map[string]bool{}
	for page := 1; page <= 10; page++ {
		query := url.QueryEscape("topic:" + topic + " is:public")
		url := fmt.Sprintf("%s/search/repositories?q=%s&per_page=100&page=%d&sort=updated",
			g.api(), query, page)
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
	return g.InstallVersion(ctx, repo, "", "", pluginsDir)
}

// InstallVersion installs the release of repo at version, or the latest when
// version is empty. Only the latest falls back to the source at the default
// branch: a version that has no release is an error. A wantSHA, hex with or
// without a "sha256:" prefix, must match the zip before it is installed.
func (g *GitHub) InstallVersion(ctx context.Context, repo, version, wantSHA, pluginsDir string) (*Manifest, error) {
	if !ValidRepo(repo) {
		return nil, fmt.Errorf("not a repository, owner/name: %q", repo)
	}
	zipURL, accept, found, err := g.releaseZip(ctx, repo, version)
	if err != nil {
		return nil, err
	}
	if zipURL == "" {
		if version != "" {
			if !found {
				return nil, fmt.Errorf("%s: no release for version %s", repo, version)
			}
			return nil, fmt.Errorf("%s: release %s has no zip asset", repo, version)
		}
		zipURL, accept = g.web()+"/"+repo+"/archive/HEAD.zip", ""
	}
	zipPath, err := g.download(ctx, zipURL, accept)
	if err != nil {
		return nil, err
	}
	defer os.Remove(zipPath)
	if wantSHA != "" {
		if err := checkSHA256(zipPath, wantSHA); err != nil {
			return nil, err
		}
	}
	return InstallZip(zipPath, pluginsDir)
}

// checkSHA256 checks that the file's sha256 is want, in either case, which
// GitHub writes as "sha256:...".
func checkSHA256(path, want string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, stripDigest(want)) {
		return fmt.Errorf("sha256 is %s, not %s", got, want)
	}
	return nil
}

// stripDigest removes the "sha256:" prefix GitHub puts on a digest.
func stripDigest(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 7 && strings.EqualFold(s[:7], "sha256:") {
		s = s[7:]
	}
	return s
}

// releaseZip is the zip of a release: the latest when version is empty, or the
// one tagged v<version> or <version>. found says the release exists; it is
// false when there is none or, for the latest, the API doesn't answer, so that
// the install falls back to the source.
func (g *GitHub) releaseZip(ctx context.Context, repo, version string) (zipURL, accept string, found bool, err error) {
	if version == "" {
		return g.releaseAsset(ctx, g.api()+"/repos/"+repo+"/releases/latest", false)
	}
	zipURL, accept, found, err = g.releaseAsset(ctx, g.api()+"/repos/"+repo+"/releases/tags/v"+url.PathEscape(version), true)
	if err != nil || found {
		return zipURL, accept, found, err
	}
	return g.releaseAsset(ctx, g.api()+"/repos/"+repo+"/releases/tags/"+url.PathEscape(version), true)
}

// releaseAsset is the first zip asset of a release, by its API URL when a
// token is set and its browser URL otherwise. A release that isn't there is
// found false. With strict, any other status is an error; without it a rate
// limit is not, so that an install can fall back to the source.
func (g *GitHub) releaseAsset(ctx context.Context, releaseURL string, strict bool) (zipURL, accept string, found bool, err error) {
	resp, err := g.request(ctx, releaseURL)
	if err != nil {
		return "", "", false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "", "", false, nil
	}
	if resp.StatusCode != http.StatusOK {
		if strict {
			return "", "", false, fmt.Errorf("%s: %s", releaseURL, resp.Status)
		}
		// A rate limit must not stop an install that the archive can do.
		return "", "", false, nil
	}
	var body struct {
		Assets []struct {
			Name string `json:"name"`
			URL  string `json:"url"` // the API asset, which works for private repositories
			Web  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return "", "", true, err
	}
	for _, a := range body.Assets {
		if !strings.HasSuffix(strings.ToLower(a.Name), ".zip") {
			continue
		}
		// A private repository's browser URL is a 404; its API asset is not.
		if g.Token != "" && a.URL != "" {
			return a.URL, "application/octet-stream", true, nil
		}
		return a.Web, "", true, nil
	}
	return "", "", true, nil
}

// releaseSHA256 is the sha256 GitHub lists for the latest release's first zip
// asset, or "" when there is no release, no token, or the API doesn't answer:
// a missing digest shouldn't fail a build, and a build shouldn't download.
func (g *GitHub) releaseSHA256(ctx context.Context, repo string) string {
	if g.Token == "" {
		return ""
	}
	resp, err := g.request(ctx, g.api()+"/repos/"+repo+"/releases/latest")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var body struct {
		Assets []struct {
			Name   string `json:"name"`
			Digest string `json:"digest"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return ""
	}
	for _, a := range body.Assets {
		if strings.HasSuffix(strings.ToLower(a.Name), ".zip") {
			return stripDigest(a.Digest)
		}
	}
	return ""
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
