// Package music adapts MyGo Music's source contract to the generic page
// contract, so a host that knows pages can run a music plugin.
package music

import (
	"fmt"
	"strings"

	plugin "github.com/yxxbc/mygo-plugin"
)

// Stream is a track's audio URL and the headers it needs.
type Stream struct {
	URL     string            `json:"url"`
	Type    string            `json:"type"`
	Headers map[string]string `json:"headers"`
}

// Adapter is one MyGo Music plugin seen through pages.
type Adapter struct{ p *plugin.Plugin }

// New adapts p.
func New(p *plugin.Plugin) *Adapter { return &Adapter{p: p} }

type artist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type album struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ImageURL string `json:"imageUrl"`
}

type track struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Artists     []artist `json:"artists"`
	Album       album    `json:"album"`
	Duration    int      `json:"duration"`
	URL         string   `json:"url"`
	Unavailable bool     `json:"unavailable"`
}

type playlist struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ImageURL    string `json:"imageUrl"`
	Description string `json:"description"`
	Owner       string `json:"owner"`
	TrackCount  int    `json:"trackCount"`
}

type show struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Publisher string `json:"publisher"`
}

type station struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type link struct {
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	Page     string `json:"page"`
	Builtin  string `json:"builtin"`
}

type section struct {
	Title     string     `json:"title"`
	Tracks    []track    `json:"tracks"`
	Playlists []playlist `json:"playlists"`
	Shows     []show     `json:"shows"`
	Links     []link     `json:"links"`
	Stations  []station  `json:"stations"`
}

type page struct {
	Title    string    `json:"title"`
	Subtitle string    `json:"subtitle"`
	Sections []section `json:"sections"`
}

type searchResult struct {
	Tracks []track `json:"tracks"`
}

type playlistDetail struct {
	playlist
	Tracks []track `json:"tracks"`
}

// Home is the plugin's home page.
func (a *Adapter) Home() (*plugin.Page, error) {
	var p page
	if err := a.p.Call("home", nil, &p); err != nil {
		return nil, err
	}
	return pageOf(p), nil
}

// Page is one of the plugin's browse pages.
func (a *Adapter) Page(id string) (*plugin.Page, error) {
	var p page
	if err := a.p.Call("browse", map[string]string{"page": id}, &p); err != nil {
		return nil, err
	}
	return pageOf(p), nil
}

// Liked is the tracks the listener liked.
func (a *Adapter) Liked() (*plugin.Page, error) {
	var r searchResult
	if err := a.p.Call("liked", map[string]int{"offset": 0}, &r); err != nil {
		return nil, err
	}
	return &plugin.Page{
		Title:    "Liked Songs",
		Sections: []plugin.Section{{Items: trackItems(r.Tracks)}},
	}, nil
}

// Stations is the plugin's stations.
func (a *Adapter) Stations() (*plugin.Page, error) {
	var ss []station
	if err := a.p.Call("stations", nil, &ss); err != nil {
		return nil, err
	}
	items := make([]plugin.Item, 0, len(ss))
	for _, st := range ss {
		items = append(items, plugin.Item{
			ID: st.ID, Title: st.Name, Subtitle: st.Description,
			Kind: "station", Meta: map[string]any{"stationID": st.ID},
		})
	}
	return &plugin.Page{Title: "Stations", Sections: []plugin.Section{{Items: items}}}, nil
}

// Search is a track search.
func (a *Adapter) Search(query string) (*plugin.Page, error) {
	var r searchResult
	if err := a.p.Call("search", map[string]any{"query": query, "kind": "tracks", "offset": 0}, &r); err != nil {
		return nil, err
	}
	return &plugin.Page{
		Title:    "Search: " + query,
		Subtitle: fmt.Sprintf("%d tracks", len(r.Tracks)),
		Sections: []plugin.Section{{Items: trackItems(r.Tracks)}},
	}, nil
}

// Playlist is one playlist with its tracks.
func (a *Adapter) Playlist(id string) (*plugin.Page, error) {
	var d playlistDetail
	if err := a.p.Call("playlist", map[string]string{"id": id}, &d); err != nil {
		return nil, err
	}
	return &plugin.Page{
		Title:    d.Name,
		Subtitle: d.Description,
		Sections: []plugin.Section{{Items: trackItems(d.Tracks)}},
	}, nil
}

// Stream resolves a track's audio URL.
func (a *Adapter) Stream(id string) (*Stream, error) {
	var s Stream
	if err := a.p.Call("stream", map[string]string{"id": id, "quality": "high"}, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// StationNext is the next tracks of a station.
func (a *Adapter) StationNext(id string) ([]plugin.Item, error) {
	var tracks []track
	if err := a.p.Call("stationNext", map[string]string{"id": id}, &tracks); err != nil {
		return nil, err
	}
	return trackItems(tracks), nil
}

func pageOf(p page) *plugin.Page {
	out := &plugin.Page{Title: p.Title, Subtitle: p.Subtitle}
	for _, s := range p.Sections {
		sec := plugin.Section{Title: s.Title}
		sec.Items = append(sec.Items, trackItems(s.Tracks)...)
		for _, pl := range s.Playlists {
			sec.Items = append(sec.Items, plugin.Item{
				ID: pl.ID, Title: pl.Name, Subtitle: playlistLine(pl),
				ImageURL: pl.ImageURL, Kind: "playlist",
				Meta: map[string]any{"playlistID": pl.ID},
			})
		}
		for _, l := range s.Links {
			meta := map[string]any{"page": l.Page}
			if l.Builtin != "" {
				meta["builtin"] = l.Builtin
			}
			sec.Items = append(sec.Items, plugin.Item{
				ID: l.Page + l.Builtin, Title: l.Title, Subtitle: l.Subtitle,
				Kind: "link", Meta: meta,
			})
		}
		for _, st := range s.Stations {
			sec.Items = append(sec.Items, plugin.Item{
				ID: st.ID, Title: st.Name, Subtitle: st.Description,
				Kind: "station", Meta: map[string]any{"stationID": st.ID},
			})
		}
		for _, sh := range s.Shows {
			sec.Items = append(sec.Items, plugin.Item{
				ID: sh.ID, Title: sh.Name, Subtitle: sh.Publisher,
				Kind: "show", Meta: map[string]any{"showID": sh.ID},
			})
		}
		if sec.Title != "" || len(sec.Items) > 0 {
			out.Sections = append(out.Sections, sec)
		}
	}
	return out
}

func trackItems(tracks []track) []plugin.Item {
	out := make([]plugin.Item, 0, len(tracks))
	for _, t := range tracks {
		item := plugin.Item{
			ID: t.ID, Title: t.Name, Subtitle: artistNames(t), ImageURL: t.Album.ImageURL,
			Kind: "track",
			Meta: map[string]any{
				"trackID": t.ID, "duration": t.Duration, "artists": artistNames(t),
				"album": t.Album.Name, "albumID": t.Album.ID,
				"imageURL": t.Album.ImageURL, "unavailable": t.Unavailable,
			},
		}
		if t.URL != "" {
			item.Actions = append(item.Actions, plugin.Action{
				ID: "open", Title: "Open in browser", Kind: "url", URL: t.URL,
			})
		}
		out = append(out, item)
	}
	return out
}

func artistNames(t track) string {
	names := make([]string, 0, len(t.Artists))
	for _, a := range t.Artists {
		names = append(names, a.Name)
	}
	return strings.Join(names, ", ")
}

func playlistLine(p playlist) string {
	switch {
	case p.Owner != "" && p.TrackCount > 0:
		return fmt.Sprintf("%s · %d tracks", p.Owner, p.TrackCount)
	case p.TrackCount > 0:
		return fmt.Sprintf("%d tracks", p.TrackCount)
	}
	return p.Description
}
