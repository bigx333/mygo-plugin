package plugin

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestWriteSite(t *testing.T) {
	now := time.Date(2026, 11, 7, 5, 0, 0, 0, time.UTC)
	reg := &Registry{
		Schema:      RegistrySchema,
		GeneratedAt: now,
		Entries: []RegistryEntry{{
			ID:          "x",
			Name:        `<script>alert(1)</script>`,
			Version:     "1.0.0",
			Description: `<b>hello</b>`,
			Repo:        "a/b",
			Permissions: Permissions{Network: []string{"*"}},
			Health:      &Health{Status: "ok", Exports: []string{"home"}},
			Stars:       12,
			License:     "MIT",
			PushedAt:    &now,
		}},
	}
	var b bytes.Buffer
	if err := WriteSite(&b, reg); err != nil {
		t.Fatal(err)
	}
	s := b.String()
	if strings.Contains(s, "<script>alert") {
		t.Error("the plugin's name was not escaped")
	}
	if !strings.Contains(s, "&lt;script&gt;") {
		t.Error("escaped name missing")
	}
	if !strings.Contains(s, "mygo-plugin install x ./plugins --allow-network") {
		t.Error("install command missing")
	}
	if !strings.Contains(s, "★ 12") || !strings.Contains(s, "compiles · 1 exports") {
		t.Error("trust signals missing")
	}
}

func TestWriteSiteWithoutHealth(t *testing.T) {
	reg := &Registry{Schema: RegistrySchema, Entries: []RegistryEntry{{ID: "x", Repo: "a/b"}}}
	var b bytes.Buffer
	if err := WriteSite(&b, reg); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "not checked") {
		t.Error("missing health state")
	}
}
