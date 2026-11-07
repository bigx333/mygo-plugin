export function page({ id }) {
  return {
    title: "Notes",
    subtitle: "A generic plugin, not a music source",
    sections: [
      {
        title: "Today",
        items: [
          {
            id: "welcome",
            title: "Welcome",
            subtitle: "This page came from a plugin",
            kind: "note",
          },
        ],
      },
    ],
  };
}

export function command({ id, args }) {
  if (id === "hello") {
    return { id: "hello", ok: true, message: "Hello from Notes", args: args ?? null };
  }
  return { id, ok: false, message: `unknown command: ${id}` };
}
