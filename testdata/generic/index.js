export function page({ id }) {
  return {
    title: id === "home" ? "Home" : id,
    subtitle: "A plugin that isn't music",
    sections: [
      {
        title: "Things",
        items: [
          { id: "one", title: "First item", subtitle: "From a plugin", kind: "note" },
        ],
      },
    ],
  };
}
