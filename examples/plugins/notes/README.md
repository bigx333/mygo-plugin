# Notes

A non-music plugin for `mygo-plugin`. It shows how the generic page contract
works: `plugin.json` declares a page under `contributes.pages`, and `index.js`
exports `page({ id })`, which answers with sections and items instead of
tracks. It asks for no permissions and makes no network calls.

`index.js` also exports `command({ id, args })`, which answers `hello` with a
small object. The reference host does not have a command UI yet; the export is
there for hosts that call it.

## Running it

```sh
# from the repository root
cd examples/host
go run . -plugins ../plugins
```

The host discovers every folder in `-plugins`, so `examples/plugins/notes`
shows up as "Notes" with a single page in the sidebar. Its test loads this
folder directly:

```sh
cd examples/host
go test -run TestNotesPlugin ./...
```
