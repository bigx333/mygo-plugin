package plugin

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	// MaxArchive is the largest accepted zip.
	MaxArchive = 1 << 30
	// MaxUnpacked is the largest a zip may unpack to.
	MaxUnpacked = 2 << 30
	// MaxFiles is the most files a zip may hold.
	MaxFiles = 50000
)

// ErrInstalled says a plugin with that id is already installed.
var ErrInstalled = errors.New("a plugin with this id is already installed")

// List reads the manifests of the plugins in pluginsDir, leaving out the ones
// that don't read: one broken plugin shouldn't hide the others.
func List(pluginsDir string) ([]*Manifest, error) {
	entries, err := os.ReadDir(pluginsDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []*Manifest
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		m, err := ReadManifest(filepath.Join(pluginsDir, e.Name()))
		if err != nil {
			continue
		}
		out = append(out, m)
	}
	return out, nil
}

// InstallDir copies the plugin in src into pluginsDir, under its id. It
// doesn't replace a plugin that's already there.
func InstallDir(src, pluginsDir string) (*Manifest, error) {
	m, err := ReadManifest(src)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp(pluginsDir, ".install-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	if err := copyTree(src, tmp); err != nil {
		return nil, err
	}
	if err := commit(tmp, filepath.Join(pluginsDir, m.ID)); err != nil {
		return nil, err
	}
	return m, nil
}

// InstallZip unpacks the zip into pluginsDir, under its id. plugin.json is at
// the zip's root, or in its one folder, as GitHub's archives have it. It
// doesn't replace a plugin that's already there.
func InstallZip(path, pluginsDir string) (*Manifest, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > MaxArchive {
		return nil, fmt.Errorf("%s is larger than %d bytes", path, int64(MaxArchive))
	}
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp(pluginsDir, ".install-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	if err := unpack(zr.File, tmp); err != nil {
		return nil, err
	}
	root, err := pluginRoot(tmp)
	if err != nil {
		return nil, err
	}
	m, err := ReadManifest(root)
	if err != nil {
		return nil, err
	}
	if err := commit(root, filepath.Join(pluginsDir, m.ID)); err != nil {
		return nil, err
	}
	return m, nil
}

// Uninstall removes the plugin's folder. Its data stays, so installing it
// again keeps the listener's settings and session.
func Uninstall(pluginsDir, id string) error {
	if id == "" || !filepath.IsLocal(id) || strings.ContainsAny(id, `/\`) {
		return fmt.Errorf("not a plugin id: %q", id)
	}
	return os.RemoveAll(filepath.Join(pluginsDir, id))
}

// Pack writes the plugin in dir as a zip, with plugin.json at its root.
func Pack(dir string, w io.Writer) error {
	if _, err := ReadManifest(dir); err != nil {
		return err
	}
	zw := zip.NewWriter(w)
	defer zw.Close()
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			if skipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		info, err := d.Info()
		if err != nil {
			return err
		}
		h, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		h.Name = filepath.ToSlash(rel)
		h.Method = zip.Deflate
		dst, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		_, err = io.Copy(dst, f)
		return err
	})
}

func skipDir(name string) bool {
	switch name {
	case ".git", ".mygo", "node_modules", "dist", "build":
		return true
	}
	return strings.HasPrefix(name, ".") && name != "."
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			if skipDir(d.Name()) {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		return copyFile(path, filepath.Join(dst, rel))
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// commit moves a checked plugin into place, all at once.
func commit(root, dst string) error {
	if _, err := os.Stat(dst); err == nil {
		return ErrInstalled
	}
	return os.Rename(root, dst)
}

// pluginRoot finds where plugin.json is: at the root or in the one folder
// there, as a zip of a repository has it.
func pluginRoot(dir string) (string, error) {
	if _, err := os.Stat(filepath.Join(dir, "plugin.json")); err == nil {
		return dir, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), "__MACOSX") {
			dirs = append(dirs, e.Name())
		}
	}
	if len(dirs) == 1 {
		root := filepath.Join(dir, dirs[0])
		if _, err := os.Stat(filepath.Join(root, "plugin.json")); err == nil {
			return root, nil
		}
	}
	return "", errors.New("the zip has no plugin.json at its root or in its one folder")
}

func unpack(files []*zip.File, dst string) error {
	if len(files) > MaxFiles {
		return fmt.Errorf("the zip holds more than %d files", MaxFiles)
	}
	var total int64
	for _, f := range files {
		name := f.Name
		// GitHub's zips put everything under one folder; that's kept.
		if !filepath.IsLocal(filepath.FromSlash(name)) {
			return fmt.Errorf("the zip has a path outside its folder: %s", name)
		}
		target := filepath.Join(dst, filepath.FromSlash(name))
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if !f.Mode().IsRegular() {
			continue
		}
		total += int64(f.UncompressedSize64)
		if total > MaxUnpacked {
			return fmt.Errorf("the zip unpacks to more than %d bytes", int64(MaxUnpacked))
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		if err != nil {
			in.Close()
			return err
		}
		_, err = io.Copy(out, io.LimitReader(in, MaxUnpacked))
		in.Close()
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return err
		}
	}
	return nil
}
