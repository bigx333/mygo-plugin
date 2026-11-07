package plugin

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestBuiltinNavigationNeedsNoTitle(t *testing.T) {
	m, err := ParseManifest([]byte(`{"id":"x","main":"index.js","navigation":[{"builtin":"home"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Navigation) != 1 || m.Navigation[0].Builtin != "home" {
		t.Errorf("navigation = %+v", m.Navigation)
	}
	if _, err := ParseManifest([]byte(`{"id":"x","main":"index.js","navigation":[{"page":"daily"}]}`)); err == nil {
		t.Error("a page without a title loaded")
	}
}

func TestGitHubManifest(t *testing.T) {
	fixture, err := os.ReadFile("testdata/hello/plugin.json")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/owner/hello/HEAD/plugin.json" {
			http.NotFound(w, r)
			return
		}
		w.Write(fixture)
	}))
	defer srv.Close()
	m, err := (&GitHub{Raw: srv.URL}).Manifest(context.Background(), "owner/hello")
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "hello" {
		t.Errorf("id = %s", m.ID)
	}
	if _, err := (&GitHub{Raw: srv.URL}).Manifest(context.Background(), "not a repo"); err == nil {
		t.Error("bad repository accepted")
	}
}

func TestGitHubRepos(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/search/repositories") {
			http.NotFound(w, r)
			return
		}
		if q := r.URL.Query().Get("q"); q != "topic:mygo-music-plugin is:public" {
			t.Errorf("q = %q", q)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]string{{"full_name": "a/one"}, {"full_name": "b/two"}, {"full_name": "a/one"}},
		})
	}))
	defer srv.Close()
	repos, err := (&GitHub{API: srv.URL}).Repos(context.Background(), "mygo-music-plugin")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(repos, ",") != "a/one,b/two" {
		t.Errorf("repos = %v", repos)
	}
}

func TestGitHubInstall(t *testing.T) {
	var buf bytes.Buffer
	if err := Pack("testdata/hello", &buf); err != nil {
		t.Fatal(err)
	}
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/hello/releases/latest":
			json.NewEncoder(w).Encode(map[string]any{
				"assets": []map[string]string{{"name": "hello.zip", "browser_download_url": srv.URL + "/asset.zip"}},
			})
		case "/asset.zip":
			w.Write(buf.Bytes())
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	plugins := t.TempDir()
	m, err := (&GitHub{API: srv.URL, Web: srv.URL, Raw: srv.URL}).Install(context.Background(), "owner/hello", plugins)
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "hello" {
		t.Errorf("id = %s", m.ID)
	}
	if _, err := os.Stat(filepath.Join(plugins, "hello", "index.js")); err != nil {
		t.Errorf("not installed: %v", err)
	}
}

func TestBuildRegistryDedup(t *testing.T) {
	one := []byte(`{"id":"same","name":"One","version":"1.0.0","main":"index.js"}`)
	two := []byte(`{"id":"same","name":"Two","version":"2.0.0","main":"index.js"}`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/a/one/HEAD/plugin.json":
			w.Write(one)
		case "/b/two/HEAD/plugin.json":
			w.Write(two)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	reg, failures := BuildRegistry(context.Background(), &GitHub{Raw: srv.URL}, []string{"a/one", "b/two", "c/missing"})
	if len(failures) != 1 {
		t.Errorf("failures = %v", failures)
	}
	if len(reg.Entries) != 1 {
		t.Fatalf("entries = %+v", reg.Entries)
	}
	if reg.Entries[0].Version != "2.0.0" || reg.Entries[0].Repo != "b/two" {
		t.Errorf("entry = %+v", reg.Entries[0])
	}
	if reg.Schema != RegistrySchema || reg.GeneratedAt.IsZero() {
		t.Errorf("registry = %+v", reg)
	}
}

func TestGitHubInspect(t *testing.T) {
	plugin := []byte(`{"id":"hello","name":"Hello","version":"0.1.0","main":"dist/index.js"}`)
	module := []byte("export function zebra() {}\nexport function alpha() { return 1; }\n")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/owner/hello/HEAD/plugin.json":
			w.Write(plugin)
		case "/owner/hello/HEAD/dist/index.js":
			w.Write(module)
		case "/owner/nada/HEAD/plugin.json":
			w.Write([]byte(`{"id":"nada","name":"Nada","main":"index.js"}`))
		case "/owner/ts/HEAD/plugin.json":
			w.Write([]byte(`{"id":"ts","name":"TS","main":"index.ts"}`))
		case "/owner/ts/HEAD/index.ts":
			w.Write([]byte("export function typed(x: number): number { return x + 1; }\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	g := &GitHub{Raw: srv.URL}
	m, exports, err := g.Inspect(context.Background(), "owner/hello")
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "hello" || m.Main != "dist/index.js" {
		t.Errorf("manifest = %+v", m)
	}
	if strings.Join(exports, ",") != "alpha,zebra" {
		t.Errorf("exports = %v", exports)
	}
	// A manifest whose module is missing is not a plugin to inspect.
	if _, _, err := g.Inspect(context.Background(), "owner/nada"); err == nil {
		t.Error("inspected a repository with no main module")
	}
	// A TypeScript main compiles as TypeScript.
	if _, exports, err := g.Inspect(context.Background(), "owner/ts"); err != nil {
		t.Error(err)
	} else if strings.Join(exports, ",") != "typed" {
		t.Errorf("TypeScript exports = %v", exports)
	}
}

func TestBuildRegistryHealth(t *testing.T) {
	var raw atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw.Add(1)
		switch r.URL.Path {
		case "/a/good/HEAD/plugin.json":
			w.Write([]byte(`{"id":"good","name":"Good","version":"1.0.0","main":"index.js"}`))
		case "/a/good/HEAD/index.js":
			w.Write([]byte("export function hello() { return 1; }\n"))
		case "/b/bad/HEAD/plugin.json":
			w.Write([]byte(`{"id":"bad","name":"Bad","version":"1.0.0","main":"index.js"}`))
		case "/b/bad/HEAD/index.js":
			w.Write([]byte("export function broken( {\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	reg, failures := BuildRegistry(context.Background(), &GitHub{Raw: srv.URL}, []string{"a/good", "b/bad"})
	if len(failures) != 0 {
		t.Errorf("failures = %v", failures)
	}
	byID := map[string]RegistryEntry{}
	for _, e := range reg.Entries {
		byID[e.ID] = e
	}
	good, ok := byID["good"]
	if !ok {
		t.Fatalf("entries = %+v", reg.Entries)
	}
	if good.Health == nil || good.Health.Status != "ok" {
		t.Fatalf("good health = %+v", good.Health)
	}
	if strings.Join(good.Health.Exports, ",") != "hello" {
		t.Errorf("good exports = %v", good.Health.Exports)
	}
	if good.Health.CheckedAt.IsZero() {
		t.Error("good health has no check time")
	}
	// A module that doesn't compile keeps its entry, with the error.
	bad, ok := byID["bad"]
	if !ok {
		t.Fatalf("a broken module was dropped: %+v", reg.Entries)
	}
	if bad.Health == nil || bad.Health.Status != "error" || bad.Health.Error == "" {
		t.Errorf("bad health = %+v", bad.Health)
	}
	if bad.Health.CheckedAt.IsZero() {
		t.Error("bad health has no check time")
	}
	// A manifest and a module each, and nothing else: the health check adds
	// one raw request per repository, not a second plugin.json.
	if n := raw.Load(); n != 4 {
		t.Errorf("raw requests = %d, want 4", n)
	}
}

func TestGitHubInstallVersion(t *testing.T) {
	var buf bytes.Buffer
	if err := Pack("testdata/hello", &buf); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(buf.Bytes())
	sha := hex.EncodeToString(sum[:])
	var asked []string
	var mu sync.Mutex
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		asked = append(asked, r.URL.Path)
		mu.Unlock()
		switch r.URL.Path {
		case "/repos/owner/hello/releases/tags/v1.2.0":
			json.NewEncoder(w).Encode(map[string]any{
				"assets": []map[string]string{{"name": "hello.zip", "browser_download_url": srv.URL + "/asset.zip"}},
			})
		case "/asset.zip":
			w.Write(buf.Bytes())
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	g := &GitHub{API: srv.URL, Web: srv.URL, Raw: srv.URL}
	plugins := t.TempDir()
	m, err := g.InstallVersion(context.Background(), "owner/hello", "1.2.0", strings.ToUpper(sha), plugins)
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "hello" {
		t.Errorf("id = %s", m.ID)
	}
	if _, err := os.Stat(filepath.Join(plugins, "hello", "index.js")); err != nil {
		t.Errorf("not installed: %v", err)
	}
	// A zip whose sha256 is not the wanted one is refused before install.
	other := t.TempDir()
	if _, err := g.InstallVersion(context.Background(), "owner/hello", "1.2.0", strings.Repeat("0", 64), other); err == nil {
		t.Error("a zip with the wrong sha256 was installed")
	}
	if entries, err := os.ReadDir(other); err != nil || len(entries) != 0 {
		t.Errorf("installed after a wrong sha256: %v, %v", entries, err)
	}
	// The v-prefixed tag is tried first; the bare one and latest never are.
	seen := map[string]bool{}
	mu.Lock()
	for _, p := range asked {
		seen[p] = true
	}
	mu.Unlock()
	if !seen["/repos/owner/hello/releases/tags/v1.2.0"] {
		t.Errorf("asked %v", asked)
	}
	for _, p := range []string{"/repos/owner/hello/releases/tags/1.2.0", "/repos/owner/hello/releases/latest"} {
		if seen[p] {
			t.Errorf("asked %s for a pinned version", p)
		}
	}
}

func TestBuildRegistrySHA256(t *testing.T) {
	var latest atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/a/x/HEAD/plugin.json":
			w.Write([]byte(`{"id":"x","name":"X","version":"1.0.0","main":"index.js"}`))
		case "/a/x/HEAD/index.js":
			w.Write([]byte("export const a = 1;\n"))
		case "/b/y/HEAD/plugin.json":
			w.Write([]byte(`{"id":"y","name":"Y","version":"1.0.0","main":"index.js"}`))
		case "/b/y/HEAD/index.js":
			w.Write([]byte("export const b = 2;\n"))
		case "/repos/a/x/releases/latest":
			latest.Add(1)
			json.NewEncoder(w).Encode(map[string]any{"assets": []map[string]string{
				{"name": "notes.txt", "digest": "sha256:1111"},
				{"name": "x.zip", "digest": "sha256:ABCDEF0123"},
			}})
		case "/repos/b/y/releases/latest":
			latest.Add(1)
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	reg, failures := BuildRegistry(context.Background(), &GitHub{API: srv.URL, Raw: srv.URL, Token: "t"}, []string{"a/x", "b/y"})
	if len(failures) != 0 {
		t.Errorf("failures = %v", failures)
	}
	byID := map[string]RegistryEntry{}
	for _, e := range reg.Entries {
		byID[e.ID] = e
	}
	if got := byID["x"].SHA256; got != "ABCDEF0123" {
		t.Errorf("sha256 = %q", got)
	}
	// A repository with no release keeps its entry, without a digest.
	if y, ok := byID["y"]; !ok {
		t.Errorf("a repository with no release was dropped: %+v", reg.Entries)
	} else if y.SHA256 != "" {
		t.Errorf("sha256 without a release = %q", y.SHA256)
	}
	// Without a token the build asks the API nothing.
	if _, failures := BuildRegistry(context.Background(), &GitHub{API: srv.URL, Raw: srv.URL}, []string{"a/x", "b/y"}); len(failures) != 0 {
		t.Errorf("failures = %v", failures)
	}
	if n := latest.Load(); n != 2 {
		t.Errorf("release requests = %d, want 2", n)
	}
}

func TestRegistryFile(t *testing.T) {
	reg, err := ReadRegistry("registry.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.Entries) == 0 {
		t.Fatal("the registry is empty")
	}
	seen := map[string]bool{}
	for _, e := range reg.Entries {
		if seen[e.ID] {
			t.Errorf("duplicate id %q", e.ID)
		}
		seen[e.ID] = true
		if !ValidRepo(e.Repo) {
			t.Errorf("%s: repo %q", e.ID, e.Repo)
		}
	}
}

func TestRegistryRoundTrip(t *testing.T) {
	reg := &Registry{Schema: RegistrySchema, Entries: []RegistryEntry{{ID: "x", Repo: "a/b"}}}
	path := filepath.Join(t.TempDir(), "registry.json")
	if err := reg.WriteFile(path); err != nil {
		t.Fatal(err)
	}
	got, err := ReadRegistry(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Entries) != 1 || got.Entries[0].ID != "x" {
		t.Errorf("got %+v", got)
	}
}
