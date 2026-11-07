package main

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

func (h *host) view(c *ui.Context) {
	t := c.Theme()
	c.Root().Background(t.Background)
	if h.win == nil {
		return
	}
	ui.Row(c).Fill().Children(func() {
		h.sidebar(c)
		ui.Column(c).Grow(1).Fill().Children(func() {
			h.header(c)
			if h.notice != "" {
				ui.Text(c, h.notice).FontSize(11).TextColor(t.TextMuted).Padding(0, 16, 4, 16)
			}
			switch {
			case h.cur == nil:
				h.message(c, h.err)
			case h.needsConsent():
				h.consentView(c)
			case h.err != "":
				h.message(c, h.err)
			case h.busy:
				ui.Text(c, "Loading…").TextColor(t.TextMuted).Padding(24)
			default:
				h.listView(c)
			}
			h.nowBar(c)
		})
	})
}

func (h *host) sidebar(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Width(216).Fill().Padding(10).Gap(2).Background(t.Surface).Children(func() {
		name := "mygo-plugin"
		if h.cur != nil {
			name = h.cur.Manifest().Name
		}
		ui.Text(c, name).FontSize(14).Bold().Padding(8, 8, 10, 8)
		for i, n := range h.nav {
			btn := ui.ButtonBase(c.Key(fmt.Sprintf("nav-%d", i))).FillWidth().Padding(7, 10).Radius(6)
			if i == h.navAt {
				btn.Background(t.Accent.Alpha(0.14))
			}
			btn.Children(func() {
				ui.Text(c, navTitle(n)).FontSize(13).TextColor(t.Text).SingleLine()
			})
			if btn.Clicked() {
				h.openNav(i)
			}
		}
		ui.Spacer(c)
		if h.signIn != "" {
			ui.Text(c, h.signIn).FontSize(10).TextColor(t.TextMuted)
		}
		ui.Text(c, "native host · no webview").FontSize(10).TextColor(t.TextMuted).Padding(8, 0)
	})
}

func (h *host) header(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Padding(12, 16).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Gap(1).Children(func() {
			title := h.title
			if title == "" {
				title = "mygo-plugin host"
			}
			ui.Text(c, title).FontSize(19).Bold().SingleLine()
			if h.subtitle != "" {
				ui.Text(c, h.subtitle).FontSize(11).TextColor(t.TextMuted).SingleLine()
			}
		})
		if ui.TextInput(c, &h.query).Width(280).Placeholder("Search tracks").Submitted() {
			h.search()
		}
	})
}

func (h *host) listView(c *ui.Context) {
	t := c.Theme()
	ui.List(c, &h.list, len(h.rows), func(i int) { h.row(c, i) }).Grow(1).Padding(4, 8).Children(func() {
		if len(h.rows) == 0 {
			ui.Text(c, "Nothing here.").TextColor(t.TextMuted).Padding(20)
		}
	})
}

func (h *host) row(c *ui.Context, i int) {
	if i < 0 || i >= len(h.rows) {
		return
	}
	r := h.rows[i]
	t := c.Theme()
	if r.kind == rowHeader {
		ui.Text(c, r.title).FontSize(11).Bold().TextColor(t.TextMuted).Padding(14, 12, 4, 12)
		return
	}
	btn := ui.ButtonBase(c.Key(fmt.Sprintf("row-%d", i))).FillWidth().Padding(6, 10).Radius(8)
	if btn.Hovered() {
		btn.Background(t.SurfaceHover)
	}
	btn.Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Avatar(c, r.title, nil).Size(32, 32)
			ui.Column(c).Grow(1).Gap(1).Children(func() {
				ui.Text(c, r.title).FontSize(13).SingleLine()
				if r.subtitle != "" {
					ui.Text(c, r.subtitle).FontSize(11).TextColor(t.TextMuted).SingleLine()
				}
			})
			if r.kind == rowTrack {
				ui.Text(c, clock(r.track.Duration)).FontSize(11).TextColor(t.TextMuted)
			}
		})
	})
	if btn.Clicked() {
		h.activate(i)
	}
}

func (h *host) consentView(c *ui.Context) {
	t := c.Theme()
	perms := h.cur.Manifest().Permissions
	ui.Column(c).Fill().Padding(28).Gap(10).Children(func() {
		ui.Text(c, "Network permission").FontSize(18).Bold()
		ui.Text(c, fmt.Sprintf("%s asks to reach these hosts:", h.cur.Manifest().Name)).
			TextColor(t.TextMuted)
		for _, host := range perms.Network {
			ui.Text(c, "• "+host).FontSize(13)
		}
		if len(perms.Process) > 0 {
			ui.Text(c, "It also declares programs it may start. This host starts none.").
				FontSize(11).TextColor(t.TextMuted)
		}
		ui.Text(c, "The plugin runs in its own sandbox; this grant only opens the hosts above.").
			FontSize(11).TextColor(t.TextMuted)
		ui.Row(c).Gap(8).Padding(8, 0).Children(func() {
			if ui.PrimaryButton(c, "Allow network").Clicked() {
				h.allow()
			}
			if ui.Button(c, "Not now").Clicked() {
				h.err = "Network permission was not granted."
			}
		})
	})
}

func (h *host) message(c *ui.Context, text string) {
	if text == "" {
		return
	}
	ui.Text(c, text).TextColor(c.Theme().TextMuted).Padding(28)
}

func (h *host) nowBar(c *ui.Context) {
	if h.now == "" {
		return
	}
	t := c.Theme()
	ui.Row(c).Padding(10, 16).Gap(10).AlignItems(ui.Center).Background(t.Surface).Children(func() {
		ui.Column(c).Grow(1).Gap(1).Children(func() {
			ui.Text(c, h.now).FontSize(12).SingleLine()
			state := "playing"
			if !h.playing {
				state = "paused"
			}
			ui.Text(c, state+" · "+h.cur.Manifest().Name).FontSize(10).TextColor(t.TextMuted)
		})
		label := "Pause"
		if !h.playing {
			label = "Play"
		}
		if ui.Button(c, label).Clicked() {
			h.toggle()
		}
		if ui.Button(c, "Stop").Clicked() {
			h.stop()
		}
	})
}
