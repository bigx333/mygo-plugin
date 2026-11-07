package music

import (
	"errors"
	"testing"

	plugin "github.com/yxxbc/mygo-plugin"
)

func load(t *testing.T) *Adapter {
	t.Helper()
	p, err := plugin.Load("../../testdata/musicadapter", plugin.Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return New(p)
}

func TestHome(t *testing.T) {
	page, err := load(t).Home()
	if err != nil {
		t.Fatal(err)
	}
	if page.Title != "Home" || len(page.Sections) != 1 {
		t.Fatalf("page = %+v", page)
	}
	items := page.Sections[0].Items
	if len(items) != 5 {
		t.Fatalf("items = %+v", items)
	}
	track := items[0]
	if track.Kind != "track" || track.Title != "A Song" || track.Subtitle != "An Artist" {
		t.Errorf("track = %+v", track)
	}
	if track.Meta["trackID"] != "1" || track.Meta["duration"] != 200000 {
		t.Errorf("meta = %v", track.Meta)
	}
	if len(track.Actions) != 1 || track.Actions[0].Kind != "url" {
		t.Errorf("actions = %+v", track.Actions)
	}
	if items[1].Kind != "playlist" || items[1].Meta["playlistID"] != "30" {
		t.Errorf("playlist = %+v", items[1])
	}
	if items[2].Kind != "link" || items[2].Meta["page"] != "daily" {
		t.Errorf("link = %+v", items[2])
	}
	if items[3].Kind != "station" || items[3].Meta["stationID"] != "DEFAULT" {
		t.Errorf("station = %+v", items[3])
	}
	if items[4].Kind != "show" || items[4].Meta["showID"] != "40" {
		t.Errorf("show = %+v", items[4])
	}
}

func TestSearchPlaylistStream(t *testing.T) {
	a := load(t)
	s, err := a.Search("周杰伦")
	if err != nil {
		t.Fatal(err)
	}
	if s.Title != "Search: 周杰伦" || len(s.Sections[0].Items) != 1 {
		t.Errorf("search = %+v", s)
	}
	pl, err := a.Playlist("30")
	if err != nil {
		t.Fatal(err)
	}
	if pl.Title != "A Playlist" || len(pl.Sections[0].Items) != 1 {
		t.Errorf("playlist = %+v", pl)
	}
	st, err := a.Stream("1")
	if err != nil {
		t.Fatal(err)
	}
	if st.URL != "https://example.com/1.mp3" || st.Headers["User-Agent"] != "test" {
		t.Errorf("stream = %+v", st)
	}
	items, err := a.StationNext("DEFAULT")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Kind != "track" {
		t.Errorf("station = %+v", items)
	}
}

func TestLikedAndStations(t *testing.T) {
	a := load(t)
	liked, err := a.Liked()
	if err != nil {
		t.Fatal(err)
	}
	if liked.Title != "Liked Songs" || len(liked.Sections[0].Items) != 1 {
		t.Errorf("liked = %+v", liked)
	}
	stations, err := a.Stations()
	if err != nil {
		t.Fatal(err)
	}
	if len(stations.Sections[0].Items) != 1 || stations.Sections[0].Items[0].Kind != "station" {
		t.Errorf("stations = %+v", stations)
	}
}

func TestUnsupported(t *testing.T) {
	p, err := plugin.Load("../..", plugin.Options{DataDir: t.TempDir()})
	if err == nil {
		p.Close()
		t.Fatal("loaded the module root as a plugin")
	}
	// A plugin without the music exports.
	p, err = plugin.Load("../../testdata/hello", plugin.Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if _, err := New(p).Home(); !errors.Is(err, plugin.ErrUnsupported) {
		t.Errorf("home on hello: %v", err)
	}
}
