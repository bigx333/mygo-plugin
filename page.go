package plugin

// Page is one page a plugin contributes, as data for the host to draw. It is
// not a view: the host owns layout, chrome, icons and actions.
type Page struct {
	Title    string `json:"title"`
	Subtitle string `json:"subtitle,omitempty"`
	// Markdown is a detail page's body, in CommonMark, which the host renders
	// above the sections. A host without Markdown support may ignore it.
	Markdown string    `json:"markdown,omitempty"`
	Sections []Section `json:"sections"`
}

// Section is a titled group of items on a Page.
type Section struct {
	Title string `json:"title,omitempty"`
	Items []Item `json:"items"`
}

// Item is one row in a Section. Kind, Meta and Actions are hints the host
// defines; this layer only carries them.
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
	// Actions are the choices the host offers for this item, in display
	// order. A host that doesn't know actions ignores them.
	Actions []Action `json:"actions,omitempty"`
}

// Action is a choice the host offers for an Item. Kind and Meta are
// host-defined; this layer only carries them.
type Action struct {
	// ID identifies the action within its item, and is what the host passes
	// back when the listener chooses it.
	ID string `json:"id"`
	// Title is the action's label.
	Title string `json:"title"`
	// Kind names the kind of action for this host, such as command or url.
	// A host that doesn't know it decides how to draw the action.
	Kind string `json:"kind,omitempty"`
	// URL is the page to open for a url action; it is http or https.
	URL string `json:"url,omitempty"`
	// Meta is JSON the host may interpret; the plugin host doesn't.
	Meta map[string]any `json:"meta,omitempty"`
}

// Page asks a plugin for one of its contributed pages. A plugin that exports
// page answers it; one that only exports browse, as MyGo Music's do, is asked
// through Browse.
func (p *Plugin) Page(id string) (*Page, error) {
	if p.Can("page") {
		var out Page
		if err := p.Call("page", map[string]string{"id": id}, &out); err != nil {
			return nil, err
		}
		return &out, nil
	}
	return p.Browse(id)
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
