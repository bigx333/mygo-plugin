package plugin

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// loadFixture loads a plugin folder for a test, with a data dir of its own.
func loadFixture(t *testing.T, dir string, opts Options) *Plugin {
	t.Helper()
	if opts.DataDir == "" {
		opts.DataDir = t.TempDir()
	}
	p, err := Load(dir, opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}

func TestRunCommand(t *testing.T) {
	p := loadFixture(t, "testdata/commands", Options{})
	if err := p.RunCommand("echo", map[string]any{"text": "hi"}); err != nil {
		t.Fatal(err)
	}
	if err := p.RunCommand("echo", nil); err != nil {
		t.Fatalf("with no args: %v", err)
	}
}

func TestRunCommandUnknown(t *testing.T) {
	p := loadFixture(t, "testdata/commands", Options{})
	err := p.RunCommand("nope", nil)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v, want the plugin's ErrUnavailable", err)
	}
	if !strings.Contains(err.Error(), "unknown command: nope") {
		t.Errorf("message lost: %v", err)
	}
}

func TestRunCommandUnsupported(t *testing.T) {
	p := loadFixture(t, "testdata/hello", Options{})
	if err := p.RunCommand("echo", nil); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("err = %v, want ErrUnsupported", err)
	}
}

func TestManifestCommands(t *testing.T) {
	p := loadFixture(t, "testdata/commands", Options{})
	commands := p.Manifest().Commands()
	if len(commands) != 1 {
		t.Fatalf("commands = %+v", commands)
	}
	c := commands[0]
	if c.ID != "echo" || c.Title != "Echo" || c.Description != "Answer with ok" {
		t.Errorf("command = %+v", c)
	}
	hello := loadFixture(t, "testdata/hello", Options{})
	if commands := hello.Manifest().Commands(); len(commands) != 0 {
		t.Errorf("hello has commands: %+v", commands)
	}
}

func TestCapability(t *testing.T) {
	asked := func(p *Plugin, name string) bool {
		t.Helper()
		var offered bool
		if err := p.Call("capability", map[string]string{"name": name}, &offered); err != nil {
			t.Fatal(err)
		}
		return offered
	}

	p := loadFixture(t, "testdata/commands", Options{Capabilities: []string{"network", "clipboard"}})
	if !asked(p, "network") {
		t.Error("network is not offered")
	}
	if asked(p, "camera") {
		t.Error("camera is offered")
	}
	p.Close()

	none := loadFixture(t, "testdata/commands", Options{})
	if asked(none, "network") {
		t.Error("network is offered by a host that offers nothing")
	}
}

// contractPlugin writes a plugin that declares a contract, for the checks at
// load time.
func contractPlugin(t *testing.T, contract string, version int) string {
	t.Helper()
	dir := t.TempDir()
	manifest := map[string]any{
		"id":              "contract",
		"name":            "Contract",
		"version":         "0.1.0",
		"main":            "index.js",
		"permissions":     map[string]any{},
		"contract":        contract,
		"contractVersion": version,
	}
	b, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	source := "export function command() { return { ok: true }; }\n"
	if err := os.WriteFile(filepath.Join(dir, "index.js"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestContractMismatch(t *testing.T) {
	dir := contractPlugin(t, "other", 2)
	_, err := Load(dir, Options{DataDir: t.TempDir(), Contract: "generic", ContractVersion: 2})
	if err == nil {
		t.Fatal("a plugin of another contract loaded")
	}
	want := `plugin Contract implements contract "other", this host provides "generic"`
	if !strings.Contains(err.Error(), want) {
		t.Errorf("err = %v, want %q", err, want)
	}
}

func TestContractNewer(t *testing.T) {
	dir := contractPlugin(t, "other", 2)
	_, err := Load(dir, Options{DataDir: t.TempDir(), Contract: "other", ContractVersion: 1})
	if err == nil {
		t.Fatal("a plugin that needs a newer contract loaded")
	}
	if !strings.Contains(err.Error(), "needs contract \"other\" version 2") {
		t.Errorf("err = %v", err)
	}
}

func TestContractLoads(t *testing.T) {
	dir := contractPlugin(t, "other", 2)
	for _, opts := range []Options{
		{Contract: "other", ContractVersion: 2},
		{}, // a host in development provides no contract
	} {
		opts.DataDir = t.TempDir()
		p, err := Load(dir, opts)
		if err != nil {
			t.Fatalf("contract %q version %d: %v", opts.Contract, opts.ContractVersion, err)
		}
		p.Close()
	}
}

// TestContractKeepsExistingPlugins loads a plugin that declares no contract,
// as MyGo Music's do, on a host that provides one.
func TestContractKeepsExistingPlugins(t *testing.T) {
	hello := loadFixture(t, "testdata/hello", Options{Contract: "generic", ContractVersion: 1})
	var out struct{ Text string }
	if err := hello.Call("hello", map[string]string{"name": "mygo"}, &out); err != nil {
		t.Fatal(err)
	}
	if out.Text != "hello mygo" {
		t.Errorf("got %q", out.Text)
	}
}

func TestContributionValidation(t *testing.T) {
	for _, c := range []struct {
		name    string
		body    string
		wantErr bool
	}{
		{"command", `"contributes":{"commands":[{"id":"echo","title":"Echo"}]}`, false},
		{"command without id", `"contributes":{"commands":[{"title":"Echo"}]}`, true},
		{"command with a bad id", `"contributes":{"commands":[{"id":"not an id"}]}`, true},
	} {
		body := `{"id":"x","main":"index.js","permissions":{},` + c.body + `}`
		_, err := ParseManifest([]byte(body))
		if (err != nil) != c.wantErr {
			t.Errorf("%s: err = %v, want error %v", c.name, err, c.wantErr)
		}
	}
}

func TestSettingValidation(t *testing.T) {
	for _, c := range []struct {
		name    string
		setting string
		wantErr bool
	}{
		{"plain", `{"key":"token","label":"Token"}`, false},
		{"string", `{"key":"token","label":"Token","type":"string"}`, false},
		{"password", `{"key":"token","label":"Token","type":"password"}`, false},
		{"boolean", `{"key":"sync","label":"Sync","type":"boolean","default":"true"}`, false},
		{"boolean unset", `{"key":"sync","label":"Sync","type":"boolean"}`, false},
		{"boolean with a bad default", `{"key":"sync","label":"Sync","type":"boolean","default":"yes"}`, true},
		{"select", `{"key":"mode","label":"Mode","type":"select","options":["a","b"]}`, false},
		{"select without options", `{"key":"mode","label":"Mode","type":"select"}`, true},
		{"unknown type", `{"key":"mode","label":"Mode","type":"number"}`, true},
	} {
		body := `{"id":"x","main":"index.js","permissions":{},"settings":[` + c.setting + `]}`
		_, err := ParseManifest([]byte(body))
		if (err != nil) != c.wantErr {
			t.Errorf("%s: err = %v, want error %v", c.name, err, c.wantErr)
		}
	}
}
