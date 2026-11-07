package plugin

import (
	"errors"
	"testing"
)

func TestBrowse(t *testing.T) {
	p, err := Load("testdata/pages", Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	page, err := p.Browse("demo")
	if err != nil {
		t.Fatal(err)
	}
	if page.Title != "Demo" {
		t.Errorf("title = %q", page.Title)
	}
	if page.Subtitle != "A page" {
		t.Errorf("subtitle = %q", page.Subtitle)
	}
	if len(page.Sections) != 1 || page.Sections[0].Title != "Tracks" {
		t.Fatalf("sections = %+v", page.Sections)
	}
	items := page.Sections[0].Items
	if len(items) != 1 {
		t.Fatalf("items = %+v", items)
	}
	if items[0].ID != "1" || items[0].Title != "One" || items[0].Kind != "track" {
		t.Errorf("item = %+v", items[0])
	}
}

func TestBrowseUnsupported(t *testing.T) {
	p, err := Load("testdata/hello", Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	if _, err := p.Browse("demo"); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("err = %v, want ErrUnsupported", err)
	}
}
