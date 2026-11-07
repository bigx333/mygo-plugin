# Reference host

A small MyGo app that runs `mygo-plugin` plugins in native UI: no web page, no
WebView. It shows a plugin's pages, draws the rows itself, and, for a music
plugin, calls `browse` and `search` and plays a track's stream in Go.

```sh
# from the repository root
go run ./cmd/mygo-plugin install netease ./plugins --allow-network --yes
cd examples/host
go run . -plugins ../../plugins
```

Or point it at a plugin folder with `-plugins /path/to/plugins`, and keep its
plugin data somewhere with `-data /path/to/data`.

## Two kinds of plugins

- **Generic plugins** declare `contributes.pages` and export
  `page({ id })`. The host draws the sections and items they return, with no
  music types. `examples/plugins/notes` is a working one: run the host against
  `examples/plugins` (see above) and its "Notes" page appears in the sidebar.
  `testdata/generic` covers the same path in the root repository.
- **Music plugins** are MyGo Music sources: they declare `navigation` and
  export `browse`. The host adapts them through `adapters/music`, which turns
  tracks, playlists, stations and links into generic `Page` items.

`TestNotesPlugin` in `generic_test.go` loads `examples/plugins/notes` and
checks its page; `TestGenericPluginPages` in `host_test.go` does the same for
`testdata/generic`.

## What it does

- A sidebar from the pages a plugin contributes: for a generic plugin,
  `contributes.pages`, answered by `page`; for a music plugin,
  `navigation`, where `page` entries call `browse` and `home`, `liked` and
  `stations` use the builtins this host knows
- A network permission card on first use; the grant is kept in
  `consent.json` and passed to `plugin.Options.Allowed`, so the runtime has no
  network until it is given
- Search calls `search({ query, kind: "tracks" })` on Enter
- Rows for tracks, playlists, links, stations and shows; a click plays a track,
  opens a playlist, or follows a link's page
- Item actions: `kind: "command"` calls the plugin's command, `kind: "url"`
  opens the page; they show as small buttons under the row
- A command bar from `contributes.commands`; its answer shows in the notice
  line
- A Markdown body when a page has `markdown`, above its sections
- Playback: `adapters/music` resolves `stream({ id, quality: "high" })`, then
  the host downloads the URL with the plugin's headers and decodes it with
  `mygo/audio`
- A now playing bar with pause and stop

## What it doesn't do

- Sign in: a plugin's `signInPrompt` text is shown in the sidebar, and pages
  that need an account show their error
- Covers: rows use letter avatars, not the plugin's images
- FLAC: the host asks for `high` (MP3); `mygo/audio` has no FLAC decoder
- One plugin at a time: it picks the first one it finds
- Install UI: use the `mygo-plugin` CLI

It is a reference, not a product: copy the shape, not the file.
