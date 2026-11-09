package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/MoeForge/moejs"
)

// maxCallArgs is the most JSON a plugin may pass one host function.
const maxCallArgs = 16 << 20

// hostCall is host.call(name, args): a promise of what the host function
// answers. The function runs on a goroutine of its own, and its answer is
// settled on the loop.
func (p *Plugin) hostCall(v *vm) native {
	return func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
		promise, settle := v.loop.NewPromise()
		name := moejs.Arg(args, 0).String()
		fn := p.opts.Functions[name]
		if fn == nil {
			settle(v.reject(fmt.Errorf("no host function %q", name)))
			return promise, nil
		}
		raw, err := v.rt.AppendJSON(nil, moejs.Arg(args, 1))
		if err == nil && len(raw) > maxCallArgs {
			err = fmt.Errorf("the arguments of %s are over %d bytes", name, maxCallArgs)
		}
		if err != nil {
			settle(v.reject(err))
			return promise, nil
		}
		go func() {
			out, err := fn(v.ctx, raw)
			var answer []byte
			if err == nil {
				answer, err = json.Marshal(out)
			}
			if err != nil {
				settle(v.reject(err))
				return
			}
			settle(func(rt *moejs.Runtime) (moejs.Value, error) { return rt.ParseJSON(answer) })
		}()
		return promise, nil
	}
}

// reject settles a promise with an Error of err's text, and its code when err
// has one.
func (v *vm) reject(err error) func(*moejs.Runtime) (moejs.Value, error) {
	return func(rt *moejs.Runtime) (moejs.Value, error) {
		var coded interface{ Code() string }
		code := ""
		if errors.As(err, &coded) {
			code = coded.Code()
		}
		e, ferr := rt.CallFunction(v.hostErr, moejs.Undefined(), moejs.String(err.Error()), moejs.String(code))
		if ferr != nil {
			return moejs.Undefined(), ferr
		}
		return moejs.Undefined(), &moejs.Exception{Value: e}
	}
}

// hostFunctions is host.functions(): the names host.call takes, sorted.
func (p *Plugin) hostFunctions(v *vm) native {
	return func(*moejs.Realm, moejs.Value, []moejs.Value) (moejs.Value, error) {
		names := make([]string, 0, len(p.opts.Functions))
		for name := range p.opts.Functions {
			names = append(names, name)
		}
		sort.Strings(names)
		return v.rt.FromGo(names)
	}
}

var hostErrorScript = func() *moejs.Script {
	s, err := moejs.CompileFunction("mygo-host-error", "message, code", `const e = new Error(message);
if (code) e.code = code;
return e;`, moejs.FunctionOptions{})
	if err != nil {
		panic(err)
	}
	return s
}()
