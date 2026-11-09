package plugin

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A call waiting on its answer as the runtime closes fails at once, not
// at its timeout.
func TestCloseFailsACallInFlight(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(`{"id": "slow", "name": "Slow", "main": "index.js"}`), 0o644)
	os.WriteFile(filepath.Join(dir, "index.js"), []byte(`export function wait() { return new Promise(() => {}) }`), 0o644)
	p, err := Load(dir, Options{DataDir: t.TempDir(), CallTimeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- p.Call("wait", nil, nil) }()
	time.Sleep(200 * time.Millisecond)
	start := time.Now()
	p.Close()
	select {
	case err := <-done:
		if err == nil || time.Since(start) > 2*time.Second {
			t.Fatalf("%v after %v", err, time.Since(start))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the call waited on past Close")
	}
}
