# Native UI: pages

Status: experimental, version 0.1. The shape and `spec/page.schema.json` may
change. Version 0.1 adds a Markdown body to a page and actions to an item;
both are optional, so a version 0 page still fits.

This note describes the data a plugin returns for a page it contributes. It does
not add to the boundary described in the README and
[trust-model.md](trust-model.md); those remain the reference for what a plugin
may reach.

## Data, not a view tree

A plugin's `browse` or `page` export returns what a page contains, not how it is
drawn: a title, optional Markdown and sections of items. The host turns that
into its own native components.

This keeps the host in charge:

- A page looks and behaves like the rest of the app: same rows, headers,
  keyboard handling, accessibility and theme.
- There is no HTML, CSS or UI toolkit in the plugin contract. A small data shape
  is easier for many hosts to render, and easier to change without breaking
  plugins.
- The host can improve layout, image handling and chrome without every plugin
  shipping a new version.
- The plugin describes content only; layout, chrome and actions stay with the
  host, as the README's native boundary says.

## The shape

```json
{
  "title": "Demo",
  "subtitle": "A page",
  "markdown": "## About\n\nSome **detail**.",
  "sections": [
    {
      "title": "Tracks",
      "items": [
        {
          "id": "1",
          "title": "One",
          "kind": "track",
          "actions": [
            { "id": "play", "title": "Play", "kind": "command" },
            { "id": "open", "title": "Open", "kind": "url", "url": "https://example.com/one" }
          ]
        }
      ]
    }
  ]
}
```

A `Page` has:

- `title` (required): the page's heading.
- `subtitle` (optional): one line under the heading.
- `markdown` (optional): a detail page's body in CommonMark. The host renders
  it above the sections. A host without Markdown support may ignore it, showing
  only the heading and sections.
- `sections` (required, may be empty): the page's groups, in display order.

A `Section` has:

- `title` (optional): the group heading. The host may show it or use its own.
- `items` (required): the rows in the group, in display order.

An `Item` has:

- `id` (required): stable within the page. It is what the host passes back when
  the listener chooses the item.
- `title` (required): the main label.
- `subtitle` (optional): a second line, such as an artist, album or description.
- `imageURL` (optional): a remote image, typically cover art. The host fetches,
  crops and scales it.
- `kind` (optional): a type name the host defines, such as `track`, `playlist`
  or `show`. A host that doesn't know the kind draws a plain row.
- `meta` (optional): a JSON object for fields this shape doesn't name. It is
  host-defined; a host that doesn't know a key ignores it.
- `actions` (optional): the choices the host offers for the item, in display
  order. See below.

An `Action` has:

- `id` (required): stable within its item. It is what the host passes back
  when the listener chooses the action.
- `title` (required): the label.
- `kind` (optional): a type name the host defines, such as `command` or `url`.
- `url` (optional): for a `url` action, the page to open. It is `http` or
  `https`; other schemes are not part of this contract.
- `meta` (optional): a JSON object for fields this shape doesn't name; host-
  defined, and a host that doesn't know a key ignores it.

The host decides how an action is drawn, as a button, a menu entry or a
keyboard shortcut, and what a `kind` means. A `command` might name its operation
in `meta`, a `url` is a page to open, and an unknown kind may be drawn its own
way or not at all. The plugin supplies a choice, not its behaviour.

The schema is `spec/page.schema.json` (JSON Schema 2020-12). It requires
`title` and `sections` on a page, `items` on a section, and `id` and `title` on
an item and on an action, and keeps `meta` as an object with additional
properties.

## What the host owns

The plugin supplies no view, no style and no behaviour. The host owns:

- layout: page size, section grouping, row height, spacing and scrolling
- chrome: window, sidebar, page headers, loading, empty and error states
- icons: names from the host's icon set, not image files; `imageURL` is content
  imagery, not an icon
- actions: how an item's `actions` are drawn, what their `kind`s mean, and
  what choosing one does: context menus, playback and navigation
- markdown: whether and how a page's `markdown` is rendered; a host without
  Markdown support may ignore it
- presentation details: image loading and cropping, theming, type sizes,
  localization, accessibility and keyboard handling

## MyGo Music's richer sections

MyGo Music's `browse` contract returns sections that may hold tracks,
playlists and shows, with more fields than the generic `Item`. This version does
not parse those entries. A host that accepts both shapes adapts them:

| MyGo Music entry | Generic projection |
| --- | --- |
| track | `Kind: "track"`; `id`, title, artist/album as `subtitle`, cover as `imageURL`; duration and other ids in `meta` |
| playlist | `Kind: "playlist"`; name as title, creator as `subtitle`, cover as `imageURL`; track count in `meta` |
| show | `Kind: "show"`; name as title, publisher as `subtitle`, artwork as `imageURL`; episode count and ids in `meta` |

`kind` values and `meta` keys are host-defined, not a stable contract: they are
a way for a host to keep what the generic fields don't cover. A host may instead
render MyGo Music's own types directly. This package defines only the generic
`Page`, `Section`, `Item` and `Action`; the richer fields are the host's to
adapt.

## Version 0.1

This is an experiment. Expect fields to be added, and possibly renamed, before
it settles. Hosts should ignore fields they don't know and tolerate empty
`sections` and `items`; plugins should omit unused fields rather than send empty
strings. There is no version negotiation yet.
