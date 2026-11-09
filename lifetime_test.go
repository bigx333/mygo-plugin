package plugin

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func writeTestPlugin(t *testing.T, manifest, js string) string {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(manifest), 0o644)
	os.WriteFile(filepath.Join(dir, "index.js"), []byte(js), 0o644)
	return dir
}

func TestShutdownIsForGood(t *testing.T) {
	dir := writeTestPlugin(t, `{"id": "s", "name": "S", "main": "index.js"}`, `export function ping() { return 1 }`)
	p, err := Load(dir, Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Call("ping", nil, nil); err != nil {
		t.Fatal(err)
	}
	p.Shutdown()
	if err := p.Call("ping", nil, nil); !errors.Is(err, ErrClosed) {
		t.Fatalf("a call after Shutdown: %v", err)
	}
}

func TestAnIdleRuntimeStopsAndStartsAgain(t *testing.T) {
	dir := writeTestPlugin(t, `{"id": "i", "name": "I", "main": "index.js"}`, `let n = 0; export function next() { return ++n }`)
	p, err := Load(dir, Options{DataDir: t.TempDir(), IdleTimeout: 100 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	var n int
	p.Call("next", nil, &n)
	p.Call("next", nil, &n)
	if n != 2 {
		t.Fatalf("busy: %d", n)
	}
	time.Sleep(400 * time.Millisecond)
	p.Call("next", nil, &n)
	if n != 1 {
		t.Fatalf("after idling: %d, want a fresh start", n)
	}
}

func TestCapabilityAndAllowedAnswerLive(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1); w.Write([]byte("ok")) }))
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")
	dir := writeTestPlugin(t, `{"id": "l", "name": "L", "main": "index.js", "permissions": {"network": ["127.0.0.1"]}}`, `
		export function can() { return host.capability("x") }
		export async function get() { try { return await (await fetch("`+srv.URL+`")).text() } catch (e) { return "refused" } }`)
	var on atomic.Bool
	p, err := Load(dir, Options{DataDir: t.TempDir(),
		Capability: func(_, name string) bool { return name == "x" && on.Load() },
		Allowed:    func(_, h string) bool { return on.Load() && h == strings.Split(host, ":")[0] },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	var can bool
	var got string
	p.Call("can", nil, &can)
	p.Call("get", nil, &got)
	if can || got != "refused" || hits.Load() != 0 {
		t.Fatalf("before: %v %q %d", can, got, hits.Load())
	}
	on.Store(true)
	p.Call("can", nil, &can)
	p.Call("get", nil, &got)
	if !can || got != "ok" {
		t.Fatalf("granted, same runtime: %v %q", can, got)
	}
}
