import assert from "node:assert/strict";
import { describe, it } from "node:test";

import {
  describeEstimatedRt60,
  describePreviewTier,
  findTimeoutWarning,
  formatEstimatedRt60,
  readPreviewTier,
  readStatisticalTier,
} from "./render-tiers.mjs";

describe("readStatisticalTier", () => {
  it("reads the estimates the demo displays", () => {
    const statistics = readStatisticalTier({
      statistics: {
        sabineRt60Secs: 0.62,
        eyringRt60Secs: 0.55,
        c80Db: 4.1,
        d50: 0.68,
      },
    });

    assert.deepEqual(statistics, {
      sabineRt60Secs: 0.62,
      eyringRt60Secs: 0.55,
      c80Db: 4.1,
      d50: 0.68,
    });
  });

  it("returns null when the room has no shoebox model to estimate from", () => {
    assert.equal(readStatisticalTier({}), null);
    assert.equal(readStatisticalTier({ statistics: {} }), null);
    assert.equal(readStatisticalTier(null), null);
  });

  it("rejects a non-positive decay time rather than showing 0.00 s", () => {
    assert.equal(readStatisticalTier({ statistics: { sabineRt60Secs: 0 } }), null);
    assert.equal(
      readStatisticalTier({ statistics: { sabineRt60Secs: Number.NaN } }),
      null,
    );
  });

  it("falls back to Sabine when Eyring is missing", () => {
    const statistics = readStatisticalTier({
      statistics: { sabineRt60Secs: 0.8 },
    });

    assert.equal(statistics.eyringRt60Secs, 0.8);
    assert.equal(statistics.c80Db, 0);
  });
});

describe("readPreviewTier", () => {
  it("keeps the tier's own duration, not the requested one", () => {
    const preview = readPreviewTier({
      samples: new Float32Array([0.1, -0.2, 0.3]),
      sampleRate: 48000,
      durationSeconds: 1,
      numRays: 384,
      maxOrder: 2,
      earlyEventCount: 7,
      elapsedMs: 143.6,
    });

    assert.equal(preview.durationSeconds, 1);
    assert.equal(preview.numRays, 384);
    assert.equal(preview.maxOrder, 2);
    assert.equal(preview.earlyEventCount, 7);
    assert.equal(preview.samples.length, 3);
  });

  it("accepts a plain array from a structured clone", () => {
    const preview = readPreviewTier({ samples: [0.5, 0.25] });

    assert.ok(preview.samples instanceof Float32Array);
    assert.equal(preview.samples.length, 2);
  });

  it("returns null when there is nothing to draw", () => {
    assert.equal(readPreviewTier({}), null);
    assert.equal(readPreviewTier({ samples: new Float32Array() }), null);
    assert.equal(readPreviewTier(null), null);
  });
});

describe("tier formatting", () => {
  it("formats the estimate and says where it came from", () => {
    const statistics = {
      sabineRt60Secs: 0.617,
      eyringRt60Secs: 0.548,
      c80Db: 4,
      d50: 0.6,
    };

    assert.equal(formatEstimatedRt60(statistics), "0.62 s");
    assert.match(describeEstimatedRt60(statistics), /Sabine 0\.62 s/);
    assert.match(describeEstimatedRt60(statistics), /Eyring 0\.55 s/);
    // The number must not be mistaken for a measurement of the rendered IR.
    assert.match(describeEstimatedRt60(statistics), /estimated/);
  });

  it("describes a preview by what it actually rendered", () => {
    const line = describePreviewTier({
      elapsedMs: 143.6,
      numRays: 384,
      maxOrder: 2,
      durationSeconds: 1,
    });

    assert.match(line, /144 ms/);
    assert.match(line, /384 rays/);
    assert.match(line, /order 2/);
    assert.match(line, /1\.00 s/);
  });
});

describe("findTimeoutWarning", () => {
  it("finds the timeout warning the Go side emits", () => {
    const warning =
      "render timeout: exceeded the 10 s demo budget after 10.4 s; returning the preview result";

    assert.equal(findTimeoutWarning([warning]), warning);
    assert.equal(
      findTimeoutWarning(["memory budget: reduced rays from 8192 to 4096", warning]),
      warning,
    );
  });

  it("does not mistake a memory-budget warning for a timeout", () => {
    assert.equal(
      findTimeoutWarning(["memory budget: reduced rays from 8192 to 4096"]),
      null,
    );
    assert.equal(findTimeoutWarning([]), null);
    assert.equal(findTimeoutWarning(undefined), null);
  });
});
