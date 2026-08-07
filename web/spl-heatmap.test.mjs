import assert from "node:assert/strict";
import test from "node:test";

import {
  interpolateSPL,
  normalizeSPLHeatmap,
  samplesForSurface,
  splHeatmapColor,
} from "./spl-heatmap.mjs";

test("normalizeSPLHeatmap rejects empty input and filters invalid samples", () => {
  assert.equal(normalizeSPLHeatmap(null), null);
  assert.equal(normalizeSPLHeatmap({ samples: [] }), null);

  const heatmap = normalizeSPLHeatmap({
    minimumDb: -30,
    maximumDb: 0,
    samples: [
      { surfaceId: "floor", x: 0, y: 0, z: 0, levelDb: -12 },
      { surfaceId: "", x: 0, y: 0, z: 0, levelDb: -4 },
      { surfaceId: "floor", x: Number.NaN, y: 0, z: 0, levelDb: -4 },
    ],
  });

  assert.equal(heatmap.samples.length, 1);
  assert.equal(samplesForSurface(heatmap, "floor").length, 1);
  assert.deepEqual(samplesForSurface(heatmap, "ceiling"), []);
});

test("interpolateSPL preserves exact probes and blends nearby levels", () => {
  const samples = [
    { x: 0, y: 0, z: 0, levelDb: -30 },
    { x: 2, y: 0, z: 0, levelDb: 0 },
  ];

  assert.equal(interpolateSPL(samples, { x: 0, y: 0, z: 0 }), -30);
  assert.equal(interpolateSPL(samples, { x: 1, y: 0, z: 0 }), -15);
  assert.equal(interpolateSPL([], { x: 0, y: 0, z: 0 }), null);
});

test("splHeatmapColor clamps the scale and returns finite RGB channels", () => {
  assert.deepEqual(splHeatmapColor(-40, -30, 0), splHeatmapColor(-30, -30, 0));
  assert.deepEqual(splHeatmapColor(5, -30, 0), splHeatmapColor(0, -30, 0));

  const middle = splHeatmapColor(-15, -30, 0);
  assert.equal(middle.length, 3);
  assert.ok(middle.every((channel) => channel >= 0 && channel <= 1));
});
