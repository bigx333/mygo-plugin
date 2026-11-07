package plugin

import (
	"bytes"
	"crypto/aes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/MoeForge/moejs"
)

type native = func(*moejs.Realm, moejs.Value, []moejs.Value) (moejs.Value, error)

// installHost gives the runtime the host global and console. What it doesn't
// put there, a plugin can't reach.
func (p *Plugin) installHost(v *vm) error {
	rt := v.rt
	logf := func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = a.String()
		}
		p.opts.Host.Logf("plugin %s: %s", p.manifest.ID, strings.Join(parts, " "))
		return moejs.Undefined(), nil
	}
	if err := rt.SetGlobal("console", map[string]any{
		"log":   moejs.NativeFunc(logf),
		"info":  moejs.NativeFunc(logf),
		"warn":  moejs.NativeFunc(logf),
		"error": moejs.NativeFunc(logf),
		"debug": moejs.NativeFunc(logf),
	}); err != nil {
		return err
	}
	osName, arch := platform()
	return rt.SetGlobal("host", map[string]any{
		"platform": osName,
		"arch":     arch,
		"dir":      p.dir,
		"dataDir":  p.data,
		"storage": map[string]any{
			"get":    moejs.NativeFunc(p.storageGet(v)),
			"set":    moejs.NativeFunc(p.storageSet(v)),
			"remove": moejs.NativeFunc(p.storageRemove),
		},
		"language": moejs.NativeFunc(func(*moejs.Realm, moejs.Value, []moejs.Value) (moejs.Value, error) {
			lang := "en"
			if p.opts.Host.Language != nil {
				lang = p.opts.Host.Language()
			}
			return moejs.String(lang), nil
		}),
		"t": moejs.NativeFunc(func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			return moejs.String(p.t(moejs.Arg(args, 0).String())), nil
		}),
		"setting": moejs.NativeFunc(func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			if p.opts.Host.Setting == nil {
				return moejs.String(""), nil
			}
			return moejs.String(p.opts.Host.Setting(p.manifest.ID, moejs.Arg(args, 0).String())), nil
		}),
		"openURL": moejs.NativeFunc(func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			address := moejs.Arg(args, 0).String()
			u, err := url.Parse(address)
			if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
				return moejs.Undefined(), fmt.Errorf("not a web address: %s", address)
			}
			if p.opts.Host.OpenURL == nil {
				return moejs.Undefined(), errors.New("this app doesn't open pages")
			}
			return moejs.Undefined(), p.opts.Host.OpenURL(address)
		}),
		"signInPrompt": moejs.NativeFunc(func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			var prompt SignInPrompt
			if err := rt.ToGoInto(moejs.Arg(args, 0), &prompt); err != nil {
				return moejs.Undefined(), err
			}
			if p.opts.Host.SignInPrompt != nil {
				p.opts.Host.SignInPrompt(p.manifest.ID, prompt)
			}
			return moejs.Undefined(), nil
		}),
		"crypto": map[string]any{
			"md5": moejs.NativeFunc(func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
				sum := md5.Sum([]byte(moejs.Arg(args, 0).String()))
				return moejs.String(hex.EncodeToString(sum[:])), nil
			}),
			"aesEcbEncrypt": moejs.NativeFunc(func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
				out, err := aesECB([]byte(moejs.Arg(args, 0).String()), []byte(moejs.Arg(args, 1).String()))
				if err != nil {
					return moejs.Undefined(), err
				}
				return moejs.String(hex.EncodeToString(out)), nil
			}),
		},
		"removeDataDir": moejs.NativeFunc(func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			name := moejs.Arg(args, 0).String()
			if !filepath.IsLocal(name) || filepath.Clean(name) == "." {
				return moejs.Undefined(), fmt.Errorf("not a folder of the plugin's data: %s", name)
			}
			return moejs.Undefined(), os.RemoveAll(filepath.Join(p.data, name))
		}),
		// This host starts no programs. A plugin that asks is told so, rather
		// than left to think it failed.
		"spawn": moejs.NativeFunc(func(*moejs.Realm, moejs.Value, []moejs.Value) (moejs.Value, error) {
			return moejs.Undefined(), errors.New("this app doesn't start programs for plugins")
		}),
	})
}

func (p *Plugin) storageGet(v *vm) native {
	return func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
		key := moejs.Arg(args, 0).String()
		p.smu.Lock()
		raw, ok := p.storage[key]
		p.smu.Unlock()
		if !ok {
			return moejs.Undefined(), nil
		}
		return v.rt.ParseJSON(raw)
	}
}

func (p *Plugin) storageSet(v *vm) native {
	return func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
		raw, err := v.rt.AppendJSON(nil, moejs.Arg(args, 1))
		if err != nil {
			return moejs.Undefined(), err
		}
		key := moejs.Arg(args, 0).String()
		p.smu.Lock()
		p.storage[key] = raw
		err = p.writeJSON("storage.json", p.storage)
		p.smu.Unlock()
		return moejs.Undefined(), err
	}
}

func (p *Plugin) storageRemove(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
	key := moejs.Arg(args, 0).String()
	p.smu.Lock()
	defer p.smu.Unlock()
	delete(p.storage, key)
	return moejs.Undefined(), p.writeJSON("storage.json", p.storage)
}

// aesECB encrypts text with PKCS#7 padding, as the ECB its callers expect.
func aesECB(key, text []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	pad := aes.BlockSize - len(text)%aes.BlockSize
	text = append(append([]byte(nil), text...), bytes.Repeat([]byte{byte(pad)}, pad)...)
	out := make([]byte, len(text))
	for i := 0; i < len(text); i += aes.BlockSize {
		block.Encrypt(out[i:i+aes.BlockSize], text[i:i+aes.BlockSize])
	}
	return out, nil
}
