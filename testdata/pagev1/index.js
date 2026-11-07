// A version 0.1 page: a Markdown body, one section, and one item with two
// actions, one command and one url.
export function page({ id }) {
  return {
    title: id === "detail" ? "Detail" : id,
    subtitle: "Markdown and actions",
    markdown: "Some **detail** from the plugin.\n\nIt has two paragraphs.",
    sections: [
      {
        title: "Actions",
        items: [
          {
            id: "one",
            title: "One",
            subtitle: "A row with two actions",
            kind: "note",
            actions: [
              { id: "play", title: "Play", kind: "command", meta: { command: "play", item: "one" } },
              { id: "open", title: "Open", kind: "url", url: "https://example.com/one" },
            ],
          },
        ],
      },
    ],
  };
}
