const song = {
  id: "1", name: "A Song",
  artists: [{ id: "10", name: "An Artist" }],
  album: { id: "20", name: "An Album", imageUrl: "https://example.com/a.jpg" },
  duration: 200000, url: "https://example.com/song/1", unavailable: false,
};

export function home() {
  return {
    title: "Home", subtitle: "Made for you",
    sections: [{
      title: "Made for you",
      tracks: [song],
      playlists: [{ id: "30", name: "A Playlist", owner: "An Owner", trackCount: 3 }],
      links: [{ title: "Daily", page: "daily" }],
      stations: [{ id: "DEFAULT", name: "Radio", description: "Endless" }],
      shows: [{ id: "40", name: "A Show", publisher: "A Publisher" }],
    }],
  };
}

export function browse({ page }) {
  return { title: page, sections: [{ tracks: [song] }] };
}

export function search({ query }) {
  return { tracks: [song], more: false };
}

export function playlist({ id }) {
  return { id, name: "A Playlist", description: "Three songs", tracks: [song] };
}

export function liked() {
  return { tracks: [song], more: false };
}

export function stations() {
  return [{ id: "DEFAULT", name: "Radio", description: "Endless" }];
}

export function stream({ id }) {
  return { url: "https://example.com/" + id + ".mp3", type: "mp3", headers: { "User-Agent": "test" } };
}

export function stationNext({ id }) {
  return [song];
}
