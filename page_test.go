package plugin

import (
	"errors"
	"testing"
)

func TestGenericPage(t *testing.T) {
	p, err := Load("testdata/generic", Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	pages := p.Manifest().Pages()
	if len(pages) != 1 || pages[0].Page != "home" || pages[0].Title != "Home" {
		t.Fatalf("pages = %+v", pages)
	}
	page, err := p.Page("home")
	if err != nil {
		t.Fatal(err)
	}
	if page.Title != "Home" || len(page.Sections) != 1 || len(page.Sections[0].Items) != 1 {
		t.Fatalf("page = %+v", page)
	}
	if page.Sections[0].Items[0].Kind != "note" {
		t.Errorf("item = %+v", page.Sections[0].Items[0])
	}
}

func TestPageV1(t *testing.T) {
	p, err := Load("testdata/pagev1", Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if pages := p.Manifest().Pages(); len(pages) != 1 || pages[0].Page != "detail" || pages[0].Title != "Detail" {
		t.Fatalf("pages = %+v", pages)
	}
	page, err := p.Page("detail")
	if err != nil {
		t.Fatal(err)
	}
	const markdown = "Some **detail** from the plugin.\n\nIt has two paragraphs."
	if page.Markdown != markdown {
		t.Errorf("markdown = %q", page.Markdown)
	}
	if len(page.Sections) != 1 || len(page.Sections[0].Items) != 1 {
		t.Fatalf("page = %+v", page)
	}
	item := page.Sections[0].Items[0]
	if len(item.Actions) != 2 {
		t.Fatalf("actions = %+v", item.Actions)
	}
	command, link := item.Actions[0], item.Actions[1]
	if command.ID != "play" || command.Title != "Play" || command.Kind != "command" {
		t.Errorf("command action = %+v", command)
	}
	if command.Meta["command"] != "play" {
		t.Errorf("command meta = %+v", command.Meta)
	}
	if link.ID != "open" || link.Title != "Open" || link.Kind != "url" || link.URL != "https://example.com/one" {
		t.Errorf("url action = %+v", link)
	}
}

func TestPageFallsBackToBrowse(t *testing.T) {
	p, err := Load("testdata/hello", Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if _, err := p.Page("demo"); !errors.Is(err, ErrUnsupported) {
		t.Errorf("hello has no browse or page: %v", err)
	}
}
