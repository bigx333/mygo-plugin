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
	"github.com/egoist/mygo/ui/markdown"
	plugin "github.com/yxxbc/mygo-plugin"
	"github.com/yxxbc/mygo-plugin/adapters/music"
)

type rowKind int

const (
	rowHeader rowKind = iota
	rowItem
)

type row struct {
	kind rowKind
	item plugin.Item
}

func (r row) key() any {
	if r.item.ID != "" {
		return r.item.ID
	}
	return r.item.Title
}

type host struct {
	win        *mygo.Window
	pluginsDir string
	dataDir    string

	plugins []*plugin.Plugin
	cur     *plugin.Plugin
	generic bool
	adapter *music.Adapter
	nav     []plugin.Navigation
	navAt   int
	gen     int

	query    string
	title    string
	subtitle string
	page     *plugin.Page
	rows     []row
	busy     bool
	err      string
	notice   string
	signIn   string

	md markdown.Options

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
		AppVersion: "0.6.0",
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
	h.generic = h.cur.Manifest().Contributes != nil && len(h.cur.Manifest().Contributes.Pages) > 0
	h.adapter = nil
	if !h.generic && (h.cur.Can("stream") || h.cur.Can("browse")) {
		h.adapter = music.New(h.cur)
	}
	h.nav = h.cur.Manifest().Pages()
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
	h.page = nil
	h.rows = nil
	h.busy = true
	if h.needsConsent() {
		h.busy = false
		return
	}
	gen := h.gen
	n := h.nav[i]
	go func() {
		page, err := h.load(n)
		h.win.Update(func() {
			if gen != h.gen {
				return
			}
			h.busy = false
			if err != nil {
				h.err = err.Error()
				return
			}
			h.show(page)
		})
	}()
}

func (h *host) load(n plugin.Navigation) (*plugin.Page, error) {
	if h.generic {
		return h.cur.Page(n.Page)
	}
	if h.adapter == nil {
		return nil, fmt.Errorf("this plugin is neither a generic page plugin nor a music source")
	}
	if n.Page != "" {
		return h.adapter.Page(n.Page)
	}
	switch n.Builtin {
	case "home":
		return h.adapter.Home()
	case "liked":
		return h.adapter.Liked()
	case "stations":
		return h.adapter.Stations()
	}
	return nil, fmt.Errorf("this host doesn't have the %q page", n.Builtin)
}

func (h *host) show(page *plugin.Page) {
	h.page = page
	h.title = page.Title
	if h.title == "" {
		h.title = navTitle(h.nav[h.navAt])
	}
	h.subtitle = page.Subtitle
	h.rows = rowsOf(page)
}

func (h *host) search() {
	q := strings.TrimSpace(h.query)
	if h.adapter == nil || q == "" || h.cur == nil || h.needsConsent() {
		return
	}
	h.gen++
	gen := h.gen
	h.busy = true
	h.err = ""
	h.page = nil
	h.rows = nil
	go func() {
		page, err := h.adapter.Search(q)
		h.win.Update(func() {
			if gen != h.gen {
				return
			}
			h.busy = false
			if err != nil {
				h.err = err.Error()
				return
			}
			h.show(page)
		})
	}()
}

func (h *host) loadPlaylist(id string) {
	if h.adapter == nil {
		return
	}
	h.gen++
	gen := h.gen
	h.busy = true
	h.err = ""
	h.page = nil
	h.rows = nil
	go func() {
		page, err := h.adapter.Playlist(id)
		h.win.Update(func() {
			if gen != h.gen {
				return
			}
			h.busy = false
			if err != nil {
				h.err = err.Error()
				return
			}
			h.show(page)
		})
	}()
}

func (h *host) activate(i int) {
	if i < 0 || i >= len(h.rows) {
		return
	}
	item := h.rows[i].item
	switch item.Kind {
	case "track":
		h.play(item)
	case "playlist":
		h.loadPlaylist(metaString(item.Meta, "playlistID"))
	case "link":
		h.openPage(metaString(item.Meta, "page"), item.Title)
	case "station":
		h.openStation(metaString(item.Meta, "stationID"))
	default:
		if page := metaString(item.Meta, "page"); page != "" {
			h.openPage(page, item.Title)
			return
		}
		if len(item.Actions) > 0 {
			h.runAction(item.Actions[0])
			return
		}
		h.notice = item.Title + " — " + item.Subtitle
		h.win.Invalidate()
	}
}

func (h *host) openPage(id, title string) {
	if id == "" {
		return
	}
	for j, n := range h.nav {
		if n.Page == id {
			h.openNav(j)
			return
		}
	}
	h.nav = append(h.nav, plugin.Navigation{Title: title, Page: id})
	h.openNav(len(h.nav) - 1)
}

func (h *host) openStation(id string) {
	if h.adapter == nil {
		return
	}
	h.busy = true
	h.err = ""
	go func() {
		items, err := h.adapter.StationNext(id)
		h.win.Update(func() {
			h.busy = false
			if err != nil {
				h.err = err.Error()
				return
			}
			h.show(&plugin.Page{Title: "Station", Sections: []plugin.Section{{Items: items}}})
			for _, it := range items {
				if it.Kind == "track" {
					h.play(it)
					return
				}
			}
		})
	}()
}

func (h *host) runAction(a plugin.Action) {
	switch a.Kind {
	case "url":
		if a.URL != "" {
			if err := mygo.Shell.OpenExternal(a.URL); err != nil {
				h.notice = err.Error()
			}
		}
	case "command":
		h.runCommand(metaString(a.Meta, "command"), a.Meta)
	default:
		h.notice = "This host doesn't know the action " + a.Title
	}
	h.win.Invalidate()
}

func (h *host) runCommand(id string, args map[string]any) {
	if id == "" || h.cur == nil {
		return
	}
	h.notice = "Running " + id + "…"
	h.win.Invalidate()
	go func() {
		var out any
		err := h.cur.Command(id, args, &out)
		h.win.Update(func() {
			if err != nil {
				h.notice = err.Error()
				return
			}
			if out == nil {
				h.notice = id + " ran"
				return
			}
			b, _ := json.Marshal(out)
			h.notice = string(b)
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

func rowsOf(page *plugin.Page) []row {
	var out []row
	for _, s := range page.Sections {
		if s.Title != "" {
			out = append(out, row{kind: rowHeader, item: plugin.Item{Title: s.Title}})
		}
		for _, item := range s.Items {
			out = append(out, row{kind: rowItem, item: item})
		}
	}
	return out
}

func metaString(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

func metaInt(m map[string]any, key string) int {
	switch v := m[key].(type) {
	case int:
		return v
	case float64:
		return int(v)
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	}
	return 0
}

func clock(ms int) string {
	if ms <= 0 {
		return ""
	}
	s := ms / 1000
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}
