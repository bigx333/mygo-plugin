package main

import (
	"os"
	"testing"

	plugin "github.com/yxxbc/mygo-plugin"
)

// TestContract reads the real plugin this example is built around, when one is
// given: MYGO_PLUGIN_REAL=/path/to/netease-haruhikage. It is skipped in CI.
func TestContract(t *testing.T) {
	dir := os.Getenv("MYGO_PLUGIN_REAL")
	if dir == "" {
		t.Skip("set MYGO_PLUGIN_REAL to a NetEase plugin folder")
	}
	p, err := plugin.Load(dir, plugin.Options{AppVersion: "0.3.0", DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	var home Page
	if err := p.Call("home", nil, &home); err != nil {
		t.Fatal(err)
	}
	if len(home.Sections) == 0 {
		t.Error("home has no sections")
	}

	var res SearchResult
	if err := p.Call("search", map[string]any{"query": "周杰伦", "kind": "tracks", "offset": 0}, &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Tracks) == 0 {
		t.Fatal("search returned no tracks")
	}
	if res.Tracks[0].ID == "" || res.Tracks[0].Name == "" || len(res.Tracks[0].Artists) == 0 {
		t.Errorf("track = %+v", res.Tracks[0])
	}
}
