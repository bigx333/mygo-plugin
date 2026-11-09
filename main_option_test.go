package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMainOverridesTheManifest(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(`{"id": "two", "name": "Two", "main": "index.js"}`), 0o644)
	os.WriteFile(filepath.Join(dir, "index.js"), []byte(`export function who() { return "window" }`), 0o644)
	os.WriteFile(filepath.Join(dir, "service.ts"), []byte(`export function who(): string { return "service" }`), 0o644)
	for main, want := range map[string]string{"": "window", "service.ts": "service"} {
		p, err := Load(dir, Options{DataDir: t.TempDir(), Main: main})
		if err != nil {
			t.Fatal(err)
		}
		var got string
		if err := p.Call("who", nil, &got); err != nil || got != want {
			t.Errorf("Main %q: %q, %v", main, got, err)
		}
		p.Close()
	}
	for _, bad := range []string{"../escape.js", "/etc/passwd"} {
		if _, err := Load(dir, Options{DataDir: t.TempDir(), Main: bad}); err == nil {
			t.Errorf("Main %q loaded", bad)
		}
	}
}
