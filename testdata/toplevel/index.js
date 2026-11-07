// Reading storage while the module is evaluated must not deadlock the host.
const saved = host.storage.get("session");

export function home() {
  return { title: saved || "OK" };
}
