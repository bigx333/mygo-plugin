# Contract v2

Status: draft, November 2026. The shapes below are implemented behind tests;
Page v1 is still experimental and may change before 1.0.

A plugin and a host meet at three contracts. They version separately, and a
host refuses a plugin whose contract it does not implement.

## 1. Runtime contract

Owned by `mygo-plugin`. It covers `plugin.json`, the `host` global, limits,
install and the registry. A breaking change to any of these bumps the runtime's
minor version before 1.0, and the major at 1.0.

```json
{
  "id": "notes",
  "name": "Notes",
  "version": "0.1.0",
  "main": "index.js",
  "contract": "mygo-notes",
  "contractVersion": 1,
  "capabilities": ["network"],
  "permissions": {"network": ["api.example.com"]},
  "contributes": {
    "pages": [{"id": "home", "title": "Home", "icon": "note"}],
    "commands": [{"id": "new", "title": "New note"}]
  },
  "settings": [
    {"key": "token", "label": "Token", "type": "password"},
    {"key": "sync", "label": "Sync", "type": "boolean", "default": "true"}
  ]
}
```

- `contract` and `contractVersion` are the app's contract, not the runtime's.
  A plugin that leaves `contract` empty is accepted by any host (MyGo Music's
  plugins do this today).
- `capabilities` are what the plugin needs from the host. The host answers
  through `host.capability(name)`. A host may refuse a plugin whose
  capabilities it does not provide; this runtime exposes the question and
  leaves the policy to the host.
- `permissions` stay the runtime's capabilities: network hosts and programs.
  A host narrows them further with `Options.Allowed` or a durable grant.

## 2. App contract

Owned by the app. It defines pages, commands, settings and any app-specific
service calls (for MyGo Music, `search`, `browse`, `stream`, ...). The app
documents the contract id and version; the runtime only checks that they match.

### Pages

A page is data, never a view tree. The host owns layout, chrome, icons,
keyboard handling and theming.

```json
{
  "title": "Home",
  "subtitle": "A plugin, not a music app",
  "markdown": "Optional CommonMark shown above the sections.",
  "sections": [
    {
      "title": "Today",
      "items": [
        {
          "id": "welcome",
          "title": "Welcome",
          "subtitle": "From a plugin",
          "imageURL": "https://example.com/cover.png",
          "kind": "note",
          "meta": {"path": "notes/welcome.md"},
          "actions": [
            {"id": "open", "title": "Open", "kind": "command", "meta": {"command": "open"}},
            {"id": "web", "title": "Website", "kind": "url", "url": "https://example.com"}
          ]
        }
      ]
    }
  ]
}
```

- `title` and `sections` are required; a section needs `items`; an item needs
  `id` and `title`.
- `kind` and `meta` are host-defined. A host that does not know a kind draws a
  plain row and ignores the meta it does not know.
- `actions` are choices the host offers for an item. `kind: "command"` calls
  the plugin's command named in `meta.command`; `kind: "url"` opens `url`.
  Other kinds are host-defined. A host may ignore actions entirely.
- `markdown` is the detail body. A host without a Markdown renderer may ignore
  it and show the sections alone.
- There is no HTML, CSS, script or view tree in a page. A plugin that needs
  those needs a WebView, which this contract does not provide.

`plugin.Page(id)` asks the plugin's `page({id})` export; a plugin with only
`browse` (MyGo Music) is asked through that. `Manifest.Pages()` returns
`contributes.pages` or, for compatibility, `navigation`.

### Commands

A command is a named action the host may put in a menu, a shortcut or a
command palette. It is declared in `contributes.commands` and implemented by
one export:

```js
export function command({ id, args }) {
  if (id === "new") return { id: "note-1" };
  throw new Error("unknown command: " + id);
}
```

`plugin.RunCommand(id, args)` calls it; a plugin without the export returns
`ErrUnsupported`. The runtime carries no menu or shortcut model: the host
decides where a command appears.

### Settings

The manifest's `settings` only describe fields; the host renders the form and
stores the values. `host.setting(key)` returns the current value as a string.

| type | value | host control |
| --- | --- | --- |
| `string` or empty | any text | text field |
| `password` | any text | obscured field, never logged |
| `boolean` | `"true"` or `"false"` | switch |
| `select` | one of `options` | pop-up menu |

`default` is the value before the listener sets one. A host that does not know
a type shows a text field. Secrets belong in `password`, not in `string`; the
runtime does not encrypt the settings file.

## 3. Distribution contract

Owned by the registry. An entry carries:

- `id`, `name`, `version`, `repo`, `main`, `minAppVersion`
- `permissions`, `contract`, `contractVersion`
- `health` (compiles or not, exports, checkedAt)
- `sha256` of the release zip when GitHub reports one
- trust signals: stars, last push, license, archived
- later: publisher key, signature, verified flag, and `contract` in the
  registry file

Installing is a trust decision: the CLI prints the permissions and requires
`--allow-network` / `--allow-process`; a host can add its own consent and
`Options.Allowed`.

## Compatibility

- MyGo Music plugins keep working: `navigation` is read when there are no
  `contributes.pages`, and `browse`/`search`/`stream` are ordinary calls.
- A generic plugin declares `contributes.pages` and exports `page`.
- A host refuses a plugin whose `contract` differs from the one it provides,
  and a `contractVersion` newer than it implements.
- The runtime does not know what a "track" or a "note" is. Adapters do.

## Open

- Form contributions (fields and submit) are not in Page v1; settings are.
- Detail pages are Markdown for now; structured metadata panels come later.
- Signature and publisher identity depend on
  [egoist/mygo#1422](https://github.com/egoist/mygo/issues/1422).
- Command discovery at runtime (`commands()`) is not needed: the manifest is
  the source, and a host reads it before running code.
