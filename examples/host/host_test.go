package main

import (
	"testing"

	plugin "github.com/yxxbc/mygo-plugin"
	"github.com/yxxbc/mygo-plugin/adapters/music"
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
	page, err := h.load(h.nav[0])
	if err != nil {
		t.Fatal(err)
	}
	if page.Title != "Home" || page.Subtitle == "" {
		t.Errorf("title=%q subtitle=%q", page.Title, page.Subtitle)
	}
	rows := rowsOf(page)
	if len(rows) != 2 { // the section header and its item
		t.Fatalf("rows = %+v", rows)
	}
	if rows[1].kind != rowItem || rows[1].item.Title != "First item" {
		t.Errorf("row = %+v", rows[1])
	}
}

func TestMusicAdapter(t *testing.T) {
	p, err := plugin.Load("../../testdata/musicadapter", plugin.Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	h := newHost(".", t.TempDir())
	h.cur = p
	h.adapter = music.New(p)
	h.nav = []plugin.Navigation{{Title: "Home", Builtin: "home"}}
	page, err := h.load(h.nav[0])
	if err != nil {
		t.Fatal(err)
	}
	rows := rowsOf(page)
	if page.Title != "Home" || len(rows) != 6 { // header + five items
		t.Fatalf("page = %+v, rows = %+v", page, rows)
	}
	if rows[1].item.Kind != "track" || rows[1].item.Meta["trackID"] != "1" {
		t.Errorf("track row = %+v", rows[1])
	}
	if rows[2].item.Kind != "playlist" {
		t.Errorf("playlist row = %+v", rows[2])
	}
}

func TestWebURL(t *testing.T) {
	for _, s := range []string{"https://example.com/x", "http://example.com"} {
		if !webURL(s) {
			t.Errorf("%s rejected", s)
		}
	}
	for _, s := range []string{"", "file:///etc/passwd", "javascript:alert(1)", "example.com", "ftp://example.com"} {
		if webURL(s) {
			t.Errorf("%s accepted", s)
		}
	}
}
