// Command host is a reference MyGo app that runs plugins from mygo-plugin in
// native UI: no web page, no WebView. It shows the plugin's navigation, calls
// browse and search, renders the rows itself, and plays a track's stream in
// Go.
//
//	mygo-plugin install netease-haruhikage ./plugins
//	go run . -plugins ./plugins
package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	_ "github.com/egoist/mygo/audio/mp3"
	"github.com/egoist/mygo/ui"
)

func main() {
	pluginsDir := flag.String("plugins", "plugins", "the folder of plugins")
	dataDir := flag.String("data", "", "where plugin data lives; the user's config folder by default")
	flag.Parse()
	if *dataDir == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			log.Fatal(err)
		}
		*dataDir = filepath.Join(dir, "mygo-plugin-host")
	}
	h := newHost(*pluginsDir, *dataDir)
	mygo.App.WhenReady(func() {
		h.win = mygo.NewWindow(mygo.WindowOptions{
			Title:   "mygo-plugin host",
			Width:   1100,
			Height:  720,
			Content: ui.View(h.view),
		})
		h.start()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
