package main

import (
	"os"
	"testing"

	"github.com/egoist/mygo/audio"
	plugin "github.com/yxxbc/mygo-plugin"
)

// TestStream downloads and decodes a real track, when a plugin and network
// are given: MYGO_PLUGIN_REAL=/path/to/plugin MYGO_PLUGIN_STREAM=1. It does
// not play it.
func TestStream(t *testing.T) {
	dir := os.Getenv("MYGO_PLUGIN_REAL")
	if dir == "" || os.Getenv("MYGO_PLUGIN_STREAM") == "" {
		t.Skip("set MYGO_PLUGIN_REAL and MYGO_PLUGIN_STREAM")
	}
	p, err := plugin.Load(dir, plugin.Options{AppVersion: "0.3.0", DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	var res SearchResult
	if err := p.Call("search", map[string]any{"query": "周杰伦", "kind": "tracks", "offset": 0}, &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Tracks) == 0 {
		t.Fatal("no tracks")
	}
	var lastErr error
	for i, tr := range res.Tracks {
		if i >= 10 {
			break
		}
		var s Stream
		if err := p.Call("stream", map[string]string{"id": tr.ID, "quality": "high"}, &s); err != nil {
			lastErr = err
			continue
		}
		f, path, err := download(s)
		if err != nil {
			lastErr = err
			continue
		}
		player, err := audio.OpenStream(f)
		if err != nil {
			f.Close()
			os.Remove(path)
			lastErr = err
			continue
		}
		if d := player.Duration(); d <= 0 {
			t.Errorf("%s: duration = %v", tr.Name, d)
		} else {
			t.Logf("%s: %s, %v", tr.Name, s.Type, d)
		}
		player.Close()
		os.Remove(path)
		return
	}
	t.Fatalf("no playable track: %v", lastErr)
}
