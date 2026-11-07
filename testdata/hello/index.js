// The fixture every host test runs: one export for each thing a host does.
export function hello({ name }) {
  return { text: "hello " + name };
}

export async function later({ value }) {
  await new Promise((resolve) => setTimeout(resolve, 10));
  return value;
}

export function add({ a, b }) {
  return a + b;
}

export function store({ key, value }) {
  host.storage.set(key, value);
  return host.storage.get(key);
}

export function stored({ key }) {
  return host.storage.get(key) ?? null;
}

export function hashes({ text }) {
  return {
    md5: host.crypto.md5(text),
    aes: host.crypto.aesEcbEncrypt("0123456789abcdef", text),
  };
}

export async function fetchText({ url }) {
  const response = await fetch(url);
  return await response.text();
}

export function translated() {
  return host.t("Hello");
}

export function prompt() {
  host.signInPrompt({ qr: "QR", text: "scan" });
  return host.language();
}

export function fail() {
  const error = new Error("sign in please");
  error.code = "signIn";
  throw error;
}

export function spin() {
  for (;;) {}
}

export function claim() {
  host.spawn("echo");
}
