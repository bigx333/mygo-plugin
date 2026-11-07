package main

import (
	"testing"

	plugin "github.com/yxxbc/mygo-plugin"
)

func TestGenericPluginPages(t *testing.T) {
	p, err := plugin.Load("../../testdata/generic", plugin.Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	h := newHost(".", t.TempDir())
	h.cur = p
	h.generic = p.Manifest().Contributes != nil
	h.nav = p.Manifest().Pages()
	if len(h.nav) != 1 {
		t.Fatalf("nav = %+v", h.nav)
	}
	title, subtitle, rows, err := h.load(h.nav[0])
	if err != nil {
		t.Fatal(err)
	}
	if title != "Home" || subtitle == "" {
		t.Errorf("title=%q subtitle=%q", title, subtitle)
	}
	if len(rows) != 2 { // the section header and its item
		t.Fatalf("rows = %+v", rows)
	}
	if rows[1].kind != rowItem || rows[1].title != "First item" {
		t.Errorf("row = %+v", rows[1])
	}
}
