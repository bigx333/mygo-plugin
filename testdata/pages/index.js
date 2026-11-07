// A plugin page is data: the host decides how it looks.
export function browse({ page }) {
  return {
    title: "Demo",
    subtitle: "A page",
    sections: [
      {
        title: "Tracks",
        items: [{ id: "1", title: "One", kind: "track" }],
      },
    ],
  };
}
