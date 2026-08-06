export const DRY_AUDIO_SOURCES = Object.freeze({
  clap: Object.freeze({ label: "Clap", url: "audio/clap.mp3" }),
  speech: Object.freeze({ label: "Speech", url: "audio/speech.mp3" }),
  music: Object.freeze({ label: "Music bed", url: "audio/music.mp3" }),
});

export class DryAudioLoader {
  constructor({ fetchFn } = {}) {
    const resolvedFetch = fetchFn ?? globalThis.fetch?.bind(globalThis);
    if (typeof resolvedFetch !== "function") {
      throw new Error("a fetch implementation is required");
    }
    this.fetchFn = resolvedFetch;
    this.contextCaches = new Map();
  }

  load(context, sourceName) {
    const source = DRY_AUDIO_SOURCES[sourceName];
    if (!source) {
      return Promise.reject(new Error(`unknown dry source: ${sourceName}`));
    }

    let cache = this.contextCaches.get(context);
    if (!cache) {
      cache = new Map();
      this.contextCaches.set(context, cache);
    }
    if (cache.has(sourceName)) {
      return cache.get(sourceName);
    }

    const pending = this.fetchFn(source.url)
      .then((response) => {
        if (!response.ok) {
          throw new Error(
            `load ${source.label.toLowerCase()}: HTTP ${response.status}`,
          );
        }
        return response.arrayBuffer();
      })
      .then((bytes) => context.decodeAudioData(bytes))
      .catch((error) => {
        cache.delete(sourceName);
        throw error;
      });
    cache.set(sourceName, pending);
    return pending;
  }
}
