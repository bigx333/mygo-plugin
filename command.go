package plugin

// RunCommand runs the command named id, with args if it takes any. A plugin
// declares its commands in contributes.commands and runs them all through one
// export, command({id, args}).
//
// A plugin that doesn't export command is ErrUnsupported. What the plugin
// throws, and any other failure, comes back as it does from Call.
func (p *Plugin) RunCommand(id string, args map[string]any) error {
	return p.Command(id, args, nil)
}

// Command runs the command named id and decodes what it answers into out,
// unless out is nil. RunCommand is Command without an answer.
func (p *Plugin) Command(id string, args map[string]any, out any) error {
	if !p.Can("command") {
		return ErrUnsupported
	}
	return p.Call("command", map[string]any{"id": id, "args": args}, out)
}
