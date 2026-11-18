package plugin

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MoeForge/moejs"
)

// An export that answers and leaves work running is still answered once
// when the runtime closes under it: a second answer would block a caller
// that already left, and the loop with it, so Close never returned.
func TestACallIsAnsweredOnceWhenClosedAfterItsAnswer(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(`{"id": "busy", "name": "Busy", "main": "index.js"}`), 0o644)
	os.WriteFile(filepath.Join(dir, "index.js"), []byte(`export function answer() { (async () => { for (;;) await 0 })(); return 1 }`), 0o644)
	p, err := Load(dir, Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	v, err := p.running()
	if err != nil {
		t.Fatal(err)
	}
	var answers atomic.Int32
	first := make(chan struct{}, 1)
	v.loop.RunOnLoop(func(*moejs.Runtime) {
		v.ask("answer", nil, func(answer) {
			answers.Add(1)
			select {
			case first <- struct{}{}:
			default:
			}
		})
	})
	select {
	case <-first:
	case <-time.After(5 * time.Second):
		t.Fatal("the export never answered")
	}
	closed := make(chan struct{})
	go func() { v.close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("the runtime never closed")
	}
	if n := answers.Load(); n != 1 {
		t.Fatalf("answered %d times, want once", n)
	}
}
