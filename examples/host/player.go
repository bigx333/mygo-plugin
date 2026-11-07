package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/egoist/mygo/audio"
	plugin "github.com/yxxbc/mygo-plugin"
	"github.com/yxxbc/mygo-plugin/adapters/music"
)

func (h *host) play(it plugin.Item) {
	if h.cur == nil || h.adapter == nil {
		return
	}
	id := metaString(it.Meta, "trackID")
	if id == "" {
		return
	}
	name := it.Title
	h.notice = "Loading " + name
	h.win.Invalidate()
	go func() {
		s, err := h.adapter.Stream(id)
		if err != nil {
			h.fail(err)
			return
		}
		f, path, err := download(s)
		if err != nil {
			h.fail(err)
			return
		}
		player, err := audio.OpenStream(f)
		if err != nil {
			f.Close()
			os.Remove(path)
			h.fail(fmt.Errorf("decode %s: %w", s.Type, err))
			return
		}
		player.OnEnd(func() {
			h.win.Update(func() {
				if h.player == player {
					h.playing = false
				}
			})
		})
		player.Play()
		h.win.Update(func() {
			h.stopPlayer()
			h.player, h.tmp, h.now, h.playing = player, path, name, true
			h.notice = ""
			h.err = ""
		})
	}()
}

func (h *host) fail(err error) {
	h.win.Update(func() {
		h.err = err.Error()
		h.notice = ""
	})
}

func (h *host) toggle() {
	if h.player == nil {
		return
	}
	if h.playing {
		h.player.Pause()
		h.playing = false
	} else {
		h.player.Play()
		h.playing = true
	}
	h.win.Invalidate()
}

func (h *host) stop() {
	h.stopPlayer()
	h.now = ""
	h.playing = false
	h.win.Invalidate()
}

func (h *host) stopPlayer() {
	if h.player != nil {
		h.player.Close()
		h.player = nil
	}
	if h.tmp != "" {
		os.Remove(h.tmp)
		h.tmp = ""
	}
}

func download(s *music.Stream) (*os.File, string, error) {
	req, err := http.NewRequest(http.MethodGet, s.URL, nil)
	if err != nil {
		return nil, "", err
	}
	for k, v := range s.Headers {
		req.Header.Set(k, v)
	}
	resp, err := (&http.Client{Timeout: 2 * time.Minute}).Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("audio: %s", resp.Status)
	}
	f, err := os.CreateTemp("", "mygo-plugin-host-*")
	if err != nil {
		return nil, "", err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, "", err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, "", err
	}
	return f, f.Name(), nil
}
