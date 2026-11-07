# Reference host

A small MyGo app that runs `mygo-plugin` plugins in native UI: no web page, no
WebView. It shows the plugin's navigation, calls `browse` and `search`, draws
the rows itself, and plays a track's stream in Go.

```sh
# from the repository root
go run ./cmd/mygo-plugin install netease ./plugins --allow-network --yes
cd examples/host
go run . -plugins ../../plugins
```

Or point it at a plugin folder with `-plugins /path/to/plugins`, and keep its
plugin data somewhere with `-data /path/to/data`.

## What it does

- A sidebar from `plugin.json`'s `navigation`: `page` entries call `browse`;
  `home`, `liked` and `stations` use the builtins this host knows
- A network permission card on first use; the grant is kept in
  `consent.json` and passed to `plugin.Options.Allowed`, so the runtime has no
  network until it is given
- Search calls `search({ query, kind: "tracks" })` on Enter
- Rows for tracks, playlists, links, stations and shows; a click plays a track,
  opens a playlist, or follows a link's page
- Playback: `stream({ id, quality: "high" })`, then the host downloads the URL
  with the plugin's headers and decodes it with `mygo/audio`
- A now playing bar with pause and stop

The track, playlist and section types in `contract.go` are this app's reading
of MyGo Music's source contract, not part of `mygo-plugin`. Another host with
another kind of plugin defines its own.

## What it doesn't do

- Sign in: a plugin's `signInPrompt` text is shown in the sidebar, and pages
  that need an account show their error
- Covers: rows use letter avatars, not the plugin's images
- FLAC: the host asks for `high` (MP3); `mygo/audio` has no FLAC decoder
- One plugin at a time: it picks the first one it finds
- Install UI: use the `mygo-plugin` CLI

It is a reference, not a product: copy the shape, not the file.
