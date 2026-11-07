package main

import (
	"os"
	"testing"

	plugin "github.com/yxxbc/mygo-plugin"
	"github.com/yxxbc/mygo-plugin/adapters/music"
)

// TestContract reads the real plugin this example is built around, when one is
// given: MYGO_PLUGIN_REAL=/path/to/netease-haruhikage. It is skipped in CI.
func TestContract(t *testing.T) {
	dir := os.Getenv("MYGO_PLUGIN_REAL")
	if dir == "" {
		t.Skip("set MYGO_PLUGIN_REAL to a NetEase plugin folder")
	}
	p, err := plugin.Load(dir, plugin.Options{AppVersion: "0.6.0", DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	a := music.New(p)

	home, err := a.Home()
	if err != nil {
		t.Fatal(err)
	}
	if len(home.Sections) == 0 {
		t.Error("home has no sections")
	}

	res, err := a.Search("周杰伦")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Sections) == 0 || len(res.Sections[0].Items) == 0 {
		t.Fatal("search returned no tracks")
	}
	if res.Sections[0].Items[0].Meta["trackID"] == "" || res.Sections[0].Items[0].Title == "" {
		t.Errorf("item = %+v", res.Sections[0].Items[0])
	}
}
