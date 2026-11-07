package plugin

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func loadHello(t *testing.T, opts Options) *Plugin {
	t.Helper()
	if opts.DataDir == "" {
		opts.DataDir = t.TempDir()
	}
	p, err := Load("testdata/hello", opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}

func TestCall(t *testing.T) {
	p := loadHello(t, Options{})
	var out struct{ Text string }
	if err := p.Call("hello", map[string]string{"name": "mygo"}, &out); err != nil {
		t.Fatal(err)
	}
	if out.Text != "hello mygo" {
		t.Errorf("got %q", out.Text)
	}
	var n int
	if err := p.Call("add", map[string]int{"a": 2, "b": 3}, &n); err != nil {
		t.Fatal(err)
	}
	if n != 5 {
		t.Errorf("add = %d", n)
	}
}

func TestAsync(t *testing.T) {
	p := loadHello(t, Options{})
	var out string
	if err := p.Call("later", map[string]string{"value": "done"}, &out); err != nil {
		t.Fatal(err)
	}
	if out != "done" {
		t.Errorf("got %q", out)
	}
}

func TestStoragePersists(t *testing.T) {
	dir := t.TempDir()
	p := loadHello(t, Options{DataDir: dir})
	var got map[string]any
	if err := p.Call("store", map[string]any{"key": "session", "value": map[string]any{"token": "abc"}}, &got); err != nil {
		t.Fatal(err)
	}
	if got["token"] != "abc" {
		t.Errorf("store = %v", got)
	}
	p.Close()

	again, err := Load("testdata/hello", Options{DataDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	var out map[string]any
	if err := again.Call("stored", map[string]string{"key": "session"}, &out); err != nil {
		t.Fatal(err)
	}
	if out["token"] != "abc" {
		t.Errorf("stored after restart = %v", out)
	}
}

func TestThrownCode(t *testing.T) {
	p := loadHello(t, Options{})
	err := p.Call("fail", nil, nil)
	if !errors.Is(err, ErrSignIn) {
		t.Fatalf("err = %v, want ErrSignIn", err)
	}
	if !strings.Contains(err.Error(), "sign in please") {
		t.Errorf("message lost: %v", err)
	}
}

func TestCrypto(t *testing.T) {
	p := loadHello(t, Options{})
	var out struct{ MD5, AES string }
	if err := p.Call("hashes", map[string]string{"text": "hello"}, &out); err != nil {
		t.Fatal(err)
	}
	if out.MD5 != "5d41402abc4b2a76b9719d911017c592" {
		t.Errorf("md5 = %s", out.MD5)
	}
	// openssl enc -aes-128-ecb -K 30313233343536373839616263646566 -nosalt
	if out.AES != "674c7ef38e78cabd9cec9c125823a639" {
		t.Errorf("aes = %s", out.AES)
	}
}

func TestNetworkPermission(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("from the server"))
	}))
	defer srv.Close()

	p := loadHello(t, Options{})
	var out string
	if err := p.Call("fetchText", map[string]string{"url": srv.URL}, &out); err != nil {
		t.Fatal(err)
	}
	if out != "from the server" {
		t.Errorf("fetch = %q", out)
	}
}

func TestNetworkDenied(t *testing.T) {
	dir := copyFixture(t, `"permissions": {}`)
	p, err := Load(dir, Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	// With no client, fetch rejects rather than reaching the address.
	if err := p.Call("fetchText", map[string]string{"url": "http://127.0.0.1:1/"}, nil); err == nil {
		t.Fatal("fetch succeeded without network permission")
	}
}

func TestTimeoutRestarts(t *testing.T) {
	p := loadHello(t, Options{CallTimeout: 200 * time.Millisecond})
	err := p.Call("spin", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "did not answer") {
		t.Fatalf("err = %v", err)
	}
	var out struct{ Text string }
	if err := p.CallWithin(5*time.Second, "hello", map[string]string{"name": "again"}, &out); err != nil {
		t.Fatalf("after restart: %v", err)
	}
	if out.Text != "hello again" {
		t.Errorf("got %q", out.Text)
	}
}

func TestNeedsNewerApp(t *testing.T) {
	dir := copyFixture(t, `"minAppVersion": "9.0.0", "permissions": {}`)
	_, err := Load(dir, Options{DataDir: t.TempDir(), AppVersion: "0.1.0"})
	var newer *NeedsNewerApp
	if !errors.As(err, &newer) {
		t.Fatalf("err = %v, want NeedsNewerApp", err)
	}
	if newer.Version != "9.0.0" {
		t.Errorf("version = %s", newer.Version)
	}
}

func TestSpawnDenied(t *testing.T) {
	p := loadHello(t, Options{})
	if err := p.Call("claim", nil, nil); err == nil {
		t.Fatal("spawn succeeded")
	}
}

func TestInspect(t *testing.T) {
	m, exports, err := Inspect("testdata/hello", "")
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "hello" {
		t.Errorf("id = %s", m.ID)
	}
	want := "add,claim,fail,fetchText,hashes,hello,later,spin,store,stored"
	if strings.Join(exports, ",") != want {
		t.Errorf("exports = %v", exports)
	}
}

func TestInstallDirAndList(t *testing.T) {
	plugins := t.TempDir()
	m, err := InstallDir("testdata/hello", plugins)
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "hello" {
		t.Errorf("id = %s", m.ID)
	}
	manifests, err := List(plugins)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifests) != 1 || manifests[0].ID != "hello" {
		t.Errorf("list = %v", manifests)
	}
	if _, err := InstallDir("testdata/hello", plugins); !errors.Is(err, ErrInstalled) {
		t.Errorf("second install = %v", err)
	}
	if err := Uninstall(plugins, "hello"); err != nil {
		t.Fatal(err)
	}
	if manifests, _ := List(plugins); len(manifests) != 0 {
		t.Errorf("still installed: %v", manifests)
	}
}

func TestPackAndInstallZip(t *testing.T) {
	var buf bytes.Buffer
	if err := Pack("testdata/hello", &buf); err != nil {
		t.Fatal(err)
	}
	zipPath := filepath.Join(t.TempDir(), "hello.zip")
	if err := os.WriteFile(zipPath, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	plugins := t.TempDir()
	m, err := InstallZip(zipPath, plugins)
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "hello" {
		t.Errorf("id = %s", m.ID)
	}
	if _, err := Load(filepath.Join(plugins, "hello"), Options{DataDir: t.TempDir()}); err != nil {
		t.Fatalf("installed plugin does not load: %v", err)
	}
}

func TestZipSlip(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("../escape.js")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	zipPath := filepath.Join(t.TempDir(), "evil.zip")
	os.WriteFile(zipPath, buf.Bytes(), 0o644)
	if _, err := InstallZip(zipPath, t.TempDir()); err == nil {
		t.Fatal("zip slip accepted")
	}
}

// TestRealPlugin loads a plugin from MYGO_PLUGIN_REAL, for checking a plugin
// the repository doesn't vendor.
func TestRealPlugin(t *testing.T) {
	dir := os.Getenv("MYGO_PLUGIN_REAL")
	if dir == "" {
		t.Skip("set MYGO_PLUGIN_REAL to a plugin folder")
	}
	p, err := Load(dir, Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if !p.Can("search") {
		t.Errorf("exports = %v, want search", p.Exports())
	}
	raw, _ := json.Marshal(p.Manifest())
	t.Logf("%s", raw)
}

func copyFixture(t *testing.T, overrides string) string {
	t.Helper()
	dir := t.TempDir()
	// Merge the overrides by rewriting the whole manifest around them.
	body := `{"id":"hello","name":"Hello","version":"0.1.0","main":"index.js",` + overrides + `}`
	var merged map[string]any
	if err := json.Unmarshal([]byte(body), &merged); err != nil {
		t.Fatal(err)
	}
	out, _ := json.MarshalIndent(merged, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), out, 0o644); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile("testdata/hello/index.js")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.js"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestRealSearch calls a real plugin over the network, for checking the host
// against a plugin the repository doesn't vendor.
func TestRealSearch(t *testing.T) {
	dir := os.Getenv("MYGO_PLUGIN_REAL")
	if dir == "" || os.Getenv("MYGO_PLUGIN_NETWORK") == "" {
		t.Skip("set MYGO_PLUGIN_REAL and MYGO_PLUGIN_NETWORK")
	}
	p, err := Load(dir, Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	var out struct {
		Tracks []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"tracks"`
	}
	if err := p.CallWithin(30*time.Second, "search", map[string]any{"query": "周杰伦", "kind": "tracks", "offset": 0}, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Tracks) == 0 {
		t.Fatal("search returned no tracks")
	}
	t.Logf("%d tracks, first: %s", len(out.Tracks), out.Tracks[0].Name)
}
