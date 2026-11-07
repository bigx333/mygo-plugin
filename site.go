package plugin

import (
	"fmt"
	"html/template"
	"io"
	"strings"
)

// WriteSite writes the registry as a static HTML page: one card per plugin,
// with its permissions, health, trust signals and install command. It needs
// no server and no JavaScript.
func WriteSite(w io.Writer, reg *Registry) error {
	type entry struct {
		RegistryEntry
		Network  string
		Process  string
		Health   string
		HealthOK bool
		Pushed   string
		Install  string
	}
	page := struct {
		Generated string
		Entries   []entry
	}{
		Generated: reg.GeneratedAt.Format("2006-01-02 15:04 MST"),
	}
	for _, e := range reg.Entries {
		it := entry{RegistryEntry: e}
		it.Network = networkText(e.Permissions.Network)
		if len(e.Permissions.Process) > 0 {
			it.Process = "may start programs"
		} else {
			it.Process = "no programs"
		}
		switch {
		case e.Health == nil:
			it.Health = "not checked"
		case e.Health.Status == "ok":
			it.Health = fmt.Sprintf("compiles · %d exports", len(e.Health.Exports))
			it.HealthOK = true
		default:
			it.Health = "does not compile"
		}
		if e.PushedAt != nil {
			it.Pushed = e.PushedAt.Format("2006-01-02")
		}
		it.Install = "mygo-plugin install " + e.ID + " ./plugins"
		if len(e.Permissions.Network) > 0 {
			it.Install += " --allow-network"
		}
		if len(e.Permissions.Process) > 0 {
			it.Install += " --allow-process"
		}
		page.Entries = append(page.Entries, it)
	}
	return siteTemplate.Execute(w, page)
}

func networkText(hosts []string) string {
	switch {
	case len(hosts) == 0:
		return "no network"
	case len(hosts) == 1 && hosts[0] == "*":
		return "any host"
	case len(hosts) > 2:
		return fmt.Sprintf("%s +%d", strings.Join(hosts[:2], ", "), len(hosts)-2)
	}
	return strings.Join(hosts, ", ")
}

var siteTemplate = template.Must(template.New("site").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>MyGo plugins</title>
<style>
  :root { color-scheme: light dark; --bg: #f6f6f4; --panel: #fff; --text: #17171a; --muted: #6b6b73; --line: #e3e3df; --accent: #2f6fed; --ok: #1f7a4d; --bad: #a33a2a; }
  @media (prefers-color-scheme: dark) { :root { --bg: #131316; --panel: #1b1b1f; --text: #ececf0; --muted: #9a9aa3; --line: #2a2a30; --accent: #7aa2ff; --ok: #5fce9b; --bad: #ff8b78; } }
  * { box-sizing: border-box; }
  body { margin: 0; background: var(--bg); color: var(--text); font: 14px/1.5 -apple-system, BlinkMacSystemFont, "Segoe UI", system-ui, sans-serif; }
  header, main, footer { max-width: 1100px; margin: 0 auto; padding: 0 20px; }
  header { padding-top: 40px; padding-bottom: 20px; }
  h1 { font-size: 26px; margin: 0 0 6px; letter-spacing: -0.02em; }
  header p, footer { color: var(--muted); font-size: 12px; }
  main { display: grid; grid-template-columns: repeat(auto-fill, minmax(min(100%, 320px), 1fr)); gap: 14px; padding-bottom: 30px; }
  .card { background: var(--panel); border: 1px solid var(--line); border-radius: 12px; padding: 16px; display: flex; flex-direction: column; gap: 8px; }
  .top { display: flex; align-items: baseline; gap: 8px; flex-wrap: wrap; }
  h2 { font-size: 15px; margin: 0; }
  .ver { color: var(--muted); font-size: 12px; font-variant-numeric: tabular-nums; }
  .desc { margin: 0; color: var(--muted); font-size: 12px; min-height: 36px; }
  .chips { display: flex; gap: 6px; flex-wrap: wrap; }
  .chip { border: 1px solid var(--line); border-radius: 999px; padding: 2px 8px; font-size: 11px; color: var(--muted); }
  .chip.ok { color: var(--ok); border-color: color-mix(in srgb, var(--ok) 40%, transparent); }
  .chip.bad { color: var(--bad); border-color: color-mix(in srgb, var(--bad) 40%, transparent); }
  code { font: 11px/1.6 ui-monospace, SFMono-Regular, Menlo, monospace; background: color-mix(in srgb, var(--text) 6%, transparent); padding: 6px 8px; border-radius: 6px; overflow-wrap: anywhere; }
  .foot { display: flex; justify-content: space-between; gap: 10px; color: var(--muted); font-size: 11px; margin-top: auto; }
  .foot a { color: var(--accent); text-decoration: none; }
  footer { padding-bottom: 40px; }
</style>
</head>
<body>
<header>
  <h1>MyGo plugins</h1>
  <p>{{len .Entries}} plugins · generated {{.Generated}} · from plugin.json manifests, checked by compiling the module</p>
</header>
<main>
{{range .Entries}}
  <article class="card">
    <div class="top"><h2>{{.Name}}</h2><span class="ver">v{{.Version}}</span>{{if .Archived}}<span class="chip bad">archived</span>{{end}}</div>
    <p class="desc">{{.Description}}</p>
    <div class="chips">
      <span class="chip">{{.Network}}</span>
      <span class="chip">{{.Process}}</span>
      <span class="chip {{if .HealthOK}}ok{{else}}bad{{end}}">{{.Health}}</span>
      {{if .License}}<span class="chip">{{.License}}</span>{{end}}
      {{if .Stars}}<span class="chip">★ {{.Stars}}</span>{{end}}
    </div>
    <code>{{.Install}}</code>
    <div class="foot"><a href="https://github.com/{{.Repo}}">{{.Repo}}</a><span>{{if .Pushed}}updated {{.Pushed}}{{end}}</span></div>
  </article>
{{end}}
</main>
<footer>Generated by <code>mygo-plugin site</code> from registry.json. Installing runs third-party code; read the permissions first.</footer>
</body>
</html>
`))
