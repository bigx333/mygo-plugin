package plugin

// Page is one page a plugin contributes, as data for the host to draw. It is
// not a view: the host owns layout, chrome, icons and actions.
type Page struct {
	Title    string    `json:"title"`
	Subtitle string    `json:"subtitle,omitempty"`
	Sections []Section `json:"sections"`
}

// Section is a titled group of items on a Page.
type Section struct {
	Title string `json:"title,omitempty"`
	Items []Item `json:"items"`
}

// Item is one row in a Section. Kind and Meta are hints the host defines; this
// layer only carries them.
type Item struct {
	// ID identifies the item within its page, and is what the host passes
	// back when the listener acts on it.
	ID       string `json:"id"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle,omitempty"`
	ImageURL string `json:"imageURL,omitempty"`
	// Kind names the kind of item for this host, such as track, playlist or
	// show. A host that doesn't know it draws a plain row.
	Kind string `json:"kind,omitempty"`
	// Meta is JSON the host may interpret; the plugin host doesn't.
	Meta map[string]any `json:"meta,omitempty"`
}

// Browse asks a plugin for the page named by a navigation entry's page, and
// decodes it as a Page. A plugin that doesn't export browse is ErrUnsupported.
//
// A plugin may instead answer MyGo Music's richer shape, whose sections hold
// tracks, playlists and shows. That shape is out of scope here: the host
// adapts it, and this method does not read its fields.
func (p *Plugin) Browse(page string) (*Page, error) {
	if !p.Can("browse") {
		return nil, ErrUnsupported
	}
	var out Page
	if err := p.Call("browse", map[string]string{"page": page}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
