package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	plugin "github.com/yxxbc/mygo-plugin"
)

// dev rechecks a plugin whenever its files change, so a manifest or a module
// that doesn't compile is reported while it is being written. It doesn't run
// an app: an app that hosts the plugin reloads it by loading it again.
func dev(args []string) {
	fs := flag.NewFlagSet("dev", flag.ExitOnError)
	app := fs.String("app-version", "", "the host version minAppVersion is checked against")
	every := fs.Duration("interval", 500*time.Millisecond, "how often to look for changes")
	pos := parseArgs(fs, args)
	dir := "."
	if len(pos) > 0 {
		dir = pos[0]
	}
	check := func() {
		at := time.Now().Format("15:04:05")
		m, exports, err := plugin.Inspect(dir, *app)
		if err != nil {
			fmt.Printf("[%s] error: %v\n", at, err)
			return
		}
		fmt.Printf("[%s] %s %s: %s\n", at, m.Name, m.Version, strings.Join(exports, ", "))
	}
	check()
	last := latestChange(dir)
	for {
		time.Sleep(*every)
		if change := latestChange(dir); change.After(last) {
			last = change
			check()
		}
	}
}

// latestChange is the modification time of the newest file in dir.
func latestChange(dir string) time.Time {
	var newest time.Time
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "dist", "build":
				if path != dir {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if info, err := d.Info(); err == nil && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
		return nil
	})
	return newest
}
