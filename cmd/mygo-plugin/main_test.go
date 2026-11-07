package main

import (
	"flag"
	"testing"
)

func TestParseArgs(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	out := fs.String("o", "", "")
	yes := fs.Bool("yes", false, "")
	app := fs.String("app-version", "", "")
	pos := parseArgs(fs, []string{"dir", "-o", "registry.json", "--yes", "-app-version", "0.6.0"})
	if len(pos) != 1 || pos[0] != "dir" {
		t.Errorf("pos = %v", pos)
	}
	if *out != "registry.json" || !*yes || *app != "0.6.0" {
		t.Errorf("flags = %q %v %q", *out, *yes, *app)
	}
}
