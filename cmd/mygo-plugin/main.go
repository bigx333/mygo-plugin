// Command mygo-plugin checks, packs and starts plugins for MyGo apps.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	plugin "github.com/yxxbc/mygo-plugin"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "check":
		check(os.Args[2:])
	case "pack":
		pack(os.Args[2:])
	case "new":
		create(os.Args[2:])
	case "list":
		list(os.Args[2:])
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "mygo-plugin: no such command %q\n", os.Args[1])
		usage()
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `mygo-plugin reads the plugin.json MyGo Music plugins use.

  mygo-plugin check [dir]        check the manifest and compile the module
  mygo-plugin pack [dir] [-o f]  write a zip with plugin.json at its root
  mygo-plugin new <id>           start a plugin in a new folder
  mygo-plugin list <plugins>     list the plugins installed in a folder

check takes -app-version to enforce minAppVersion.
`)
	os.Exit(2)
}

func check(args []string) {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	app := fs.String("app-version", "", "the host version minAppVersion is checked against")
	fs.Parse(args)
	dir := "."
	if fs.NArg() > 0 {
		dir = fs.Arg(0)
	}
	m, exports, err := plugin.Inspect(dir, *app)
	if err != nil {
		fail(err)
	}
	fmt.Printf("%s %s (%s)\n", m.Name, m.Version, m.ID)
	fmt.Printf("main: %s\n", m.Main)
	if m.MinAppVersion != "" {
		fmt.Printf("needs app: %s or later\n", m.MinAppVersion)
	}
	if len(m.Permissions.Network) > 0 {
		fmt.Printf("network: %s\n", strings.Join(m.Permissions.Network, ", "))
	}
	if len(m.Permissions.Process) > 0 {
		fmt.Printf("process: %s\n", strings.Join(m.Permissions.Process, ", "))
	}
	for _, n := range m.Navigation {
		what := n.Page
		if n.Builtin != "" {
			what = "builtin:" + n.Builtin
		}
		fmt.Printf("navigation: %s (%s)\n", n.Title, what)
	}
	fmt.Printf("exports: %s\n", strings.Join(exports, ", "))
	if len(exports) == 0 {
		fail(fmt.Errorf("%s exports nothing", m.Main))
	}
}

func pack(args []string) {
	fs := flag.NewFlagSet("pack", flag.ExitOnError)
	out := fs.String("o", "", "the zip to write")
	fs.Parse(args)
	dir := "."
	if fs.NArg() > 0 {
		dir = fs.Arg(0)
	}
	m, _, err := plugin.Inspect(dir, "")
	if err != nil {
		fail(err)
	}
	if *out == "" {
		*out = fmt.Sprintf("%s-%s.zip", m.ID, m.Version)
	}
	f, err := os.Create(*out)
	if err != nil {
		fail(err)
	}
	if err := plugin.Pack(dir, f); err != nil {
		f.Close()
		fail(err)
	}
	if err := f.Close(); err != nil {
		fail(err)
	}
	fmt.Println(*out)
}

func create(args []string) {
	if len(args) != 1 {
		usage()
	}
	id := args[0]
	m := &plugin.Manifest{ID: id, Name: id, Version: "0.1.0", Main: "index.js", Navigation: []plugin.Navigation{
		{Title: "Hello", Page: "hello", Icon: "star"},
	}}
	if err := m.Validate(); err != nil {
		fail(err)
	}
	dir := id
	if _, err := os.Stat(dir); err == nil {
		fail(fmt.Errorf("%s exists", dir))
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fail(err)
	}
	manifest := fmt.Sprintf(`{
  "id": %q,
  "name": %q,
  "version": "0.1.0",
  "main": "index.js",
  "permissions": {},
  "navigation": [
    {"title": "Hello", "page": "hello", "icon": "star"}
  ]
}
`, id, id)
	files := map[string]string{
		"plugin.json": manifest,
		"index.js": `// The host calls exports by the names of what it needs.
export function hello() {
  return { title: "Hello from " + host.platform };
}

export function browse({ page }) {
  return { title: "Hello", sections: [{ tracks: [] }] };
}
`,
		"README.md": "# " + id + "\n\nA MyGo plugin. Run `mygo-plugin check .` and `mygo-plugin pack .`.\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			fail(err)
		}
	}
	fmt.Println(dir)
}

func list(args []string) {
	if len(args) != 1 {
		usage()
	}
	manifests, err := plugin.List(args[0])
	if err != nil {
		fail(err)
	}
	sort.Slice(manifests, func(i, j int) bool { return manifests[i].ID < manifests[j].ID })
	for _, m := range manifests {
		fmt.Printf("%s\t%s\t%s\n", m.ID, m.Version, m.Name)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "mygo-plugin:", err)
	os.Exit(1)
}
