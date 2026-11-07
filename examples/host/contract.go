package main

// The MyGo Music source contract, as this host reads it. It is app-specific:
// the plugin format is shared, the shape of a track is not.

type Artist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Album struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ImageURL string `json:"imageUrl"`
}

type Track struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Artists     []Artist `json:"artists"`
	Album       Album    `json:"album"`
	Duration    int      `json:"duration"` // milliseconds
	Unavailable bool     `json:"unavailable"`
}

type Playlist struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ImageURL    string `json:"imageUrl"`
	Description string `json:"description"`
	Owner       string `json:"owner"`
	TrackCount  int    `json:"trackCount"`
}

type Show struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ImageURL    string `json:"imageUrl"`
	Description string `json:"description"`
	Publisher   string `json:"publisher"`
}

type Station struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Link struct {
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	Page     string `json:"page"`
	Builtin  string `json:"builtin"`
}

type Section struct {
	Title     string     `json:"title"`
	Tracks    []Track    `json:"tracks"`
	Playlists []Playlist `json:"playlists"`
	Shows     []Show     `json:"shows"`
	Links     []Link     `json:"links"`
	Stations  []Station  `json:"stations"`
}

type Page struct {
	Title    string    `json:"title"`
	Subtitle string    `json:"subtitle"`
	Sections []Section `json:"sections"`
}

type SearchResult struct {
	Tracks []Track `json:"tracks"`
	More   bool    `json:"more"`
}

type PlaylistDetail struct {
	Playlist
	Tracks []Track `json:"tracks"`
}

type Stream struct {
	URL     string            `json:"url"`
	Type    string            `json:"type"`
	Headers map[string]string `json:"headers"`
}
