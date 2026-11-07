package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// RegistrySchema is the version of the registry file this code writes.
const RegistrySchema = 1

// RegistryEntry is one plugin a registry knows, without its code.
type RegistryEntry struct {
	ID            string      `json:"id"`
	Name          string      `json:"name"`
	Version       string      `json:"version"`
	Description   string      `json:"description,omitempty"`
	Author        string      `json:"author,omitempty"`
	Homepage      string      `json:"homepage,omitempty"`
	Repo          string      `json:"repo"`
	Main          string      `json:"main,omitempty"`
	MinAppVersion string      `json:"minAppVersion,omitempty"`
	Permissions   Permissions `json:"permissions,omitempty"`
	// SHA256 is the sha256 of the latest release's first zip asset, when
	// GitHub lists one and a token was used.
	SHA256 string `json:"sha256,omitempty"`
	// Health is what the last build saw of the module: whether it compiles,
	// and what it exports when it does.
	Health *Health `json:"health,omitempty"`
	// Trust signals, when GitHub answered: stars, the last push, the
	// license, and whether the repository is archived.
	Stars    int        `json:"stars,omitempty"`
	PushedAt *time.Time `json:"pushedAt,omitempty"`
	License  string     `json:"license,omitempty"`
	Archived bool       `json:"archived,omitempty"`
}

// Health says whether a repository's module compiles, and what it exports
// when it does.
type Health struct {
	Status    string    `json:"status"` // "ok" or "error"
	Error     string    `json:"error,omitempty"`
	Exports   []string  `json:"exports,omitempty"`
	CheckedAt time.Time `json:"checkedAt"`
}

// Registry is a list of plugins, built from their repositories.
type Registry struct {
	Schema      int             `json:"schema"`
	GeneratedAt time.Time       `json:"generatedAt"`
	Entries     []RegistryEntry `json:"entries"`
}

// BuildRegistry reads the plugin.json and main module of each repository and
// collects them, one entry per plugin id. A repository that doesn't read is
// left out and reported; it doesn't take the others with it. One whose module
// doesn't compile is kept, with Health saying why.
func BuildRegistry(ctx context.Context, g *GitHub, repos []string) (*Registry, map[string]error) {
	repos = append([]string(nil), repos...)
	sort.Strings(repos)
	type result struct {
		repo      string
		m         *Manifest
		exports   []string
		healthErr error
		sha       string
		meta      *RepoInfo
		err       error
	}
	jobs := make(chan string)
	results := make(chan result)
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for repo := range jobs {
				m, err := g.Manifest(ctx, repo)
				if err != nil {
					results <- result{repo: repo, err: err}
					continue
				}
				// The manifest is already read: the second request is the
				// module, as Inspect makes it after its own Manifest call.
				exports, healthErr := g.compileMain(ctx, repo, m)
				meta, _ := g.Repo(ctx, repo)
				results <- result{
					repo:      repo,
					m:         m,
					exports:   exports,
					healthErr: healthErr,
					sha:       g.releaseSHA256(ctx, repo),
					meta:      meta,
				}
			}
		}()
	}
	go func() {
		for _, repo := range repos {
			jobs <- repo
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	byID := map[string]RegistryEntry{}
	failures := map[string]error{}
	for r := range results {
		if r.err != nil {
			failures[r.repo] = r.err
			continue
		}
		entry := RegistryEntry{
			ID:            r.m.ID,
			Name:          r.m.Name,
			Version:       r.m.Version,
			Description:   r.m.Description,
			Author:        r.m.Author,
			Homepage:      r.m.Homepage,
			Repo:          r.repo,
			Main:          r.m.Main,
			MinAppVersion: r.m.MinAppVersion,
			Permissions:   r.m.Permissions,
			SHA256:        r.sha,
			Health:        &Health{Status: "ok", Exports: r.exports, CheckedAt: time.Now().UTC().Truncate(time.Second)},
		}
		if r.healthErr != nil {
			entry.Health.Status = "error"
			entry.Health.Error = r.healthErr.Error()
			entry.Health.Exports = nil
		}
		if r.meta != nil {
			entry.Stars = r.meta.Stars
			entry.PushedAt = r.meta.PushedAt
			entry.Archived = r.meta.Archived
			if r.meta.License != nil {
				entry.License = r.meta.License.SPDXID
			}
		}
		old, ok := byID[entry.ID]
		if !ok || newerEntry(entry, old) {
			byID[entry.ID] = entry
		}
	}
	reg := &Registry{
		Schema:      RegistrySchema,
		GeneratedAt: time.Now().UTC().Truncate(time.Second),
	}
	for _, e := range byID {
		reg.Entries = append(reg.Entries, e)
	}
	sort.Slice(reg.Entries, func(i, j int) bool { return reg.Entries[i].ID < reg.Entries[j].ID })
	return reg, failures
}

// newerEntry says whether a replaces b: a newer version, or the same version
// from the repository that sorts first, so that a build is deterministic.
func newerEntry(a, b RegistryEntry) bool {
	av, aok := parseVersion(a.Version)
	bv, bok := parseVersion(b.Version)
	if aok && bok {
		for i := range av {
			if av[i] != bv[i] {
				return av[i] > bv[i]
			}
		}
	}
	return a.Repo < b.Repo
}

// WriteFile writes the registry as JSON, replacing the file whole.
func (r *Registry) WriteFile(path string) error {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp := path + ".part"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ReadRegistry reads a registry file.
func ReadRegistry(path string) (*Registry, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r Registry
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if r.Schema != RegistrySchema {
		return nil, fmt.Errorf("%s: schema %d, want %d", path, r.Schema, RegistrySchema)
	}
	return &r, nil
}
