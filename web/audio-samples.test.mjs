import test from "node:test";
import assert from "node:assert/strict";
import { stat } from "node:fs/promises";

import { DRY_AUDIO_SOURCES, DryAudioLoader } from "./audio-samples.mjs";

test("dry-source catalog exposes the three bundled samples", () => {
  assert.deepEqual(Object.keys(DRY_AUDIO_SOURCES), ["clap", "speech", "music"]);
  for (const source of Object.values(DRY_AUDIO_SOURCES)) {
    assert.match(source.url, /^audio\/[a-z]+\.mp3$/);
  }
});

test("bundled samples exist within the web size budget", async () => {
  let totalBytes = 0;
  for (const source of Object.values(DRY_AUDIO_SOURCES)) {
    const file = new URL(source.url, import.meta.url);
    const info = await stat(file);
    assert.ok(info.size > 1_000, `${source.url} is unexpectedly small`);
    totalBytes += info.size;
  }
  assert.ok(totalBytes < 200 * 1024, `audio assets total ${totalBytes} bytes`);
});

test("loader fetches and decodes each sample once per audio context", async () => {
  const requests = [];
  const decoded = { duration: 1.25 };
  const context = {
    decodeAudioData: async (bytes) => {
      assert.equal(bytes.byteLength, 4);
      return decoded;
    },
  };
  const loader = new DryAudioLoader({
    fetchFn: async (url) => {
      requests.push(url);
      return {
        ok: true,
        status: 200,
        arrayBuffer: async () => new ArrayBuffer(4),
      };
    },
  });

  const first = loader.load(context, "speech");
  const second = loader.load(context, "speech");
  assert.equal(first, second);
  assert.equal(await first, decoded);
  assert.deepEqual(requests, ["audio/speech.mp3"]);
});

test("loader reports HTTP failures and permits a retry", async () => {
  let attempts = 0;
  const loader = new DryAudioLoader({
    fetchFn: async () => {
      attempts += 1;
      return {
        ok: attempts > 1,
        status: attempts > 1 ? 200 : 404,
        arrayBuffer: async () => new ArrayBuffer(2),
      };
    },
  });
  const context = { decodeAudioData: async () => ({ duration: 1 }) };

  await assert.rejects(loader.load(context, "clap"), /HTTP 404/);
  await assert.doesNotReject(loader.load(context, "clap"));
  assert.equal(attempts, 2);
});

test("loader rejects unknown source names", async () => {
  const loader = new DryAudioLoader({ fetchFn: async () => assert.fail() });
  await assert.rejects(loader.load({}, "missing"), /unknown dry source/);
});
