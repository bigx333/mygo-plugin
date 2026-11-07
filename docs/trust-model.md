# Trust model

Status: proposal, v0.1.1. Scope: MyGo apps that host third-party plugins extending the app's native UI.

This document describes the boundary this host implements, and the parts only the framework or the app can decide. It is meant to be argued with.

## Assets

- The listener's accounts and sessions, which a plugin may be allowed to use
- The listener's files, clipboard, network and OS permissions
- The app's integrity and availability: its UI thread, memory and startup
- Other plugins' code and data

## Adversary

- A malicious plugin author
- A compromised release, dependency or update channel
- An honest plugin with a bug: an endless loop, a memory blowup, a leaked session
- Malicious data returned by a service, which is not code but may reach the host's renderer

## What MyGo already defines

MyGo defines trust for **pages**, not for **code**:

- A window trusts the app's own pages, custom schemes, `file:` and `about:`, and `TrustedOrigins`. A trusted page calls every bound method and every `mygo.Plugin`.
- `PageOptions{Untrusted: true}` makes a page someone else's: its calls are rejected, it gets no permission without an app handler, and the app talks to it through its own scheme and `EmitPage`. This is the right tool for a plugin UI that needs HTML, and its limits are documented in `docs/frontend.md`.
- `mygo.Plugin` is compile-time: a Go service added with `mygo.Use`, callable only from trusted pages. It is not a sandbox for third-party code.
- MyGo has no runtime for installing and running third-party code. Apps that want one build it themselves: MyGo Music runs `plugin.json` modules in moejs, and `mygo-plugin` does the same as a reusable host.

The gap is therefore not untrusted UI. It is a shared model for **third-party code** that runs in the app process, with no ambient authority, and for the native UI it contributes without a page.

## Proposed code trust boundary

1. **No ambient authority.** A plugin starts with the JavaScript standard library and only the host functions its manifest declares. It has no DOM, no bridge, no filesystem outside its data folder, and no network unless `permissions.network` names the hosts.
2. **Capabilities, not flags.** The manifest lists network hosts, programs, and the UI pages it contributes. Everything else is denied. Grants are explicit, per plugin, revocable, and survive restarts only as a recorded decision.
3. **One runtime per plugin.** Separate heap, globals and event loop. Calls enter that runtime one at a time. Memory is capped; calls time out; code that stops taking tasks is terminated and restarted.
4. **The data plane stays in Go.** A plugin returns a media URL, headers and metadata; the host fetches, decodes and plays. Audio, video, covers and lyrics never cross into JavaScript as bytes. This is both a performance and a trust decision: the plugin never holds the decoded content.
5. **Native UI is host-rendered.** A plugin cannot touch the UI tree. It contributes data (navigation entries, page sections, items) and the host draws it with its own native components. Icons are names in the host's set, not files.
6. **Distribution is code.** Install verifies the manifest, the host version and, eventually, a signature; installs atomically; and keeps the previous version for rollback. A plugin update is a new trust decision, not a silent file swap.
7. **Fail closed.** Unknown capabilities are an error, not ignored. A host function the host does not implement returns an error. A missing permission is a failure, not a fallback.

## Threats and controls

| Threat | Control |
| --- | --- |
| Reading or exfiltrating a session | Per-plugin storage; network allowlist on every request, redirects included |
| Reaching the intranet or cloud metadata | Allowlist is explicit hosts; no wildcard by default; loopback only for granted local programs |
| Reading files | No filesystem API; `dataDir` only; `removeDataDir` cannot leave it |
| Starting programs | `permissions.process` is a declaration; the host starts nothing in this version |
| Freezing the app | Per-call timeout; stuck runtimes are interrupted off the UI thread |
| Exhausting memory | 256 MiB per runtime; the runtime is restarted after an overrun |
| Replacing another plugin's data | One data folder per plugin id; the host never shares a runtime |
| Shipping a malicious update | Signed releases and pinned versions are on the roadmap; until then, install is a local, explicit action |
| Spoofing app UI | Plugins cannot draw chrome, dialogs or prompts; `signInPrompt` is rendered by the host |

## What the framework could provide

These are the pieces an app cannot settle alone, and the questions this proposal asks:

- A documented boundary between page trust and code trust, so an app can say what a plugin is allowed to do without inventing vocabulary.
- A `pluginhost` package, or only a specification, for manifest, install, permissions, limits and capability negotiation.
- Deeplink registration for `mygo://install-plugin?...`-style installs, and a consent surface the OS can attribute to the app.
- Generated TypeScript types for the host contract, so a plugin's calls cannot drift from the app's Go methods.
- A native UI contribution schema small enough to freeze: lists, sections, items and actions, with the host owning layout and chrome.
- Versioned capability negotiation, LSP-style, so a host update does not silently break a plugin.

## Non-goals

- Sandboxing native Go code loaded with `plugin.Open` or cgo. In-process native code has the app's authority; no manifest can change that.
- Replacing `PageOptions{Untrusted: true}` for plugins that genuinely need HTML.
- A universal UI toolkit for plugins. The host draws; the plugin only describes data.
- DRM or copyright enforcement. That belongs to the service and the listener's account.

## Open questions

1. Should MyGo own the runtime, or only the specification and the trust vocabulary?
2. Is a per-plugin JavaScript runtime the right default, or should the host require a separate process for untrusted code?
3. How should a host declare its capability schema, and how should plugins negotiate versions?
4. What is the smallest native UI contribution schema that is still useful across music, mail, notes and readers?
5. What does a plugin identity mean across hosts: a repository, a signing key, or both?
6. Where should install consent live: the host app, the framework, or the OS?

A reference implementation of this model is [mygo-plugin](https://github.com/yxxbc/mygo-plugin); MyGo Music's `internal/plugin` is an earlier one. Neither is normative. The model should stand on its own.
