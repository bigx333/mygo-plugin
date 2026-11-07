# Changelog

mygo-plugin follows Semantic Versioning. Before 1.0, a minor version may change the API, and a patch version fixes bugs. The manifest and the host API freeze at 1.0.

## 0.6.0

### Added

- `host.t` and `locales/<language>.json`, so a plugin's text follows the listener's language; 17 of the 23 public plugins call it.
- A compatibility test that calls `home()` on a real public plugin, not just compiles it.

## 0.5.0

### Added

- Registry health: `index` compiles each plugin's module without running it, records its exports and whether it still loads, and keeps a broken plugin in the registry with the error.
- `install` by plugin id from `registry.json`, with `-version` and `-sha256`, and a checksum from the release asset when GitHub reports one.
- Install-time consent: a plugin's network and process permissions must be accepted with `--allow-network` and `--allow-process`, and `--yes` is required without a terminal.
- `mygo-plugin dev`: rechecks a plugin whenever its files change.
- `HostTypeScript`, `spec/host.d.ts` and the `plugintest` package for plugin authors.
- An experimental host-rendered page schema: `Page`, `Section`, `Item`, `Plugin.Browse`, and `spec/page.schema.json`.
- `Options.Allowed`, so the app decides which network hosts a plugin may reach.
- `examples/host`: a reference native MyGo host with consent, search, browse and playback.
- Registry trust signals: stars, last push, license, archived; only public repositories are crawled.
- `mygo-plugin site`: a static store page with permissions, health, trust signals and install commands.

## 0.2.0

### Added

- `registry.json`, built from the `mygo-music-plugin` GitHub topic: 23 plugins from 27 repositories, one entry per id, the newest version winning.
- `mygo-plugin index` and `mygo-plugin install owner/repo <dir>`.
- Install from a release zip, through the asset API for private repositories, or from the source archive when there is no release.
- Builtin navigation entries no longer need a title, as existing MyGo Music plugins omit it.

## 0.1.2

### Fixed

- A deadlock when a plugin read `host.storage` while its module was evaluated; storage has its own mutex now.

## 0.1.1

### Added

- English README, with Chinese as `README.zh-CN.md`.
- `docs/trust-model.md`.

### Fixed

- The README no longer claims MyGo lacks untrusted WebView pages; it has `PageOptions{Untrusted: true}`.

## 0.1.0

### Added

- A plugin host for `plugin.json` modules: one moejs runtime per plugin, fetch allowlists, storage, crypto, sign-in prompts, timeouts, memory limits and stuck restarts.
- Install from a folder or zip, `List`, `Uninstall`, and the `mygo-plugin` CLI.
