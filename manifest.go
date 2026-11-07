// Package plugin hosts third-party plugins in MyGo apps.
//
// A plugin is a folder with a plugin.json and a JavaScript or TypeScript
// module. The host reads it, compiles it once, and runs it in a moejs runtime
// of its own, so a plugin never shares a runtime, a heap or an event loop with
// another. The app calls exports by name and gets JSON values back; the plugin
// reaches the app only through the host global, whose permissions the manifest
// declares.
//
// The manifest is the one MyGo Music uses, so its plugins load here unchanged.
package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// DefaultMain is the module a manifest names when it doesn't.
const DefaultMain = "index.js"

// Manifest is a plugin's plugin.json.
type Manifest struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Version     string `json:"version,omitempty"`
	Author      string `json:"author,omitempty"`
	Homepage    string `json:"homepage,omitempty"`
	// Main is the module to run, relative to the plugin's folder.
	Main string `json:"main,omitempty"`
	// MinAppVersion is the oldest host version that can run the plugin.
	MinAppVersion string `json:"minAppVersion,omitempty"`
	// Playback tells the host how the plugin plays: "stream" when it answers
	// a URL the host fetches and plays itself, as MyGo Music's sources do.
	Playback    string      `json:"playback,omitempty"`
	Permissions Permissions `json:"permissions"`
	Settings    []Setting   `json:"settings,omitempty"`
	// Navigation is MyGo Music's sidebar shape; Contributes is the generic
	// one. A plugin may use either, and a host reads Pages.
	Navigation  []Navigation   `json:"navigation,omitempty"`
	Contributes *Contributions `json:"contributes,omitempty"`
}

// Contributions are what a plugin adds to a host that doesn't know its kind.
type Contributions struct {
	Pages []PageContribution `json:"pages,omitempty"`
}

// PageContribution is one page a plugin draws through the host.
type PageContribution struct {
	ID    string `json:"id"`
	Title string `json:"title,omitempty"`
	Icon  string `json:"icon,omitempty"`
}

// Pages is what the host's sidebar shows: the generic contributions when a
// plugin has them, and MyGo Music's navigation otherwise.
func (m Manifest) Pages() []Navigation {
	if m.Contributes == nil || len(m.Contributes.Pages) == 0 {
		return m.Navigation
	}
	out := make([]Navigation, 0, len(m.Contributes.Pages))
	for _, p := range m.Contributes.Pages {
		title := p.Title
		if title == "" {
			title = p.ID
		}
		out = append(out, Navigation{Title: title, Page: p.ID, Icon: p.Icon})
	}
	return out
}

// Permissions are what a plugin asks for beyond its own folder and data.
type Permissions struct {
	// Network lists the hosts fetch may reach: example.com, *.example.com
	// for its subdomains, or * for any. Empty reaches nothing.
	Network []string `json:"network,omitempty"`
	// Process lists the programs the plugin may start, once the listener
	// grants it. This version declares them but doesn't start any: a host
	// that does reads them from here.
	Process Programs `json:"process,omitempty"`
}

// Programs is a permission's list of programs: names on PATH, paths from the
// plugin's folder, patterns as filepath.Match has them, or true for any.
type Programs []string

// UnmarshalJSON accepts a list of programs, or true for any.
func (p *Programs) UnmarshalJSON(b []byte) error {
	var all bool
	if err := json.Unmarshal(b, &all); err == nil {
		*p = nil
		if all {
			*p = Programs{"*"}
		}
		return nil
	}
	var names []string
	if err := json.Unmarshal(b, &names); err != nil {
		return errors.New("process is a list of programs, or true for any")
	}
	*p = names
	return nil
}

// Any reports whether the plugin may start any program.
func (p Programs) Any() bool {
	for _, name := range p {
		if name == "*" {
			return true
		}
	}
	return false
}

// Setting is a value the listener fills in, which the plugin reads with
// host.setting.
type Setting struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Help  string `json:"help,omitempty"`
	Link  string `json:"link,omitempty"`
}

// Navigation is an entry in the host's sidebar.
type Navigation struct {
	Title string `json:"title"`
	// Page is the name the host asks the plugin's browse export for.
	Page string `json:"page,omitempty"`
	// Builtin names a page the host draws itself.
	Builtin string `json:"builtin,omitempty"`
	// Icon names one of the host's icons, not a file.
	Icon string `json:"icon,omitempty"`
}

var (
	idPattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	hostPattern = regexp.MustCompile(`^(\*|(\*\.)?[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?(\.[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?)*)$`)
)

// ReadManifest reads the plugin.json in dir, fills in what it leaves out, and
// checks it.
func ReadManifest(dir string) (*Manifest, error) {
	path := filepath.Join(dir, "plugin.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	m, err := ParseManifest(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return m, nil
}

// ParseManifest reads a plugin.json, fills in what it leaves out, and checks
// it.
func ParseManifest(b []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	m.normalize()
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

func (m *Manifest) normalize() {
	if m.ID != "" {
		m.ID = strings.ToLower(m.ID)
	}
	if m.Name == "" {
		m.Name = m.ID
	}
	if m.Version == "" {
		m.Version = "0.0.0"
	}
	if m.Main == "" {
		m.Main = DefaultMain
	}
	if m.Playback == "" {
		m.Playback = "stream"
	}
}

// Validate says what's wrong with the manifest, or nil.
func (m *Manifest) Validate() error {
	if m.ID == "" {
		return errors.New("plugin.json needs an id")
	}
	if !idPattern.MatchString(m.ID) {
		return fmt.Errorf("id %q is letters, digits, dots, underscores and hyphens, up to 64, starting with a letter or digit", m.ID)
	}
	if m.ID == "mygo" {
		return errors.New(`the id "mygo" is reserved`)
	}
	if m.MinAppVersion != "" {
		if _, ok := parseVersion(m.MinAppVersion); !ok {
			return fmt.Errorf("minAppVersion is not a version such as 0.1.8: %q", m.MinAppVersion)
		}
	}
	if m.Main == "" || !filepath.IsLocal(m.Main) || strings.ContainsAny(m.Main, `\:`) {
		return fmt.Errorf("main is a file inside the plugin's folder: %q", m.Main)
	}
	for _, h := range m.Permissions.Network {
		if !hostPattern.MatchString(h) {
			return fmt.Errorf("network permission %q is a host, *.host or *", h)
		}
	}
	for i, n := range m.Navigation {
		if n.Page == "" && n.Builtin == "" {
			return fmt.Errorf("navigation[%d] %q needs a page or a builtin", i, n.Title)
		}
		// The host names its own pages, so a builtin needs no title; a page
		// of the plugin's does.
		if n.Title == "" && n.Page != "" {
			return fmt.Errorf("navigation[%d] needs a title", i)
		}
	}
	for i, s := range m.Settings {
		if s.Key == "" {
			return fmt.Errorf("settings[%d] needs a key", i)
		}
	}
	if m.Contributes != nil {
		for i, p := range m.Contributes.Pages {
			if p.ID == "" || !idPattern.MatchString(p.ID) {
				return fmt.Errorf("contributes.pages[%d] needs an id of letters, digits, dots, underscores and hyphens", i)
			}
		}
	}
	return nil
}

// NeedsNewerApp is why a plugin doesn't load: it needs a newer host.
type NeedsNewerApp struct {
	Plugin  string // its name
	Version string // the oldest host version it works with
}

func (e *NeedsNewerApp) Error() string {
	return fmt.Sprintf("%s needs this app %s or later", e.Plugin, e.Version)
}

// parseVersion reads 1 to 4 numbers, as 0.1.8 has them, ignoring a leading v
// and a pre-release or build suffix.
func parseVersion(s string) ([4]int, bool) {
	var v [4]int
	s = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "v"))
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	if s == "" {
		return v, false
	}
	parts := strings.Split(s, ".")
	if len(parts) > len(v) {
		return v, false
	}
	for i, p := range parts {
		if p == "" {
			return v, false
		}
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return v, false
		}
		v[i] = n
	}
	return v, true
}

// CompareVersions compares two versions as 0.1.8: -1 when a is older, 0 when
// they are the same, and 1 when a is newer.
func CompareVersions(a, b string) (int, error) {
	va, ok := parseVersion(a)
	if !ok {
		return 0, fmt.Errorf("not a version: %q", a)
	}
	vb, ok := parseVersion(b)
	if !ok {
		return 0, fmt.Errorf("not a version: %q", b)
	}
	for i := range va {
		if va[i] < vb[i] {
			return -1, nil
		}
		if va[i] > vb[i] {
			return 1, nil
		}
	}
	return 0, nil
}
