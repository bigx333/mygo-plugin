# Roadmap: from a MyGo Music plugin host to a MyGo plugin platform

Status: proposal, November 2026.

This document audits what mygo-plugin is, compares it with plugin systems that
have worked (and one that has not), and proposes a direction. It is deliberately
narrow: the goal is not to run VS Code extensions or Obsidian plugins. It is to
make third-party plugins a native, safe part of MyGo apps.

## Summary

mygo-plugin today is a **capability runtime with a music-shaped contract**. The
runtime — manifest, one moejs realm per plugin, permissions, limits, install,
registry, health, checksums — is app-agnostic and works. The contract is not:
every public plugin in the registry is a MyGo Music source, and the reference
host still knows what a track is.

That does not make it a bad runtime. It makes it an unfinished platform. The
next step is a **host-defined contract**: pages, commands and settings that any
MyGo app can expose, with MyGo Music as the first adapter rather than the model.

The recommended direction is **B: a MyGo-native plugin platform**, not a
universal plugin runtime and not only a music host.

## Evidence

### The registry is one ecosystem

- `mygo-music-plugin` topic: 29 public repositories, 23 unique plugin ids after
  deduplication, all MyGo Music sources.
- `mygo-plugin` topic: 0 repositories. There is no other runtime plugin format
  in the MyGo ecosystem to adopt.
- MyGo's own `plugins/` are 12 **compile-time Go packages** (`fetch`, `sqlite`,
  `terminal`, `sfsymbols`, ...). They are added with `mygo.Use`, have no
  manifest, and cannot be installed or run at runtime.
- awesome-mygo's community "plugins" (limu, MyGo MCP, layout-audit) are Go
  libraries too.

So a generic MyGo plugin system cannot inherit a contract from the ecosystem;
it has to define one. MyGo Music's format is the only existing contract and
should be supported as an adapter, not treated as the platform.

### What the existing plugins actually call

A scan of the 23 public plugins' modules:

| Host API | Plugins | mygo-plugin |
| --- | ---: | --- |
| `storage` | 23 | yes |
| `t` | 17 | yes (0.6.0) |
| `setting` | 15 | yes, strings only |
| `signInPrompt` | 14 | yes |
| `crypto` | 11 | yes |
| `spawn` | 8 | no |
| `webSignIn` | 7 | no |
| `language` | 4 | yes |
| `platform`, `arch`, `dir`, `dataDir` | 4 or fewer | yes |
| `openURL` | 2 | yes |
| `waitForRedirect` | 1 | no |

The core is covered; the remaining gaps are external programs (8), a sign-in
window (7), and one localhost redirect (1). That is a compatibility problem, not
a reason to change the architecture.

### MyGo's own trust model

MyGo defines trust for **pages**:

- The app's pages, custom schemes, `file:`, `about:` and `TrustedOrigins` are
  trusted; they call every bound method and every `mygo.Plugin`.
- `PageOptions{Untrusted: true}` makes a page someone else's: calls rejected,
  permissions and navigation through app handlers, communication through a
  custom scheme and `EmitPage`.
- `mygo.Plugin` is compile-time and callable only from trusted pages.

It does not define trust for **code**. There is no runtime for installing and
running third-party code, and no vocabulary for what such code may do. Apps
that want one build it themselves — MyGo Music did, and this project did.

The trust model proposal is [egoist/mygo#1422](https://github.com/egoist/mygo/issues/1422).
It is open for discussion; no commitment to a framework-level package.

## Prior art

| System | Isolation | UI model | Distribution | Lesson |
| --- | --- | --- | --- | --- |
| VS Code | Extension host process, RPC | Webview or no UI; declarative contributions | Marketplace, signatures | Contributions are data read before code runs; activation is lazy; a separate host protects the UI |
| Raycast | Extensions run in the app, React API | Host-rendered AppKit components via a reconciler | Store, review | Host-rendered UI can feel native, and keeps layout and chrome under the host |
| Figma | Sandboxed JS (QuickJS in Wasm) plus a UI iframe | Plugin UI is HTML in an iframe | Community, review | Split logic (no DOM) from UI (DOM); the bridge is explicit and narrow |
| Obsidian | None: Node and DOM | Plugin draws into the app | Community directory, light review | An unsandboxed ecosystem inherits the app's authority; permissions alone cannot fix it |
| WASI | Wasm, capability handles | N/A | N/A | Deny by default; no ambient authority; capabilities are unforgeable and explicit |
| HashiCorp go-plugin | Separate process, gRPC | N/A | App-specific | Process isolation gives crash and authority separation for native or third-party binaries |
| MCP | Host/client/server, JSON-RPC | N/A | Servers, local or remote | Versioned capability negotiation; the host owns consent and lifecycle |
| MyGo Music (`internal/plugin`) | In-process moejs per plugin | Host-rendered native pages | GitHub topic, deeplink, ZIP | The current incumbent: proof that in-process JS with a narrow host API is practical |

Sources: [VS Code architecture](https://github.com/microsoft/vscode-wiki/blob/main/Source-Code-Organization.md),
[Raycast's API and rendering](https://www.raycast.com/blog/how-raycast-api-extensions-work),
[Figma's plugin environment](https://developers.figma.com/docs/plugins/how-plugins-run/),
[Obsidian's plugin security](https://obsidian.md/help/plugin-security),
[WASI security](https://wasi.dev/security),
[go-plugin](https://github.com/hashicorp/go-plugin),
[MCP architecture](https://modelcontextprotocol.io/docs/2026-07-28/learn/architecture).

## Current implementation audit

Strengths:

- Pure Go runtime, one moejs realm per plugin, no cgo, no WebView.
- Manifest, capabilities, network allowlist, memory limit, call timeout,
  stuck restart, per-plugin storage.
- Install from folder, ZIP or GitHub; registry with health, checksums, stars,
  licenses; a static store page.
- TypeScript declarations, `plugintest`, a reference native host, generic
  `Page`/`Section`/`Item` contract.
- Honest trust model and threat table in `docs/trust-model.md`.

Gaps:

- The contract is music-shaped: `navigation`, `browse`, `search`, `stream`.
  The generic page contract exists but is not the primary path.
- `spawn`, `webSignIn` and `waitForRedirect` are missing.
- Consent is install-time plus an optional host callback; there is no durable
  runtime grant store or revocation.
- No signatures, publisher identity, or attestations.
- No process isolation, no crash isolation, no audit log.
- UI schema is minimal: no commands, menus, forms, actions or detail views.
- No app contract versioning or capability negotiation.
- One real host (the reference example), no production app integration.

## Direction

### A. MyGo Music plugin host

Keep the music contract, finish `spawn`/`webSignIn`, polish the reference host,
grow the 23-plugin registry. Safe, useful, and small. It caps the project as a
feature of one app's ecosystem.

### B. MyGo-native plugin platform (recommended)

Define a small, host-defined contract that any MyGo app can expose, with
MyGo Music as one adapter:

- **Pages**: the generic `Page`/`Section`/`Item` shape, versioned.
- **Commands**: `contributes.commands`, `runCommand`, menus and shortcuts.
- **Settings**: a small schema (string, password, boolean, select), rendered by
  the host, read through `host.setting`.
- **Capabilities**: `host.capabilities()` and a contract id/version, negotiated
  at load, LSP-style.
- **Trust**: durable grants, signatures, process allowlist, audit.
- **Adapters**: `mygo-music` maps tracks, playlists and streams; another app
  writes its own.

This is a platform rather than a music feature, without pretending to run every
other ecosystem's plugins.

### C. Universal plugin runtime

Run VS Code, Raycast, Figma or Obsidian plugins. Rejected: those APIs assume
DOM, Node, React or a document model; a compatibility layer would be larger
than this project, would require a WebView, and would lose the native
advantage. Importing their *lessons* is useful; importing their *plugins* is
not the goal.

### D. Framework-internal

Ask MyGo to own the host. The maintainer declined for MyGo Music
([#46](https://github.com/egoist/mygo-music/issues/46)) and left the trust
model open ([#1422](https://github.com/egoist/mygo/issues/1422)); the project
should stay a separate module and remain format-compatible.

## Target architecture

```
┌────────────────────────────── host app (MyGo native UI) ──────────────────────────────┐
│  pages, commands, settings, menus, navigation, consent UI, app services               │
│  ┌──────────────────────────── plugin SDK (Go + generated .d.ts) ──────────────────┐  │
│  │ contract id/version, capability negotiation, Page v1 renderer, adapters          │  │
│  └─────────────────────────────────────────────────────────────────────────────────┘  │
│  ┌──────────────────────────── mygo-plugin runtime ────────────────────────────────┐  │
│  │ manifest · install · registry · one moejs realm per plugin · limits · grants    │  │
│  │ host functions: storage, fetch, crypto, t, setting, openURL, signInPrompt       │  │
│  │ optional: spawn (allowlist + grant), webSignIn (host callback), redirect        │  │
│  └─────────────────────────────────────────────────────────────────────────────────┘  │
└───────────────────────────────────────────────────────────────────────────────────────┘
                          │                                    │
                    plugin.json                          registry.json
                 (code + contributions)              (identity, health, trust)
```

Three contracts, each versioned separately:

1. **Runtime contract** — manifest fields, host functions, limits. Owned by
   this project; bumps on breaking changes.
2. **App contract** — pages, commands, settings, capabilities an app exposes.
   Owned by the app; the plugin declares which contract it implements.
3. **Distribution contract** — identity, version, checksums, signatures,
   health, publisher. Owned by the registry.

## Roadmap

### Phase 0 — done

Runtime, MyGo Music compatibility, registry and store, SDK, reference host,
generic page contract, trust model. Version 0.6.0; next release 0.7.0.

### Phase 1 — Contract v2 (2–4 weeks)

Status: the manifest fields, commands, settings types, capability negotiation,
Page v1 and the notes plugin are in `main`; the music adapter and forms remain.

Deliverables:

- `contributes.pages`, `contributes.commands`, `contributes.settings` in the
  manifest; `Manifest.Pages()` and `Plugin.Page()` as the primary path.
- Page v1: list, detail, form, actions; frozen JSON schema; host renderer
  contract; `spec/page.schema.json` at 1.0.
- Commands: `commands()` / `runCommand({ id, args })`, host menus, shortcuts,
  command palette contribution.
- Settings schema: string, password, boolean, select; `host.setting` returns
  typed values; a settings form in the reference host.
- Capability negotiation: `contract`, `contractVersion`, `capabilities()`;
  refuse a plugin whose contract is unknown or too new.
- Music adapter package: MyGo Music's `browse`/`search`/`stream` to Page/commands,
  outside the core.
- A non-music example plugin (`examples/plugins/notes`) that uses pages; its
  command is ready for a host command UI.
- Remaining: extract the music adapter from `examples/host`; form contributions.

Exit criteria: the same reference host runs the music plugin and the non-music
plugin; both use Page v1; `go test ./...` and the host build pass.

### Phase 2 — Trust and compatibility (4–8 weeks)

Deliverables:

- Durable grants: consent stored per plugin and per capability; runtime reads
  it; revocation restarts the runtime; no network without a grant.
- `webSignIn` host callback and an untrusted-WebView implementation in the
  reference host; `waitForRedirect` in the runtime.
- `spawn` with a program allowlist, explicit grant, child process group, kill
  on close, and audit entries. Never enabled by default.
- Ed25519/minisign signatures for releases; publisher keys; checksums already
  in the registry; install refuses a bad signature.
- Registry contract tags (`mygo-music`, `generic`) and verified publisher.

Exit criteria: top 10 registry plugins signed; no ungranted network or process;
a security review of the manifest, install and spawn paths.

### Phase 3 — SDK and adoption (6–10 weeks)

Deliverables:

- `host.d.ts` generated from an app's contract; `plugintest` against a mock
  host; a conformance suite every host runs.
- `mygo-plugin dev` with a real host: reload on change, log stream, a plugin
  inspector.
- Templates: page, command, settings, music-source adapter.
- Integrate two real apps: haruhikage-app (music) and one non-music app.
- 10 non-music plugins in the registry (notes, RSS, git, system, ...).

Exit criteria: two hosts, 10 non-music plugins, time to first plugin under 10
minutes, no host-specific code in plugins.

### Phase 4 — Ecosystem and governance (next quarter)

Deliverables:

- Registry filters by contract, capability and health; verified publishers;
  download/like signals.
- Docs and examples; a plugin of the week; a compatibility matrix.
- Upstream conversation: a `mygo/pluginhost` package or a documented trust
  boundary; deeplink install; generated types from `mygo.Bind`.
- At least three maintainers and a security policy.

Exit criteria: 50 plugins, 5 hosts, 3 maintainers.

### Phase 5 — Isolation (later)

Optional, driven by real incidents:

- Subprocess or Wasm host for plugins that need native code or stronger
  isolation; crash containment; per-plugin audit trail.
- Keep the in-process moejs path for the common case.

## Metrics

North star: **plugins that run in at least two hosts without changes**.

Leading indicators:

- host API coverage of registry plugins (currently 17/23 for `t`, 23/23 storage)
- install success rate and checksum/signature coverage
- health checks passing
- time to first plugin for a new host
- non-music plugins and hosts

Guardrails:

- crashes per 1,000 plugin calls
- permission incidents (network or process without a grant)
- review latency and abandoned plugins

## Risks

| Risk | Mitigation |
| --- | --- |
| The contract calcifies around music | Freeze Page v1, ship a non-music example, extract the music adapter |
| Upstream changes MyGo's plugin model | Keep the runtime app-level, follow `mygo.Plugin` and untrusted pages, contribute the trust model |
| Security incident from `spawn` | Deny by default, explicit grants, process groups, audit; ship later than signatures |
| Ecosystem chicken-and-egg | Two real hosts and 10 non-music plugins before asking for more |
| API churn | Contract version and capability negotiation; weekly releases at most |
| Bus factor | Docs, tests, conformance suite, a second maintainer |
| License mismatch | Runtime MIT; adapters and plugins keep their own licenses; no copied AGPL code |
| Cross-platform gaps | CI on macOS, Linux and Windows for runtime and host; spawn/sign-in per platform |

## What we will not do

- Run VS Code, Obsidian, Raycast, Figma or Chrome extensions.
- Let plugins draw the UI tree, inject CSS, or open a WebView as their primary
  surface.
- Load native Go plugins into the app process.
- Call a plugin safe because it has a manifest. Signatures and grants are
  separate controls.
- Release every change. Milestones, at most weekly.

## Open questions

1. Does MyGo want a `pluginhost` package, or only the trust vocabulary?
2. Is in-process moejs the default for untrusted code, or must a host choose a
   process for anything beyond pages?
3. What is the smallest Page v1 that works across music, notes, mail and RSS?
4. Is a plugin identity a repository, a signing key, or both?
5. Where does install consent live: the host app, the framework, or the OS?
6. How do AGPL plugins and an MIT runtime coexist in one registry?

## Next two weeks

1. Write `docs/contract-v2.md` with Page v1, commands, settings and capability
   negotiation; freeze the JSON shapes.
2. Implement the manifest and API changes behind tests; keep `navigation` and
   `browse` working.
3. Extract the music adapter from `examples/host`; add a notes plugin.
4. Implement the durable grant store and route `Allowed` through it.
5. Release 0.7.0 at the milestone, not before.
