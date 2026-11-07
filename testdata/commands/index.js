// The fixture for commands and capability negotiation: one export runs every
// command the manifest declares, and one asks the host what it offers.

export function command({ id, args }) {
  if (id === "echo") {
    return { ok: true };
  }
  const error = new Error("unknown command: " + id);
  error.code = "unavailable";
  throw error;
}

export function capability({ name }) {
  return host.capability(name);
}
