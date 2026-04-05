import test from "node:test";
import assert from "node:assert/strict";

import { getReflectionPreviewConfig } from "./reflection-preview.mjs";

test("reflection preview keeps the direct path visible and adds only the requested reflection orders", () => {
  assert.deepEqual(getReflectionPreviewConfig(0), {
    showDirectPath: true,
    reflectionOrders: [],
  });

  assert.deepEqual(getReflectionPreviewConfig(1), {
    showDirectPath: true,
    reflectionOrders: [1],
  });

  assert.deepEqual(getReflectionPreviewConfig(2), {
    showDirectPath: true,
    reflectionOrders: [1, 2],
  });
});
