package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/audio"
	"github.com/egoist/mygo/ui"
	plugin "github.com/yxxbc/mygo-plugin"
)

type rowKind int

const (
	rowHeader rowKind = iota
	rowTrack
	rowPlaylist
	rowLink
	rowStation
	rowShow
)

type row struct {
	kind     rowKind
	id       string
	title    string
	subtitle string
	page     string
	track    Track
}

func (r row) key() any {
	if r.id != "" {
		return r.id
	}
	return r.title
}

type host struct {
	win        *mygo.Window
	pluginsDir string
	dataDir    string

	plugins []*plugin.Plugin
	cur     *plugin.Plugin
	nav     []plugin.Navigation
	navAt   int
	gen     int

	query    string
	title    string
	subtitle string
	rows     []row
	busy     bool
	err      string
	notice   string
	signIn   string

	player  *audio.Player
	now     string
	playing bool
	tmp     string

	consentMu sync.RWMutex
	consent   map[string]bool

	list ui.ListState
}

func newHost(pluginsDir, dataDir string) *host {
	h := &host{pluginsDir: pluginsDir, dataDir: dataDir, consent: map[string]bool{}}
	h.loadConsent()
	return h
}

func (h *host) start() {
	h.plugins = plugin.Discover(h.pluginsDir, plugin.Options{
		AppVersion: "0.3.0",
		DataDir:    h.dataDir,
		Host: plugin.Host{
			Language: func() string { return "en" },
			OpenURL:  func(u string) error { return mygo.Shell.OpenExternal(u) },
			SignInPrompt: func(_ string, p plugin.SignInPrompt) {
				h.win.Update(func() {
					h.signIn = strings.TrimSpace(p.Text)
					if p.QR != "" {
						h.signIn = strings.TrimSpace(h.signIn + "\n" + p.QR)
					}
				})
			},
			Logf: log.Printf,
		},
		Allowed: h.allowed,
	})
	if len(h.plugins) == 0 {
		h.err = fmt.Sprintf("No plugins in %s.\nInstall one with: mygo-plugin install <id> %s", abs(h.pluginsDir), h.pluginsDir)
		return
	}
	h.selectPlugin(0)
}

func abs(p string) string {
	if a, err := filepath.Abs(p); err == nil {
		return a
	}
	return p
}

func (h *host) selectPlugin(i int) {
	h.cur = h.plugins[i]
	h.nav = h.cur.Manifest().Navigation
	if len(h.nav) == 0 {
		h.nav = []plugin.Navigation{{Title: "Home", Builtin: "home"}}
	}
	h.win.SetTitle("mygo-plugin host — " + h.cur.Manifest().Name)
	h.openNav(0)
}

func (h *host) openNav(i int) {
	h.navAt = i
	h.gen++
	h.err = ""
	h.notice = ""
	h.title = navTitle(h.nav[i])
	h.subtitle = ""
	h.rows = nil
	h.busy = true
	if h.needsConsent() {
		h.busy = false
		return
	}
	gen := h.gen
	n := h.nav[i]
	go func() {
		title, subtitle, rows, err := h.load(n)
		h.win.Update(func() {
			if gen != h.gen {
				return
			}
			h.busy = false
			h.err = ""
			if err != nil {
				h.err = err.Error()
				return
			}
			h.title, h.subtitle, h.rows = title, subtitle, rows
		})
	}()
}

func (h *host) load(n plugin.Navigation) (string, string, []row, error) {
	if n.Page != "" {
		var page Page
		if err := h.cur.Call("browse", map[string]string{"page": n.Page}, &page); err != nil {
			return "", "", nil, err
		}
		return page.Title, page.Subtitle, rowsOf(page), nil
	}
	switch n.Builtin {
	case "home":
		var page Page
		if err := h.cur.Call("home", nil, &page); err != nil {
			return "", "", nil, err
		}
		return page.Title, page.Subtitle, rowsOf(page), nil
	case "liked":
		var liked struct {
			Tracks []Track `json:"tracks"`
		}
		if err := h.cur.Call("liked", map[string]int{"offset": 0}, &liked); err != nil {
			return "", "", nil, err
		}
		return "Liked Songs", "", trackRows(liked.Tracks), nil
	case "stations":
		var stations []Station
		if err := h.cur.Call("stations", nil, &stations); err != nil {
			return "", "", nil, err
		}
		rows := make([]row, 0, len(stations))
		for _, s := range stations {
			rows = append(rows, row{kind: rowStation, id: s.ID, title: s.Name, subtitle: s.Description})
		}
		return "Stations", "", rows, nil
	}
	return "", "", nil, fmt.Errorf("this host doesn't have the %q page", n.Builtin)
}

func (h *host) search() {
	q := strings.TrimSpace(h.query)
	if q == "" || h.cur == nil || h.needsConsent() {
		return
	}
	h.gen++
	gen := h.gen
	h.busy = true
	h.err = ""
	h.rows = nil
	go func() {
		var res SearchResult
		err := h.cur.Call("search", map[string]any{"query": q, "kind": "tracks", "offset": 0}, &res)
		h.win.Update(func() {
			if gen != h.gen {
				return
			}
			h.busy = false
			if err != nil {
				h.err = err.Error()
				return
			}
			h.title = "Search: " + q
			h.subtitle = fmt.Sprintf("%d tracks", len(res.Tracks))
			h.rows = trackRows(res.Tracks)
		})
	}()
}

func (h *host) loadPlaylist(id string) {
	h.gen++
	gen := h.gen
	h.busy = true
	h.err = ""
	h.rows = nil
	go func() {
		var p PlaylistDetail
		err := h.cur.Call("playlist", map[string]string{"id": id}, &p)
		h.win.Update(func() {
			if gen != h.gen {
				return
			}
			h.busy = false
			if err != nil {
				h.err = err.Error()
				return
			}
			h.title = p.Name
			h.subtitle = p.Description
			h.rows = trackRows(p.Tracks)
		})
	}()
}

func (h *host) activate(i int) {
	if i < 0 || i >= len(h.rows) {
		return
	}
	r := h.rows[i]
	switch r.kind {
	case rowTrack:
		h.play(r.track)
	case rowPlaylist:
		h.loadPlaylist(r.id)
	case rowLink:
		if r.page != "" {
			for j, n := range h.nav {
				if n.Page == r.page {
					h.openNav(j)
					return
				}
			}
			h.nav = append(h.nav, plugin.Navigation{Title: r.title, Page: r.page})
			h.openNav(len(h.nav) - 1)
		}
	case rowStation:
		h.openStation(r.id)
	}
}

func (h *host) openStation(id string) {
	h.busy = true
	h.err = ""
	go func() {
		var tracks []Track
		err := h.cur.Call("stationNext", map[string]string{"id": id}, &tracks)
		h.win.Update(func() {
			h.busy = false
			if err != nil {
				h.err = err.Error()
				return
			}
			if len(tracks) > 0 {
				h.title = "Station"
				h.subtitle = tracks[0].Name
				h.rows = trackRows(tracks)
				h.play(tracks[0])
			}
		})
	}()
}

func (h *host) allowed(pluginID, host string) bool {
	h.consentMu.RLock()
	defer h.consentMu.RUnlock()
	return h.consent[pluginID]
}

func (h *host) needsConsent() bool {
	if h.cur == nil {
		return false
	}
	network := h.cur.Manifest().Permissions.Network
	if len(network) == 0 {
		return false
	}
	h.consentMu.RLock()
	defer h.consentMu.RUnlock()
	return !h.consent[h.cur.ID()]
}

func (h *host) allow() {
	if h.cur == nil {
		return
	}
	h.consentMu.Lock()
	h.consent[h.cur.ID()] = true
	h.consentMu.Unlock()
	h.saveConsent()
	h.cur.Close()
	h.openNav(h.navAt)
	h.win.Invalidate()
}

func (h *host) loadConsent() {
	b, err := os.ReadFile(filepath.Join(h.dataDir, "consent.json"))
	if err == nil {
		json.Unmarshal(b, &h.consent)
	}
}

func (h *host) saveConsent() error {
	if err := os.MkdirAll(h.dataDir, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(h.consent, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(h.dataDir, "consent.json"), b, 0o600)
}

func navTitle(n plugin.Navigation) string {
	if n.Title != "" {
		return n.Title
	}
	if n.Builtin != "" {
		return strings.ToUpper(n.Builtin[:1]) + n.Builtin[1:]
	}
	return n.Page
}

func rowsOf(page Page) []row {
	var out []row
	for _, s := range page.Sections {
		if s.Title != "" {
			out = append(out, row{kind: rowHeader, title: s.Title})
		}
		for _, t := range s.Tracks {
			out = append(out, row{kind: rowTrack, id: t.ID, title: t.Name, subtitle: artists(t), track: t})
		}
		for _, p := range s.Playlists {
			out = append(out, row{kind: rowPlaylist, id: p.ID, title: p.Name, subtitle: playlistLine(p)})
		}
		for _, l := range s.Links {
			out = append(out, row{kind: rowLink, title: l.Title, subtitle: l.Subtitle, page: l.Page})
		}
		for _, st := range s.Stations {
			out = append(out, row{kind: rowStation, id: st.ID, title: st.Name, subtitle: st.Description})
		}
		for _, sh := range s.Shows {
			out = append(out, row{kind: rowShow, id: sh.ID, title: sh.Name, subtitle: sh.Publisher})
		}
	}
	return out
}

func trackRows(tracks []Track) []row {
	out := make([]row, 0, len(tracks))
	for _, t := range tracks {
		out = append(out, row{kind: rowTrack, id: t.ID, title: t.Name, subtitle: artists(t), track: t})
	}
	return out
}

func artists(t Track) string {
	names := make([]string, 0, len(t.Artists))
	for _, a := range t.Artists {
		names = append(names, a.Name)
	}
	return strings.Join(names, ", ")
}

func playlistLine(p Playlist) string {
	switch {
	case p.Owner != "" && p.TrackCount > 0:
		return fmt.Sprintf("%s · %d tracks", p.Owner, p.TrackCount)
	case p.TrackCount > 0:
		return fmt.Sprintf("%d tracks", p.TrackCount)
	}
	return p.Description
}

func clock(ms int) string {
	if ms <= 0 {
		return ""
	}
	s := ms / 1000
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}
