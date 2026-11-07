// Package plugintest loads a plugin the way an app does, for the plugin
// author's own tests. New hands back a loaded plugin and closes it when the
// test ends; the test brings the host, a fake one or a real one, in
// Options.Host.
package plugintest

import (
	"testing"
	"time"

	plugin "github.com/yxxbc/mygo-plugin"
)

// New loads the plugin in dir for a test, filling in what a test usually
// doesn't care about and leaving the rest of opts as given:
//
//   - an empty DataDir becomes t.TempDir(), so no test sees another's storage;
//   - a zero CallTimeout becomes ten seconds;
//   - a nil Host.Logf becomes t.Logf, so a plugin's console output shows in the
//     test's output.
//
// The plugin is closed when the test ends. One that doesn't load fails the
// test.
func New(t *testing.T, dir string, opts plugin.Options) *plugin.Plugin {
	t.Helper()
	if opts.DataDir == "" {
		opts.DataDir = t.TempDir()
	}
	if opts.CallTimeout == 0 {
		opts.CallTimeout = 10 * time.Second
	}
	if opts.Host.Logf == nil {
		opts.Host.Logf = t.Logf
	}
	p, err := plugin.Load(dir, opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}
