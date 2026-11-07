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
