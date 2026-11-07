package main

import (
	"os"
	"testing"

	"github.com/egoist/mygo/audio"
	plugin "github.com/yxxbc/mygo-plugin"
	"github.com/yxxbc/mygo-plugin/adapters/music"
)

// TestStream downloads and decodes a real track, when a plugin and network
// are given: MYGO_PLUGIN_REAL=/path/to/plugin MYGO_PLUGIN_STREAM=1. It does
// not play it.
func TestStream(t *testing.T) {
	dir := os.Getenv("MYGO_PLUGIN_REAL")
	if dir == "" || os.Getenv("MYGO_PLUGIN_STREAM") == "" {
		t.Skip("set MYGO_PLUGIN_REAL and MYGO_PLUGIN_STREAM")
	}
	p, err := plugin.Load(dir, plugin.Options{AppVersion: "0.6.0", DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	a := music.New(p)

	res, err := a.Search("周杰伦")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Sections) == 0 || len(res.Sections[0].Items) == 0 {
		t.Fatal("no tracks")
	}
	lastErr := error(nil)
	for i, item := range res.Sections[0].Items {
		if i >= 10 {
			break
		}
		s, err := a.Stream(item.Meta["trackID"].(string))
		if err != nil {
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
			t.Errorf("%s: duration = %v", item.Title, d)
		} else {
			t.Logf("%s: %s, %v", item.Title, s.Type, d)
		}
		player.Close()
		os.Remove(path)
		return
	}
	t.Fatalf("no playable track: %v", lastErr)
}
