package main

import (
	"path/filepath"
	"testing"

	plugin "github.com/yxxbc/mygo-plugin"
)

// TestNotesPlugin loads the notes example through the generic path: a page
// from contributes.pages, answered by the plugin's page export.
func TestNotesPlugin(t *testing.T) {
	p, err := plugin.Load(filepath.Join("..", "plugins", "notes"), plugin.Options{
		AppVersion: "0.3.0",
		DataDir:    t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	if m := p.Manifest(); m.ID != "notes" || m.Name != "Notes" {
		t.Fatalf("manifest = %+v", m)
	}
	h := newHost(".", t.TempDir())
	h.cur = p
	h.generic = p.Manifest().Contributes != nil && len(p.Manifest().Contributes.Pages) > 0
	h.nav = p.Manifest().Pages()
	if len(h.nav) != 1 {
		t.Fatalf("nav = %+v", h.nav)
	}

	title, subtitle, rows, err := h.load(h.nav[0])
	if err != nil {
		t.Fatal(err)
	}
	if title != "Notes" {
		t.Errorf("title = %q", title)
	}
	if subtitle != "A generic plugin, not a music source" {
		t.Errorf("subtitle = %q", subtitle)
	}
	if len(rows) != 2 { // the section header and its item
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].kind != rowHeader || rows[0].title != "Today" {
		t.Errorf("header = %+v", rows[0])
	}
	got := rows[1]
	if got.kind != rowItem || got.id != "welcome" || got.title != "Welcome" ||
		got.subtitle != "This page came from a plugin" {
		t.Errorf("row = %+v", got)
	}

	// The command export is only exercised when the API calls it by name;
	// this version has Call, so ask for the hello answer directly.
	if p.Can("command") {
		var out struct {
			OK      bool   `json:"ok"`
			Message string `json:"message"`
		}
		if err := p.Call("command", map[string]any{"id": "hello"}, &out); err != nil {
			t.Fatal(err)
		}
		if !out.OK || out.Message != "Hello from Notes" {
			t.Errorf("command = %+v", out)
		}
	}
}
