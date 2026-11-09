package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MoeForge/moejs"
	"github.com/MoeForge/moejs/eventloop"
	"github.com/MoeForge/moejs/webapi"
)

const (
	// DefaultMemoryLimit is what one plugin may keep, in bytes.
	DefaultMemoryLimit = 256 << 20
	// DefaultCallTimeout is how long one call may take.
	DefaultCallTimeout = 30 * time.Second
	// stuckAfter is how long a runtime has to take on a task before it counts
	// as stuck in plugin code rather than slow about something.
	stuckAfter = time.Second
)

// Host is what a plugin reaches of the app. The zero value reaches nothing
// and logs to the standard logger.
type Host struct {
	// Language is the listener's language, as a BCP 47 tag: zh-CN, en.
	Language func() string
	// OpenURL opens a web page in the listener's browser.
	OpenURL func(url string) error
	// SignInPrompt shows what a plugin wants shown as it signs in.
	SignInPrompt func(pluginID string, prompt SignInPrompt)
	// Setting is the value the listener gave a setting of the plugin.
	Setting func(pluginID, key string) string
	// Logf receives what a plugin logs. Nil logs to the standard logger.
	Logf func(format string, args ...any)
}

// SignInPrompt is what a plugin wants shown while the listener signs in.
type SignInPrompt struct {
	QR   string `json:"qr,omitempty"`
	Text string `json:"text,omitempty"`
}

// Options say where a plugin keeps its data and what it may reach.
type Options struct {
	// AppVersion is the host's version, which minAppVersion is checked
	// against. Empty loads any plugin, as in development.
	AppVersion string
	// DataDir is where the host keeps the plugins' data, one folder per
	// plugin. It is required, and should belong to the app, so that two
	// apps with the same plugin don't share its storage.
	DataDir string
	// Host is what the app offers the plugin.
	Host Host
	// MemoryLimit is what one runtime may keep. Zero means 256 MiB.
	MemoryLimit int64
	// CallTimeout is how long one call may take. Zero means 30 seconds.
	CallTimeout time.Duration
	// HTTPClient sends fetch's requests. Nil means an http.Client with a
	// ten minute timeout.
	HTTPClient *http.Client
	// Allowed, when set, decides which network hosts a plugin may reach.
	// The manifest still lists what it asks for; this is the listener's
	// answer, host by host. Nil allows every host the manifest lists.
	Allowed func(pluginID, host string) bool
	// Contract is the app contract this host provides, such as
	// "mygo-music". Empty loads any plugin, as in development.
	Contract string
	// ContractVersion is the version of that contract the host implements.
	ContractVersion int
	// Capabilities are what this host offers plugins, which
	// host.capability answers: "network", "clipboard". A capability absent
	// here is answered false, and the host decides whether to load the
	// plugin at all.
	Capabilities []string
	// Functions are the host's own async functions, which a plugin calls
	// with host.call(name, args) and awaits. Each call runs on a goroutine of
	// its own, never the plugin's loop.
	Functions map[string]Function
	// Prelude is a classic script run in each runtime after the host global
	// is installed and before the module loads, so the host can define helper
	// globals. Empty runs none.
	Prelude string
	// LoadTimeout is how long the prelude and the module's top level may run
	// as a runtime starts. Zero means CallTimeout.
	LoadTimeout time.Duration
	// Main, when set, is the module to load in place of the manifest's
	// main, relative to the plugin's folder: a host runs a plugin's second
	// module (a headless service) as a runtime of its own.
	Main string
}

// Function is a host function a plugin calls through host.call. args is the
// JSON of the value the plugin passed, null when it passed none. ctx ends
// when the plugin's runtime closes. The result is sent back as JSON; an
// error rejects the plugin's promise with its text, and with its code when
// it has a Code() string method.
type Function func(ctx context.Context, args json.RawMessage) (any, error)

func (o *Options) defaults() {
	if o.MemoryLimit <= 0 {
		o.MemoryLimit = DefaultMemoryLimit
	}
	if o.CallTimeout <= 0 {
		o.CallTimeout = DefaultCallTimeout
	}
	if o.LoadTimeout <= 0 {
		o.LoadTimeout = o.CallTimeout
	}
	if o.HTTPClient == nil {
		o.HTTPClient = &http.Client{Timeout: 10 * time.Minute}
	}
	if o.Host.Logf == nil {
		o.Host.Logf = log.Printf
	}
}

// Plugin is one plugin, compiled and ready to run. It starts on the first
// call, in a runtime of its own, and starts over after it gets stuck.
type Plugin struct {
	manifest Manifest
	dir      string
	data     string
	mod      *moejs.Module
	prelude  *moejs.Script
	exports  map[string]bool
	locales  map[string]map[string]string
	opts     Options

	mu      sync.Mutex // guards vm
	vm      *vm
	loading atomic.Pointer[vm] // the runtime start is loading, for Close to end
	smu     sync.Mutex         // guards storage, which module evaluation may read
	storage map[string]json.RawMessage
}

// Load reads and compiles the plugin in dir, without running it.
func Load(dir string, opts Options) (*Plugin, error) {
	opts.defaults()
	if opts.DataDir == "" {
		return nil, errors.New("plugin: Options.DataDir is required")
	}
	m, err := ReadManifest(dir)
	if err != nil {
		return nil, err
	}
	if err := checkContract(m, opts); err != nil {
		return nil, err
	}
	if m.MinAppVersion != "" && opts.AppVersion != "" {
		newer, err := CompareVersions(m.MinAppVersion, opts.AppVersion)
		if err != nil {
			return nil, err
		}
		if newer > 0 {
			return nil, &NeedsNewerApp{Plugin: m.Name, Version: m.MinAppVersion}
		}
	}
	name := m.Main
	if opts.Main != "" {
		if !filepath.IsLocal(filepath.FromSlash(opts.Main)) {
			return nil, fmt.Errorf("plugin: Options.Main %q is not a file in the plugin's folder", opts.Main)
		}
		name = opts.Main
	}
	main := filepath.Join(dir, filepath.FromSlash(name))
	source, err := os.ReadFile(main)
	if err != nil {
		return nil, err
	}
	compile := moejs.Compile
	if strings.HasSuffix(name, ".ts") {
		compile = moejs.CompileTS
	}
	mod, err := compile(name, string(source))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", dir, err)
	}
	var prelude *moejs.Script
	if opts.Prelude != "" {
		if prelude, err = moejs.CompileScript("prelude.js", opts.Prelude); err != nil {
			return nil, fmt.Errorf("plugin: Options.Prelude: %w", err)
		}
	}
	p := &Plugin{
		manifest: *m,
		dir:      dir,
		data:     filepath.Join(opts.DataDir, m.ID),
		mod:      mod,
		prelude:  prelude,
		exports:  make(map[string]bool),
		opts:     opts,
		storage:  make(map[string]json.RawMessage),
	}
	for _, name := range mod.Exports() {
		p.exports[name] = true
	}
	p.locales = readLocales(dir)
	readJSON(filepath.Join(p.data, "storage.json"), &p.storage)
	return p, nil
}

// checkContract says why a plugin doesn't fit this host's contract, or nil. A
// host that leaves Contract empty, as in development, loads any plugin.
func checkContract(m *Manifest, opts Options) error {
	if m.Contract == "" || opts.Contract == "" {
		return nil
	}
	if m.Contract != opts.Contract {
		return fmt.Errorf("plugin %s implements contract %q, this host provides %q", m.Name, m.Contract, opts.Contract)
	}
	if m.ContractVersion > opts.ContractVersion {
		return fmt.Errorf("plugin %s needs contract %q version %d, this host provides version %d", m.Name, m.Contract, m.ContractVersion, opts.ContractVersion)
	}
	return nil
}

// Discover loads every plugin folder in dir. One that doesn't load is logged
// and left out, so it can't take the others with it.
func Discover(dir string, opts Options) []*Plugin {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	opts.defaults()
	var plugins []*Plugin
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		p, err := Load(filepath.Join(dir, e.Name()), opts)
		if err != nil {
			opts.Host.Logf("plugin: %v", err)
			continue
		}
		plugins = append(plugins, p)
	}
	return plugins
}

// Manifest is what plugin.json says.
func (p *Plugin) Manifest() Manifest { return p.manifest }

// ID is the plugin's identifier.
func (p *Plugin) ID() string { return p.manifest.ID }

// Dir is the plugin's folder.
func (p *Plugin) Dir() string { return p.dir }

// DataDir is where the plugin keeps its data.
func (p *Plugin) DataDir() string { return p.data }

// Exports lists the functions the module exports, without running it.
func (p *Plugin) Exports() []string {
	out := make([]string, 0, len(p.exports))
	for name := range p.exports {
		out = append(out, name)
	}
	return out
}

// Can reports whether the module exports the function.
func (p *Plugin) Can(method string) bool { return p.exports[method] }

// Call calls an export with args and decodes its answer into out, unless out
// is nil. An export the module doesn't have is ErrUnsupported.
func (p *Plugin) Call(method string, args, out any) error {
	return p.CallWithin(p.opts.CallTimeout, method, args, out)
}

// CallWithin is Call with a timeout of its own.
func (p *Plugin) CallWithin(timeout time.Duration, method string, args, out any) error {
	if !p.exports[method] {
		return ErrUnsupported
	}
	v, err := p.running()
	if err != nil {
		return err
	}
	var encoded []byte
	if args != nil {
		encoded, err = json.Marshal(args)
		if err != nil {
			return err
		}
	}
	if timeout <= 0 {
		timeout = p.opts.CallTimeout
	}
	ch := make(chan answer, 1)
	if !v.loop.RunOnLoop(func(*moejs.Runtime) {
		v.ask(method, encoded, func(a answer) { ch <- a })
	}) {
		p.drop(v)
		return errors.New("the plugin stopped; try again")
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case a := <-ch:
		if a.err != nil {
			return a.err
		}
		if out == nil {
			return nil
		}
		return json.Unmarshal(a.json, out)
	case <-timer.C:
		p.dropIfStuck(v)
		return fmt.Errorf("the plugin did not answer %s within %s", method, timeout)
	}
}

// Close stops the plugin, and a start in progress. It starts again on the next
// call.
func (p *Plugin) Close() {
	if v := p.loading.Load(); v != nil {
		v.close()
	}
	p.mu.Lock()
	v := p.vm
	p.vm = nil
	p.mu.Unlock()
	if v != nil {
		v.close()
	}
}

// vm is one running plugin. Only its loop's goroutine touches the runtime.
type vm struct {
	rt      *moejs.Runtime
	loop    *eventloop.Loop
	api     *webapi.API
	helper  moejs.Value
	hostErr moejs.Value
	ctx     context.Context // ends with the runtime, for host functions
	cancel  context.CancelFunc
	once    sync.Once
}

func (v *vm) close() {
	v.once.Do(func() {
		v.cancel()
		v.api.Close()
		v.loop.Terminate()
	})
}

// running is the plugin's runtime, started if it isn't.
func (p *Plugin) running() (*vm, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.vm != nil {
		return p.vm, nil
	}
	v, err := p.start()
	if err != nil {
		return nil, fmt.Errorf("starting plugin %s: %w", p.manifest.ID, err)
	}
	p.vm = v
	return v, nil
}

func (p *Plugin) start() (*vm, error) {
	if err := os.MkdirAll(p.data, 0o700); err != nil {
		return nil, err
	}
	rt := moejs.NewRuntime(moejs.Options{MemoryLimit: p.opts.MemoryLimit})
	loop, err := eventloop.New(rt, eventloop.Options{
		OnError: func(err error) { p.opts.Host.Logf("plugin %s: %v", p.manifest.ID, err) },
	})
	if err != nil {
		return nil, err
	}
	api, err := webapi.Install(loop, p.webOptions())
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	v := &vm{rt: rt, loop: loop, api: api, ctx: ctx, cancel: cancel}
	p.loading.Store(v)
	defer p.loading.Store(nil)
	if err := p.load(v); err != nil {
		v.close()
		return nil, err
	}
	return v, nil
}

// load readies a new runtime and starts its loop. The prelude and the module's
// top level run on the caller's goroutine, under p.mu, so they are interrupted
// once they run past LoadTimeout.
func (p *Plugin) load(v *vm) error {
	if err := p.installHost(v); err != nil {
		return err
	}
	var err error
	if v.helper, err = v.rt.RunScript(helperScript); err != nil {
		return err
	}
	if v.hostErr, err = v.rt.RunScript(hostErrorScript); err != nil {
		return err
	}
	late := time.AfterFunc(p.opts.LoadTimeout, func() { v.rt.Interrupt(errLoadTimeout) })
	err = p.evaluate(v.rt)
	if !late.Stop() {
		return fmt.Errorf("the prelude and the module's top level did not finish within %s", p.opts.LoadTimeout)
	}
	if errors.Is(err, eventloop.ErrTerminated) {
		return errors.New("the plugin was closed while it loaded")
	}
	if err != nil {
		return err
	}
	return v.loop.Start()
}

var errLoadTimeout = errors.New("the plugin did not load in time")

// evaluate runs the prelude, then the module's top level.
func (p *Plugin) evaluate(rt *moejs.Runtime) error {
	if p.prelude != nil {
		if _, err := rt.RunScript(p.prelude); err != nil {
			return fmt.Errorf("prelude: %w", err)
		}
	}
	if err := rt.Load(p.mod); err != nil && !errors.Is(err, moejs.ErrModulePending) {
		return err
	}
	return nil
}

// webOptions are what fetch may reach: nothing without the network
// permission, and only the hosts it lists. The caller holds p.mu.
func (p *Plugin) webOptions() webapi.Options {
	var hosts []string
	for _, h := range p.manifest.Permissions.Network {
		if p.opts.Allowed != nil && !p.opts.Allowed(p.manifest.ID, h) {
			continue
		}
		if h == "*" {
			return webapi.Options{Client: p.opts.HTTPClient}
		}
		hosts = append(hosts, h)
	}
	if len(hosts) == 0 {
		return webapi.Options{}
	}
	return webapi.Options{Client: p.opts.HTTPClient, Allow: webapi.AllowHosts(hosts...)}
}

// helperBody calls an export and answers through ok or bad, whether it returns
// a value or a promise. A thrown error's code travels on bad.
const helperBody = `(async () => {
  try {
    const value = await fn(arg);
    ok(value === undefined ? null : value);
  } catch (e) {
    const isObject = e !== null && typeof e === "object";
    bad(isObject && e.code ? String(e.code) : "", isObject ? String(e.message || e) : String(e));
  }
})();`

var helperScript = func() *moejs.Script {
	s, err := moejs.CompileFunction("mygo-call", "fn, arg, ok, bad", helperBody, moejs.FunctionOptions{})
	if err != nil {
		panic(err)
	}
	return s
}()

type answer struct {
	json []byte
	err  error
}

// ask calls an export on the loop; the answer comes once, through done.
func (v *vm) ask(method string, params []byte, done func(answer)) {
	rt := v.rt
	fn, ok := rt.Export(method)
	if !ok {
		done(answer{err: ErrUnsupported})
		return
	}
	arg := moejs.Undefined()
	if params != nil {
		var err error
		if arg, err = rt.ParseJSON(params); err != nil {
			done(answer{err: err})
			return
		}
	}
	okFn := rt.Function("ok", 1, func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
		out, err := rt.AppendJSON(nil, moejs.Arg(args, 0))
		done(answer{json: out, err: err})
		return moejs.Undefined(), nil
	})
	badFn := rt.Function("bad", 2, func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
		done(answer{err: thrownError(moejs.Arg(args, 0).String(), moejs.Arg(args, 1).String())})
		return moejs.Undefined(), nil
	})
	// Only an interrupt of a stuck runtime gets an error here; what the
	// plugin throws is caught by the helper.
	if _, err := rt.CallFunction(v.helper, moejs.Undefined(), fn, arg, okFn, badFn); err != nil {
		done(answer{err: err})
	}
}

// dropIfStuck ends a runtime that takes on no more tasks, as code that never
// returns leaves it. One that is only slow, waiting on something, goes on.
func (p *Plugin) dropIfStuck(v *vm) {
	alive := make(chan struct{})
	v.loop.RunOnLoop(func(*moejs.Runtime) { close(alive) })
	select {
	case <-alive:
	case <-time.After(stuckAfter):
		p.opts.Host.Logf("plugin %s: stuck; restarting it", p.manifest.ID)
		p.drop(v)
	}
}

func (p *Plugin) drop(v *vm) {
	p.mu.Lock()
	if p.vm == v {
		p.vm = nil
	}
	p.mu.Unlock()
	go v.close()
}

// readLocales reads the plugin's translations: locales/<language>.json, each
// a map from the English text to what it is in that language.
func readLocales(dir string) map[string]map[string]string {
	out := map[string]map[string]string{}
	entries, err := os.ReadDir(filepath.Join(dir, "locales"))
	if err != nil {
		return out
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		var texts map[string]string
		b, err := os.ReadFile(filepath.Join(dir, "locales", e.Name()))
		if err != nil || json.Unmarshal(b, &texts) != nil {
			continue
		}
		out[strings.TrimSuffix(e.Name(), ".json")] = texts
	}
	return out
}

// t is text in the listener's language, if the plugin translated it. A
// language without a translation falls back to its base language, as zh-CN
// does to zh, and then to the English text.
func (p *Plugin) t(text string) string {
	lang := "en"
	if p.opts.Host.Language != nil {
		lang = p.opts.Host.Language()
	}
	if tr := p.locales[lang][text]; tr != "" {
		return tr
	}
	if i := strings.IndexByte(lang, '-'); i > 0 {
		if tr := p.locales[lang[:i]][text]; tr != "" {
			return tr
		}
	}
	return text
}

// readJSON reads the file into v, leaving v as it is when the file is absent
// or unreadable: a lost setting is better than a plugin that won't start.
func readJSON(path string, into any) {
	if b, err := os.ReadFile(path); err == nil {
		json.Unmarshal(b, into)
	}
}

// writeJSON replaces the file whole, so that a crash leaves the last one.
func (p *Plugin) writeJSON(name string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(p.data, 0o700); err != nil {
		return err
	}
	path := filepath.Join(p.data, name)
	if err := os.WriteFile(path+".part", b, 0o600); err != nil {
		return err
	}
	return os.Rename(path+".part", path)
}

// platform is the host's operating system and architecture, for a plugin that
// has binaries of its own.
func platform() (string, string) { return runtime.GOOS, runtime.GOARCH }
