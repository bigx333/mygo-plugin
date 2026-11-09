package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// loadSource loads a plugin whose module is source.
func loadSource(t *testing.T, source string, opts Options) *Plugin {
	t.Helper()
	dir := t.TempDir()
	manifest := `{"id": "fork", "name": "Fork", "version": "0.1.0", "main": "index.js"}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.js"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	opts.DataDir = t.TempDir()
	p, err := Load(dir, opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}

type codedError struct{ code, text string }

func (e codedError) Error() string { return e.text }
func (e codedError) Code() string  { return e.code }

const callerSource = `
export async function call({ name, args }) {
  try {
    return { value: await host.call(name, args) };
  } catch (e) {
    return { error: e.message, code: e.code ?? null, isError: e instanceof Error };
  }
}
export async function bare() { return await host.call("echo"); }
export function names() { return host.functions(); }
`

type callResult struct {
	Value   any     `json:"value"`
	Error   string  `json:"error"`
	Code    *string `json:"code"`
	IsError bool    `json:"isError"`
}

func callerPlugin(t *testing.T) *Plugin {
	return loadSource(t, callerSource, Options{Functions: map[string]Function{
		"echo": func(ctx context.Context, args json.RawMessage) (any, error) {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return map[string]any{"got": args}, nil
		},
		"busy": func(context.Context, json.RawMessage) (any, error) {
			return nil, codedError{"busy", "the database is busy"}
		},
		"plain": func(context.Context, json.RawMessage) (any, error) {
			return nil, errors.New("plain failure")
		},
	}})
}

func TestHostCallResolves(t *testing.T) {
	p := callerPlugin(t)
	var out callResult
	if err := p.Call("call", map[string]any{"name": "echo", "args": map[string]any{"n": 1}}, &out); err != nil {
		t.Fatal(err)
	}
	got, _ := json.Marshal(out.Value)
	if string(got) != `{"got":{"n":1}}` || out.Error != "" {
		t.Errorf("echo = %s, error %q", got, out.Error)
	}
	var bare map[string]any
	if err := p.Call("bare", nil, &bare); err != nil {
		t.Fatal(err)
	}
	if v, ok := bare["got"]; !ok || v != nil {
		t.Errorf("host.call without args passed %v, want null", bare)
	}
	var names []string
	if err := p.Call("names", nil, &names); err != nil {
		t.Fatal(err)
	}
	if strings.Join(names, ",") != "busy,echo,plain" {
		t.Errorf("host.functions() = %v", names)
	}
}

func TestHostCallRejects(t *testing.T) {
	p := callerPlugin(t)
	for _, tc := range []struct {
		name, error string
		code        *string
	}{
		{"busy", "the database is busy", ptr("busy")},
		{"plain", "plain failure", nil},
		{"missing", `no host function "missing"`, nil},
	} {
		var out callResult
		if err := p.Call("call", map[string]any{"name": tc.name}, &out); err != nil {
			t.Fatal(err)
		}
		if out.Error != tc.error || !out.IsError || (out.Code == nil) != (tc.code == nil) || (tc.code != nil && *out.Code != *tc.code) {
			t.Errorf("%s rejected with %+v, want %q code %v", tc.name, out, tc.error, tc.code)
		}
	}
}

func ptr(s string) *string { return &s }

func TestHostCallArgsCapped(t *testing.T) {
	called := false
	p := loadSource(t, `export async function big() {
  try { await host.call("f", "x".repeat(16 << 20)); return "resolved"; } catch (e) { return e.message; }
}`, Options{Functions: map[string]Function{"f": func(context.Context, json.RawMessage) (any, error) {
		called = true
		return nil, nil
	}}})
	var out string
	if err := p.Call("big", nil, &out); err != nil {
		t.Fatal(err)
	}
	if called || !strings.Contains(out, "over 16777216 bytes") {
		t.Errorf("called %v, answer %q", called, out)
	}
}

func TestHostCallContextEndsOnClose(t *testing.T) {
	started, ended := make(chan struct{}), make(chan error, 1)
	p := loadSource(t, `export async function wait() { return await host.call("wait"); }`, Options{Functions: map[string]Function{
		"wait": func(ctx context.Context, _ json.RawMessage) (any, error) {
			close(started)
			<-ctx.Done()
			ended <- ctx.Err()
			return nil, ctx.Err()
		},
	}})
	call := make(chan error, 1)
	go func() { call <- p.CallWithin(200*time.Millisecond, "wait", nil, nil) }()
	<-started
	p.Close()
	select {
	case err := <-ended:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("ctx ended with %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("the host function's ctx did not end on Close")
	}
	<-call
}

func TestPrelude(t *testing.T) {
	p := loadSource(t, `const greeting = greet("module");
export async function run() { return { greeting, rows: await db.query("select 1") }; }`, Options{
		Prelude: `function greet(who) { return "hello " + who; }
globalThis.db = { query: (sql) => host.call("query", { sql }) };`,
		Functions: map[string]Function{"query": func(_ context.Context, args json.RawMessage) (any, error) {
			var q struct{ SQL string }
			if err := json.Unmarshal(args, &q); err != nil {
				return nil, err
			}
			return []string{q.SQL}, nil
		}},
	})
	var out struct {
		Greeting string
		Rows     []string
	}
	if err := p.Call("run", nil, &out); err != nil {
		t.Fatal(err)
	}
	if out.Greeting != "hello module" || len(out.Rows) != 1 || out.Rows[0] != "select 1" {
		t.Errorf("got %+v", out)
	}
}

func TestPreludeFails(t *testing.T) {
	p := loadSource(t, `export function run() { return 1; }`, Options{Prelude: `throw new Error("no database");`})
	err := p.Call("run", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "prelude") || !strings.Contains(err.Error(), "no database") {
		t.Errorf("err = %v", err)
	}
	if _, err := Load(p.Dir(), Options{DataDir: t.TempDir(), Prelude: "function ("}); err == nil || !strings.Contains(err.Error(), "Options.Prelude") {
		t.Errorf("a prelude that does not compile loaded: %v", err)
	}
}

// hangOnce is a module whose top level never ends while the host's "hang"
// setting is "yes".
const hangOnce = `if (host.setting("hang") === "yes") { for (;;) {} }
export function ok() { return "ok"; }`

func TestLoadTimeout(t *testing.T) {
	var loads atomic.Int32
	const limit = 300 * time.Millisecond
	p := loadSource(t, hangOnce, Options{LoadTimeout: limit, Host: Host{Setting: func(string, string) string {
		if loads.Add(1) == 1 {
			return "yes"
		}
		return ""
	}}})
	begun := time.Now()
	err := p.Call("ok", nil, nil)
	if err == nil || !strings.Contains(err.Error(), limit.String()) {
		t.Fatalf("err = %v, want one naming %s", err, limit)
	}
	if took := time.Since(begun); took > limit+time.Second {
		t.Errorf("the start failed after %s", took)
	}
	var out string
	if err := p.Call("ok", nil, &out); err != nil || out != "ok" {
		t.Errorf("the next call got %q, %v", out, err)
	}
}

func TestCloseWhileLoading(t *testing.T) {
	var loads atomic.Int32
	loading := make(chan struct{})
	p := loadSource(t, hangOnce, Options{LoadTimeout: time.Minute, Host: Host{Setting: func(string, string) string {
		if loads.Add(1) == 1 {
			close(loading)
			return "yes"
		}
		return ""
	}}})
	call := make(chan error, 1)
	go func() { call <- p.Call("ok", nil, nil) }()
	<-loading
	closed := make(chan struct{})
	go func() {
		p.Close()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("Close waited on a top level that never ends")
	}
	select {
	case err := <-call:
		if err == nil || !strings.Contains(err.Error(), "closed") {
			t.Errorf("err = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("the call went on after Close")
	}
	var out string
	if err := p.Call("ok", nil, &out); err != nil || out != "ok" {
		t.Errorf("after Close got %q, %v", out, err)
	}
}
