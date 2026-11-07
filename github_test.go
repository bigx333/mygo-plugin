package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
