package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MoeForge/moejs"
)

// Inspect reads and compiles a plugin without running it: what plugin.json
// says, and the exports the host can call.
func Inspect(dir, appVersion string) (*Manifest, []string, error) {
	m, err := ReadManifest(dir)
	if err != nil {
		return nil, nil, err
	}
	if m.MinAppVersion != "" && appVersion != "" {
		newer, err := CompareVersions(m.MinAppVersion, appVersion)
		if err != nil {
			return nil, nil, err
		}
		if newer > 0 {
			return nil, nil, &NeedsNewerApp{Plugin: m.Name, Version: m.MinAppVersion}
		}
	}
	source, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(m.Main)))
	if err != nil {
		return nil, nil, err
	}
	compile := moejs.Compile
	if strings.HasSuffix(m.Main, ".ts") {
		compile = moejs.CompileTS
	}
	mod, err := compile(m.Main, string(source))
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", dir, err)
	}
	exports := append([]string(nil), mod.Exports()...)
	sort.Strings(exports)
	return m, exports, nil
}
