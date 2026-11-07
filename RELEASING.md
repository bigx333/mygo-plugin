# Releasing

mygo-plugin follows [Semantic Versioning](https://semver.org). The 0.x line
keeps its usual meaning:

- **PATCH** (`0.6.1`, `0.6.2`): backward-compatible fixes only.
- **MINOR** (`0.7.0`): backward-compatible features, and before 1.0, changes
  that may break the Go API or the plugin contract. Every minor release notes
  any break in the changelog.
- **1.0.0**: when the manifest and the host API are frozen.

## Cadence

Release at a milestone, not on every merge: **at most one release a week**,
unless a security fix needs its own. Between releases, `main` carries the
changes and the changelog's `Unreleased` section accumulates them; a release
moves that section under the version.

The `0.1` to `0.6` tags were cut in a single day while the shape was still
being found. They stay as they are: a published tag is never deleted or moved,
because the Go module proxy caches it and a changed version is a different
version to every consumer. The next feature release after this policy is
`0.7.0`, once a week's worth of changes has accumulated.

## Dates

Dates belong in release names, not in module versions. Go modules want SemVer;
a CalVer major (`v2026.11.0`) tells every Go tool that the API broke every
year, and `+build` metadata is ignored when a version is resolved. If a date
helps, write it in the release title: `v0.7.0 — 2026-11-14`.

## Checklist

1. `go test ./...`, `go test -race ./...`, `go vet ./...`
2. `cd examples/host && go build ./...`
3. Move `Unreleased` in `CHANGELOG.md` under the new version
4. `git tag -a vX.Y.Z -m "mygo-plugin vX.Y.Z"` and push the tag
5. Create the GitHub release with the changelog section
6. If the registry or the site changed, run `index` and `site` and commit both
