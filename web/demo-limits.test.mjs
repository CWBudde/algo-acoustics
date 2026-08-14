import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { applyRangeBounds, rangeBoundsFromLimits } from "./demo-limits.mjs";

// The shape web/wasm/limits.go publishes on the worker's ready message.
const ENGINE_LIMITS = {
  sampleRate: 48000,
  maxSurfaces: 50,
  maxMeshTriangles: 20000,
  maxMaterials: 128,
  minNumRays: 128,
  maxNumRays: 16384,
  minMaxOrder: 1,
  maxMaxOrder: 12,
  minDurationSeconds: 0.25,
  maxDurationSeconds: 3,
  minRoomMeters: 2,
  maxRoomMeters: 50,
  renderTimeoutSeconds: 10,
};

function fakeDocument(inputs) {
  return {
    getElementById(id) {
      return inputs[id] ?? null;
    },
  };
}

function fakeRange(value) {
  return { value: String(value), min: "", max: "", step: "" };
}

describe("rangeBoundsFromLimits", () => {
  it("derives the render slider bounds from the engine envelope", () => {
    const bounds = rangeBoundsFromLimits(ENGINE_LIMITS);

    assert.deepEqual(bounds["render-rays"], { min: 128, max: 16384, step: 128 });
    assert.deepEqual(bounds["render-order"], { min: 1, max: 12, step: 1 });
    assert.deepEqual(bounds["render-duration"], {
      min: 0.25,
      max: 3,
      step: 0.05,
    });
  });

  it("keeps every maximum reachable in whole steps", () => {
    const bounds = rangeBoundsFromLimits(ENGINE_LIMITS);

    for (const [id, bound] of Object.entries(bounds)) {
      const steps = (bound.max - bound.min) / bound.step;
      assert.ok(
        Math.abs(steps - Math.round(steps)) < 1e-9,
        `${id} cannot be dragged to its maximum: ${steps} steps`,
      );
    }
  });

  it("returns nothing when the engine published no limits", () => {
    assert.deepEqual(rangeBoundsFromLimits(null), {});
    assert.deepEqual(rangeBoundsFromLimits(undefined), {});
    assert.deepEqual(rangeBoundsFromLimits({}), {});
  });

  it("skips a limit the engine reported as unusable", () => {
    const bounds = rangeBoundsFromLimits({
      ...ENGINE_LIMITS,
      maxNumRays: 0,
      minDurationSeconds: Number.NaN,
    });

    assert.equal(bounds["render-rays"], undefined);
    assert.equal(bounds["render-duration"], undefined);
    assert.ok(bounds["render-order"]);
  });
});

describe("applyRangeBounds", () => {
  it("writes the bounds onto the inputs", () => {
    const rays = fakeRange(3072);
    const document = fakeDocument({ "render-rays": rays });

    const clamped = applyRangeBounds(
      document,
      rangeBoundsFromLimits(ENGINE_LIMITS),
    );

    assert.equal(rays.min, "128");
    assert.equal(rays.max, "16384");
    assert.equal(rays.step, "128");
    assert.deepEqual(clamped, []);
  });

  it("pulls an out-of-range value back inside and reports it", () => {
    // A shared URL from a build with a wider envelope.
    const rays = fakeRange(65536);
    const duration = fakeRange(0.05);
    const document = fakeDocument({
      "render-rays": rays,
      "render-duration": duration,
    });

    const clamped = applyRangeBounds(
      document,
      rangeBoundsFromLimits(ENGINE_LIMITS),
    );

    assert.equal(rays.value, "16384");
    assert.equal(duration.value, "0.25");
    assert.deepEqual(clamped.sort(), ["render-duration", "render-rays"]);
  });

  it("ignores inputs the page does not have", () => {
    const clamped = applyRangeBounds(
      fakeDocument({}),
      rangeBoundsFromLimits(ENGINE_LIMITS),
    );

    assert.deepEqual(clamped, []);
  });
});
