# mygo-plugin

[简体中文](README.zh-CN.md)

[![Listed on Awesome MyGo](https://awesomemygo.dev/badge.svg)](https://awesomemygo.dev/plugins)

A native plugin host for MyGo apps: pure Go, in-process, no WebView. A plugin is a `plugin.json` plus a JavaScript or TypeScript module, run by [moejs](https://github.com/MoeForge/moejs) in a runtime of its own; the interface is still drawn by the host with MyGo's native UI.

This is not another WebView plugin container. A plugin gets no DOM and cannot draw the interface: it provides data, and the host decides how to show it. However heavy a plugin is, the app stays a MyGo app.

The `plugin.json` shape is the one MyGo Music uses, so existing plugins, such as the NetEase one, load unchanged.

## Why a separate project

MyGo's own `mygo.Plugin` is a **compile-time** capability module: a Go service plus an npm package, registered with `mygo.Use`, callable only from trusted pages. It suits official capabilities (fetch, sqlite, terminal), not installable third-party plugins.

MyGo Music has a complete third-party plugin runtime, but it lives in `internal/plugin`, tied to its music types, and other apps cannot take it. This repository extracts the host: manifest, install, permissions, runtime, limits and CLI, with no dependency on music.

## What it does

- One moejs runtime per plugin, with its own heap and event loop
- ES modules, async/await, TypeScript (when `main` ends in `.ts`)
- `host` API: storage, crypto (MD5 / AES-ECB), language, setting, openURL, signInPrompt
- `fetch` limited to `permissions.network` hosts; no permission means no network
- Call timeouts, a 256 MiB memory limit, and automatic restart when stuck
- Install from a folder, a ZIP or GitHub, by id, version and checksum, with explicit consent for network and process permissions
- A registry of the `mygo-music-plugin` topic, rebuilt weekly, with a health check that compiles each module and records its exports
- TypeScript declarations for the host global and a `plugintest` package for plugin authors
- An experimental host-rendered page schema for plugin pages
- `mygo-plugin` CLI: `check`, `pack`, `new`, `list`, `index`, `install`, `dev`
- Manifest JSON Schema: `spec/plugin.schema.json`

## Quick start

```go
import plugin "github.com/yxxbc/mygo-plugin"

p, err := plugin.Load("plugins/netease-haruhikage", plugin.Options{
    AppVersion: "0.1.26",
    DataDir:    filepath.Join(configDir, "mygo-plugin"),
    Host: plugin.Host{
        Language:     func() string { return "zh-CN" },
        OpenURL:      func(u string) error { return mygo.Shell.OpenExternal(u) },
        SignInPrompt: func(id string, p plugin.SignInPrompt) { showQR(p.QR, p.Text) },
    },
})
if err != nil {
    return err
}
defer p.Close()

var result struct {
    Tracks []struct {
        ID   string `json:"id"`
        Name string `json:"name"`
    } `json:"tracks"`
}
err = p.Call("search", map[string]any{"query": "周杰伦", "kind": "tracks", "offset": 0}, &result)
```

`Options.DataDir` is required and must belong to the app: the same plugin installed in two apps must not share its session and storage.

`Call`'s contract belongs to the app, not to this layer: whatever a plugin exports, the app calls. MyGo Music's source contract (`search` / `browse` / `stream` / `lyrics` / `signIn`, ...) is one such contract, and existing plugins work with it directly.

## The native boundary

A plugin cannot touch the host's UI tree. The host builds its sidebar from `plugin.json`'s `navigation`; when the listener opens an entry:

- with `page`: call the plugin's `browse({ page })`, get data such as `{ title, sections: [...] }`, and draw it with MyGo's native components
- with `builtin`: show the host's own page (home, liked, podcasts, ...)

The playback **data plane stays in Go**: a plugin's `stream` returns only `{ url, headers, type }`; the host downloads, decodes and plays it, so audio bytes never cross into JavaScript. Lyrics work the same way: the plugin returns text, the host lays it out.

MyGo already separates trusted pages from untrusted ones: a plugin UI that needs HTML can run in a WebView with `PageOptions{Untrusted: true}`, where it calls no Go method and reaches only what the app's handlers allow. This host is for plugins that don't need HTML: native UI is drawn in Go, so third-party code cannot draw it. A plugin here has no page, no bridge and no DOM; it sees only the host functions its manifest declares, and the host renders its data with native views.

The threat model, and the questions a framework-level boundary would have to answer, are in [docs/trust-model.md](docs/trust-model.md).

That is where the performance comes from. moejs calls take microseconds, but the real saving is not putting audio, covers and lyrics through JSON. Keep calls coarse; don't build a universal RPC.

Icons are names in the host's icon set (`calendar`, `star`, `playlist`, ...), not files. On macOS a host can draw Apple's SF Symbols with `plugins/sfsymbols`; other platforms use their own fallback set. Plugins should not ship SVG, or every host has to deal with rasterization and design consistency.

## Host API

| Name | What it does |
| --- | --- |
| `host.platform` / `host.arch` | The host's platform |
| `host.dir` / `host.dataDir` | The plugin's folder / data folder |
| `host.storage.get/set/remove` | Per-plugin JSON KV, written to `storage.json` |
| `host.crypto.md5` / `host.crypto.aesEcbEncrypt` | Hex output, PKCS#7 padding |
| `host.language()` | A BCP 47 language tag |
| `host.setting(key)` | The value the listener gave this plugin's setting |
| `host.openURL(url)` | http/https only |
| `host.signInPrompt({ qr, text })` | What to show while signing in |
| `host.spawn` | Starts no programs in this version; calling it returns an error |

`permissions.process` is parsed but not executed by this host. A host that runs programs should implement it, and keep it deny-by-default.

## Permissions

```json
{
  "permissions": {
    "network": ["music.163.com", "interfacepc.music.163.com"],
    "process": ["bin/helper-*"]
  }
}
```

- Empty `network`: `fetch` has no network
- `example.com` matches exactly; `*.example.com` matches subdomains; `*` allows everything
- The allowlist is checked on every request, redirects included
- The app can narrow it further with `Options.Allowed`, asked host by host when a runtime starts
- `process` is a declaration, not a grant

## Registry

`registry.json` lists the plugins in the `mygo-music-plugin` topic, one entry per plugin id: 24 plugins from 30 repositories today. Each entry carries the manifest's permissions, a checksum of the release zip when GitHub reports one, and a health record: `index` compiles the module without running it and keeps the exports, or the error that stopped it.

```sh
GITHUB_TOKEN=... mygo-plugin index -o registry.json  # the token also reads release checksums
mygo-plugin install netease-haruhikage /path/to/plugins --allow-network
```

When two repositories share an id, the newer version wins, then the repository that sorts first. Installing a plugin is a trust decision: the command prints its network and process permissions and refuses to continue without `--allow-network` and `--allow-process`. The registry refreshes weekly from GitHub Actions.

## Installing and distributing

```go
plugin.InstallDir(src, pluginsDir)   // from a folder
plugin.InstallZip(zip, pluginsDir)   // from a ZIP; plugin.json at the root or in the one folder
plugin.List(pluginsDir)
plugin.Uninstall(pluginsDir, id)     // keeps data, so a reinstall keeps the session
```

```sh
mygo-plugin check -app-version 0.1.26 ./my-plugin
mygo-plugin pack -o my-plugin.zip ./my-plugin
mygo-plugin new my-plugin
mygo-plugin list ~/Library/Application\ Support/MyApp/plugins

# From a registry, with the release's checksum and explicit consent
mygo-plugin install netease-haruhikage /path/to/plugins --allow-network --yes
# A pinned version, by repository
mygo-plugin install -version 0.2.1 -sha256 <hex> owner/repo /path/to/plugins --yes
```

`install` reads `registry.json` for an id, takes the latest release or the version asked for, verifies the checksum when one is known, and installs atomically. A plugin that asks for network or programs needs `--allow-network` or `--allow-process`: the flags are the listener's consent, not the plugin's declaration. Without a terminal, pass `--yes`.

## For plugin authors

- [`spec/host.d.ts`](spec/host.d.ts) declares the `host` global; `plugin.HostTypeScript` returns the same text for generators. It is versioned with `HostTypeScriptVersion`.
- `plugintest.New(t, dir, plugin.Options{...})` loads a plugin for a test with a temporary data folder, a ten second call timeout and `t.Logf` for its console.
- `mygo-plugin dev .` rechecks the manifest and compiles the module whenever a file changes, printing the exports or the error.
- [`spec/plugin.schema.json`](spec/plugin.schema.json) checks `plugin.json` in an editor.

## Native pages (experimental)

A plugin may answer `browse` with a host-rendered page: a title, sections and items, with no view tree, HTML or CSS. `plugin.Browse(page)` decodes it into `Page`, `Section` and `Item`; [`spec/page.schema.json`](spec/page.schema.json) describes the JSON and [`docs/native-ui.md`](docs/native-ui.md) explains what the host owns. MyGo Music's richer sections (tracks, playlists, shows) are adapted by the host; this shape is a smaller, generic subset and may change.

Signatures, a registry index and installing straight from a GitHub release are not done yet; see the roadmap. Don't build a review backend for v0.1.

## Limits

- Memory: 256 MiB by default; going over interrupts the runtime, and the next call restarts it
- Time: 30s per call by default; code stuck in JavaScript is terminated
- One runtime per plugin; calls enter that runtime one at a time
- `import` is not available (a single-file module); bundle first if you need several files

## Development

```sh
go test ./...
go vet ./...

# Verify with a real plugin (not run in CI)
MYGO_PLUGIN_REAL=../mygo-music-netease MYGO_PLUGIN_NETWORK=1 go test -run TestReal -v
```

## Roadmap

- Deeplinks for installs
- Ed25519 signatures and install verification
- A real implementation of the process permission (allowlist plus a grant UI)
- Generate app-specific host types from a Go contract
- A standard icon name table with cross-platform fallbacks

## Reference host

[`examples/host`](examples/host) is a small native MyGo app that runs plugins with this module: a sidebar from `navigation`, `browse` and `search` rendered as native rows, a network consent card, and playback that downloads the stream in Go. It is the shape to copy for an app that hosts plugins, not a product.

## Versioning

[`CHANGELOG.md`](CHANGELOG.md) lists the releases. Before 1.0, a minor version adds features and may change the API; a patch fixes bugs. The manifest and the host API freeze at 1.0.

## Credits and license

MIT. The `plugin.json` field shape is compatible with MyGo Music's plugins; this repository is an independent implementation and contains no code from MyGo Music. MyGo is [egoist/mygo](https://github.com/egoist/mygo) (MIT).
