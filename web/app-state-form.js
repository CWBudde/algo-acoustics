import {
  average,
  capitalize,
  clamp,
  clampInt,
  copyState,
} from "./app-utils.js";
import { requireAppStateContext } from "./app-state-context.js";
import { decodeStateFromUrl, scheduleUrlSync } from "./app-state-url.js";
import { normalizeReflectionOrder } from "./reflection-preview.mjs";

function requireCtx() {
  return requireAppStateContext();
}

export function populateMaterialSelects() {
  const { refs, MATERIALS } = requireCtx();
  const materialEntries = Object.entries(MATERIALS);
  [
    refs.wallWest,
    refs.wallEast,
    refs.wallSouth,
    refs.wallNorth,
    refs.wallFloor,
    refs.wallCeiling,
  ].forEach((select) => {
    select.innerHTML = materialEntries
      .map(
        ([value, material]) =>
          `<option value="${value}">${material.label}</option>`,
      )
      .join("");
  });
}

export function populatePresetSelects() {
  const { refs, ROOM_PRESET_GROUPS, ROOM_PRESETS, MATERIAL_PRESETS } = requireCtx();
  const roomOptions = ROOM_PRESET_GROUPS.map(({ label, presets }) => {
    const options = presets
      .filter((value) => value in ROOM_PRESETS)
      .map((value) => `<option value="${value}">${ROOM_PRESETS[value].label}</option>`)
      .join("");
    return `<optgroup label="${label}">${options}</optgroup>`;
  }).join("");

  refs.roomPreset.innerHTML = `
    <option value="custom">Custom</option>
    ${roomOptions}
  `;
  refs.materialPreset.innerHTML = Object.entries(MATERIAL_PRESETS)
    .map(([value, preset]) => `<option value="${value}">${preset.label}</option>`)
    .join("");
}

export function updateMaterialSummary() {
  const { refs, state, MATERIALS } = requireCtx();
  refs.materialSummary.innerHTML = Object.entries(state.materials)
    .map(([wall, materialKey]) => {
      const material = MATERIALS[materialKey];
      const absorptionAverage = average(material.absorption);
      return `
        <article class="metric-card">
          <span class="metric-label"><strong>${capitalize(wall)}</strong></span>
          <strong>${material.label}</strong>
          <small>${Math.round(absorptionAverage * 100)}% absorption</small>
        </article>
      `;
    })
    .join("");
}

export function syncPresetSelects() {
  const { refs, state, ROOM_PRESETS, MATERIAL_PRESETS } = requireCtx();
  refs.roomPreset.value = state.roomPreset in ROOM_PRESETS ? state.roomPreset : "custom";
  refs.materialPreset.value = state.materialPreset in MATERIAL_PRESETS ? state.materialPreset : "custom";
}

export function syncFormFromState() {
  const { refs, state } = requireCtx();
  refs.roomPreset.value = state.roomPreset;
  refs.materialPreset.value = state.materialPreset;
  refs.sourceDirectivity.value = state.source.directivity;
  refs.sourceAzimuth.value = String(state.source.azimuthDegrees);
  refs.sourceAzimuthValue.textContent = `${Math.round(state.source.azimuthDegrees)}°`;
  refs.sourceFocus.value = String(state.source.cardioidOrder);
  refs.sourceFocusValue.textContent = state.source.cardioidOrder.toFixed(2);
  refs.sourceGain.value = String(state.source.gainDb);
  refs.sourceGainValue.textContent = `${state.source.gainDb.toFixed(1)} dB`;

  refs.roomWidth.value = state.room.width.toFixed(1);
  refs.roomDepth.value = state.room.depth.toFixed(1);
  refs.roomHeight.value = state.room.height.toFixed(1);
  syncPositionConstraints();
  syncRoomModeAvailability();
  refs.sourceX.value = state.source.x.toFixed(2);
  refs.sourceY.value = state.source.y.toFixed(2);
  refs.sourceZ.value = state.source.z.toFixed(2);
  refs.receiverX.value = state.receiver.x.toFixed(2);
  refs.receiverY.value = state.receiver.y.toFixed(2);
  refs.receiverZ.value = state.receiver.z.toFixed(2);

  refs.wallWest.value = state.materials.west;
  refs.wallEast.value = state.materials.east;
  refs.wallSouth.value = state.materials.south;
  refs.wallNorth.value = state.materials.north;
  refs.wallFloor.value = state.materials.floor;
  refs.wallCeiling.value = state.materials.ceiling;

  refs.renderRays.value = String(state.render.numRays);
  refs.renderRaysValue.textContent = `${state.render.numRays.toLocaleString()} rays`;
  refs.renderOrder.value = String(state.render.maxOrder);
  refs.renderOrderValue.textContent = `order ${state.render.maxOrder}`;
  refs.renderDuration.value = String(state.render.durationSeconds);
  refs.renderDurationValue.textContent = `${state.render.durationSeconds.toFixed(2)} s`;
  refs.renderCrossover.max = String(
    Math.max(0.03, state.render.durationSeconds * 0.85),
  );
  refs.renderCrossover.value = String(state.render.crossoverTimeSeconds);
  refs.renderCrossoverValue.textContent = `${state.render.crossoverTimeSeconds.toFixed(2)} s`;
  refs.renderWindow.value = state.render.crossoverWindow;
  refs.sceneReflections.value = String(state.reflections);

  syncModeButtons();
  syncDirectivityAvailability();
}

export function syncDirectivityAvailability() {
  const { refs, state } = requireCtx();
  const isCardioid = state.source.directivity === "cardioid";
  refs.sourceAzimuth.disabled = !isCardioid;
  refs.sourceFocus.disabled = !isCardioid;
}

export function syncRoomModeAvailability() {
  const { refs, state } = requireCtx();
  const isMesh = state.room.kind === "mesh";
  refs.roomWidth.disabled = false;
  refs.roomDepth.disabled = false;
  refs.roomHeight.disabled = false;
  if (isMesh) {
    refs.roomWidth.title = "Mesh preset uses bounding-box dimensions";
    refs.roomDepth.title = "Mesh preset uses bounding-box dimensions";
    refs.roomHeight.title = "Mesh preset uses bounding-box dimensions";
  } else {
    refs.roomWidth.removeAttribute("title");
    refs.roomDepth.removeAttribute("title");
    refs.roomHeight.removeAttribute("title");
  }
}

export function syncModeButtons() {
  const { refs, state } = requireCtx();
  refs.renderModeButtons.forEach((button) => {
    button.classList.toggle(
      "is-active",
      button.dataset.mode === state.render.mode,
    );
  });
}

export function syncIrViewButtons() {
  const { refs, state } = requireCtx();
  refs.irViewButtons.forEach((button) => {
    button.classList.toggle("is-active", button.dataset.view === state.irView);
  });
}

export function syncPositionConstraints() {
  const { refs, state, POSITION_MARGIN } = requireCtx();
  const fields = [
    [refs.sourceX, state.room.width],
    [refs.receiverX, state.room.width],
    [refs.sourceY, state.room.depth],
    [refs.receiverY, state.room.depth],
    [refs.sourceZ, state.room.height],
    [refs.receiverZ, state.room.height],
  ];
  fields.forEach(([element, maxValue]) => {
    element.max = String(Math.max(POSITION_MARGIN, maxValue - POSITION_MARGIN));
  });
}

export function normalizeSpatialState() {
  const { state, POSITION_MARGIN } = requireCtx();
  state.source.x = clamp(
    state.source.x,
    POSITION_MARGIN,
    state.room.width - POSITION_MARGIN,
  );
  state.source.y = clamp(
    state.source.y,
    POSITION_MARGIN,
    state.room.depth - POSITION_MARGIN,
  );
  state.source.z = clamp(
    state.source.z,
    POSITION_MARGIN,
    state.room.height - POSITION_MARGIN,
  );
  state.receiver.x = clamp(
    state.receiver.x,
    POSITION_MARGIN,
    state.room.width - POSITION_MARGIN,
  );
  state.receiver.y = clamp(
    state.receiver.y,
    POSITION_MARGIN,
    state.room.depth - POSITION_MARGIN,
  );
  state.receiver.z = clamp(
    state.receiver.z,
    POSITION_MARGIN,
    state.room.height - POSITION_MARGIN,
  );
}

export function applyRoomPreset(presetName, options = {}) {
  const {
    state,
    ROOM_PRESETS,
    MATERIAL_PRESETS,
    updateSceneView,
    drawWaveform,
    refs,
  } = requireCtx();
  const preset = ROOM_PRESETS[presetName] ?? ROOM_PRESETS.custom;
  state.roomPreset = presetName in ROOM_PRESETS ? presetName : "custom";

  if (preset.kind === "mesh") {
    state.room = {
      kind: "mesh",
      width: preset.room.width,
      depth: preset.room.depth,
      height: preset.room.height,
      mesh: structuredClone(preset.mesh),
    };
  } else {
    state.room = {
      kind: "shoebox",
      width: preset.room.width,
      depth: preset.room.depth,
      height: preset.room.height,
      mesh: null,
    };
  }

  if (preset.source) {
    state.source = structuredClone(preset.source);
  }
  if (preset.receiver) {
    state.receiver = structuredClone(preset.receiver);
  }
  if (preset.materialPreset && preset.materialPreset in MATERIAL_PRESETS) {
    state.materialPreset = preset.materialPreset;
  }
  if (preset.materials) {
    state.materials = structuredClone(preset.materials);
    state.materialPreset = "custom";
  }
  if (preset.renderMode) {
    state.render.mode = preset.renderMode;
  }

  syncFormFromState();
  updateSceneView();
  drawWaveform(refs.waveformCanvas, requireCtx().getLastRender?.()?.samples ?? null, {
    irView: state.irView,
    renderMode: state.render.mode,
    durationSeconds: state.render.durationSeconds,
    crossoverTimeSeconds: state.render.crossoverTimeSeconds,
  });
  if (!options.skipUrlSync) {
    scheduleUrlSync();
  }
}

export function applyMaterialPreset(presetName, options = {}) {
  const { state, MATERIAL_PRESETS, updateMaterialSummary, updateSceneView } = requireCtx();
  const preset = MATERIAL_PRESETS[presetName] ?? MATERIAL_PRESETS.custom;
  state.materialPreset = presetName in MATERIAL_PRESETS ? presetName : "custom";

  if (preset.materials) {
    state.materials = structuredClone(preset.materials);
  }

  syncFormFromState();
  updateMaterialSummary();
  updateSceneView();
  if (!options.skipUrlSync) {
    scheduleUrlSync();
  }
}

export function loadStateFromUrl() {
  const { window } = requireCtx();
  const params = new URLSearchParams(window.location.search);
  const encoded = params.get("scene");
  if (!encoded) {
    syncPresetSelects();
    return;
  }

  try {
    const decoded = decodeStateFromUrl(encoded);
    applySerializedState(decoded);
  } catch (error) {
    console.warn("Ignoring invalid scene URL", error);
  }
}

export function applySerializedState(input) {
  const {
    state,
    DEFAULT_STATE,
    ROOM_PRESETS,
    MATERIAL_PRESETS,
    updateSceneView,
  } = requireCtx();
  copyState(DEFAULT_STATE, state);

  if (input && typeof input === "object") {
    if (typeof input.roomPreset === "string" && input.roomPreset in ROOM_PRESETS) {
      state.roomPreset = input.roomPreset;
      applyRoomPreset(input.roomPreset, { skipUrlSync: true });
    }
    if (typeof input.materialPreset === "string" && input.materialPreset in MATERIAL_PRESETS) {
      state.materialPreset = input.materialPreset;
      applyMaterialPreset(input.materialPreset, { skipUrlSync: true });
    }

    assignRoomState(input.room);
    assignMaterialState(input.materials);
    assignSourceState(input.source);
    assignReceiverState(input.receiver);
    assignRenderState(input.render);

    if (typeof input.irView === "string" && input.irView) {
      state.irView = input.irView;
    }
    if (typeof input.reflections === "number" && Number.isFinite(input.reflections)) {
      state.reflections = input.reflections;
    }
  }

  syncFormFromState();
  updateMaterialSummary();
  updateSceneView();
  syncPresetSelects();
}

export function assignRoomState(room) {
  const { state, ROOM_PRESETS } = requireCtx();
  if (!room || typeof room !== "object") {
    return;
  }
  if (typeof room.kind === "string" && room.kind) {
    state.room.kind = room.kind;
  }
  if (typeof room.width === "number" && Number.isFinite(room.width)) {
    state.room.width = room.width;
  }
  if (typeof room.depth === "number" && Number.isFinite(room.depth)) {
    state.room.depth = room.depth;
  }
  if (typeof room.height === "number" && Number.isFinite(room.height)) {
    state.room.height = room.height;
  }
  if (room.kind === "mesh" && room.mesh) {
    state.room.mesh = structuredClone(room.mesh);
  }
  if (room.preset && room.preset in ROOM_PRESETS) {
    state.roomPreset = room.preset;
  }
}

export function assignMaterialState(materials) {
  const { state, MATERIALS } = requireCtx();
  if (!materials || typeof materials !== "object") {
    return;
  }
  for (const key of ["west", "east", "south", "north", "floor", "ceiling"]) {
    if (typeof materials[key] === "string" && materials[key] in MATERIALS) {
      state.materials[key] = materials[key];
    }
  }
}

export function assignSourceState(source) {
  const { state } = requireCtx();
  if (!source || typeof source !== "object") {
    return;
  }
  if (typeof source.x === "number" && Number.isFinite(source.x)) {
    state.source.x = source.x;
  }
  if (typeof source.y === "number" && Number.isFinite(source.y)) {
    state.source.y = source.y;
  }
  if (typeof source.z === "number" && Number.isFinite(source.z)) {
    state.source.z = source.z;
  }
  if (typeof source.gainDb === "number" && Number.isFinite(source.gainDb)) {
    state.source.gainDb = source.gainDb;
  }
  if (typeof source.directivity === "string" && source.directivity) {
    state.source.directivity = source.directivity;
  }
  if (typeof source.azimuthDegrees === "number" && Number.isFinite(source.azimuthDegrees)) {
    state.source.azimuthDegrees = source.azimuthDegrees;
  }
  if (typeof source.cardioidOrder === "number" && Number.isFinite(source.cardioidOrder)) {
    state.source.cardioidOrder = source.cardioidOrder;
  }
}

export function assignReceiverState(receiver) {
  const { state } = requireCtx();
  if (!receiver || typeof receiver !== "object") {
    return;
  }
  if (typeof receiver.x === "number" && Number.isFinite(receiver.x)) {
    state.receiver.x = receiver.x;
  }
  if (typeof receiver.y === "number" && Number.isFinite(receiver.y)) {
    state.receiver.y = receiver.y;
  }
  if (typeof receiver.z === "number" && Number.isFinite(receiver.z)) {
    state.receiver.z = receiver.z;
  }
}

export function assignRenderState(render) {
  const { state } = requireCtx();
  if (!render || typeof render !== "object") {
    return;
  }
  if (typeof render.mode === "string" && render.mode) {
    state.render.mode = render.mode;
  }
  if (typeof render.maxOrder === "number" && Number.isFinite(render.maxOrder)) {
    state.render.maxOrder = render.maxOrder;
  }
  if (typeof render.numRays === "number" && Number.isFinite(render.numRays)) {
    state.render.numRays = render.numRays;
  }
  if (typeof render.durationSeconds === "number" && Number.isFinite(render.durationSeconds)) {
    state.render.durationSeconds = render.durationSeconds;
  }
  if (typeof render.crossoverTimeSeconds === "number" && Number.isFinite(render.crossoverTimeSeconds)) {
    state.render.crossoverTimeSeconds = render.crossoverTimeSeconds;
  }
  if (typeof render.crossoverWindow === "string" && render.crossoverWindow) {
    state.render.crossoverWindow = render.crossoverWindow;
  }
}

export function normalizeSceneState() {
  const { state } = requireCtx();
  state.room.width = clamp(state.room.width, 2.5, 16);
  state.room.depth = clamp(state.room.depth, 2.5, 16);
  state.room.height = clamp(state.room.height, 2.2, 7);
  state.source.gainDb = clamp(state.source.gainDb, -24, 12);
  state.source.azimuthDegrees = clamp(state.source.azimuthDegrees, -180, 180);
  state.source.cardioidOrder = clamp(state.source.cardioidOrder, 0.25, 2.5);
  state.render.maxOrder = clampInt(Math.round(state.render.maxOrder), 1, 12);
  state.render.numRays = clampInt(Math.round(state.render.numRays), 128, 16384);
  state.render.durationSeconds = clamp(state.render.durationSeconds, 0.25, 3);
  state.render.crossoverTimeSeconds = clamp(
    state.render.crossoverTimeSeconds,
    0.03,
    state.render.durationSeconds * 0.85,
  );
  normalizeSpatialState();
  state.roomPreset = state.roomPreset in requireCtx().ROOM_PRESETS ? state.roomPreset : "custom";
  state.materialPreset = state.materialPreset in requireCtx().MATERIAL_PRESETS ? state.materialPreset : "custom";
  state.reflections = normalizeReflectionOrder(state.reflections);
  if (state.room.kind === "mesh" && !state.room.mesh) {
    state.room.kind = "shoebox";
  }
  if (state.room.kind !== "mesh") {
    state.room.mesh = null;
  }
}

export function normalizeCrossoverWindow(name, fallback) {
  return ["hann", "blackman", "rect"].includes(name) ? name : fallback;
}
