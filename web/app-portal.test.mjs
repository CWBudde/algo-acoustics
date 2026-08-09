import test from "node:test";
import assert from "node:assert/strict";

import { PORTAL_MATERIALS, ROOM_PRESETS } from "./app-presets.js";
import { setAppStateContext } from "./app-state-context.js";
import {
  buildRequest,
  decodeStateFromUrl,
  encodeStateForUrl,
} from "./app-state-url.js";
import { copyState } from "./app-utils.js";

function portalState() {
  return {
    roomPreset: "twoRoom",
    materialPreset: "studio",
    room: { kind: "shoebox", width: 5.625, depth: 4, height: 4 },
    materials: {},
    source: { x: 1.5, y: 2, z: 1.4 },
    receiver: { x: 4.1, y: 2, z: 1.4 },
    portal: structuredClone(ROOM_PRESETS.twoRoom.portal),
    render: { mode: "hybrid" },
    irView: "linear",
    reflections: 1,
  };
}

test("two-room preset defines a local receiver room and valid portal material", () => {
  const preset = ROOM_PRESETS.twoRoom;

  assert.equal(preset.kind, "shoebox");
  assert.equal(preset.portal.enabled, true);
  assert.equal(preset.portal.rootOrder, 2);
  assert.ok(preset.portal.material in PORTAL_MATERIALS);
  assert.ok(preset.portal.receiverRoom.width > 0);
  assert.ok(preset.portal.opening.width > 0);
  assert.ok(preset.portal.opening.height > 0);
  assert.ok(preset.receiver.x < preset.portal.receiverRoom.width);
});

test("portal state round-trips through URL serialization and render requests", () => {
  const state = portalState();
  state.portal.aperture = 0.37;
  state.portal.material = "glassPartition";
  setAppStateContext({ state });

  const decoded = decodeStateFromUrl(encodeStateForUrl());
  assert.deepEqual(decoded.portal, state.portal);

  const request = buildRequest();
  assert.deepEqual(request.portal, state.portal);
});

test("copyState defensively copies nested portal geometry", () => {
  const source = portalState();
  const target = {};

  copyState(source, target);
  target.portal.receiverRoom.width = 99;
  target.portal.opening.width = 99;

  assert.equal(source.portal.receiverRoom.width, 5.625);
  assert.equal(source.portal.opening.width, 1.2);
});
