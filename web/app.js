import * as presets from "./app-presets.js";
import * as sceneModule from "./app-scene.js";
import * as audioModule from "./app-audio.js";
import {
  average,
  capitalize,
  clamp,
  clampInt,
  dbToLinear,
  copyState,
  downloadBytes,
  formatVec,
  roundToStep,
} from "./app-utils.js";

const MATERIALS = presets.MATERIALS;
const ROOM_PRESETS = presets.ROOM_PRESETS;
const ROOM_PRESET_GROUPS = presets.ROOM_PRESET_GROUPS;
const MATERIAL_PRESETS = presets.MATERIAL_PRESETS;

const DEFAULT_STATE = {
  roomPreset: "custom",
  materialPreset: "custom",
  room: {
    kind: "shoebox",
    width: 6.4,
    depth: 4.8,
    height: 2.9,
    mesh: null,
  },
  materials: {
    west: "perforatedWood",
    east: "glassWindow",
    south: "smoothConcrete",
    north: "defaultMaterial",
    floor: "pileCarpet",
    ceiling: "heavyCurtain",
  },
  source: {
    x: 1.4,
    y: 1.9,
    z: 1.25,
    gainDb: 0,
    directivity: "omni",
    azimuthDegrees: 18,
    cardioidOrder: 1.15,
  },
  receiver: { x: 4.85, y: 2.9, z: 1.2 },
  render: {
    mode: "hybrid",
    maxOrder: 4,
    numRays: 3072,
    durationSeconds: 1.35,
    crossoverTimeSeconds: 0.22,
    crossoverWindow: "hann",
  },
  irView: "linear",
  reflections: 1,
};

const POSITION_MARGIN = 0.15;
const THEME_KEY = "algo-acoustics-demo-theme";
const THEME_MODES = ["auto", "light", "dark"];
const THEME_ICONS = {
  auto: `
    <circle cx="12" cy="12" r="5"></circle>
    <line x1="12" y1="1" x2="12" y2="3"></line>
    <line x1="12" y1="21" x2="12" y2="23"></line>
    <line x1="4.22" y1="4.22" x2="5.64" y2="5.64"></line>
    <line x1="18.36" y1="18.36" x2="19.78" y2="19.78"></line>
    <line x1="1" y1="12" x2="3" y2="12"></line>
    <line x1="21" y1="12" x2="23" y2="12"></line>
    <line x1="4.22" y1="19.78" x2="5.64" y2="18.36"></line>
    <line x1="18.36" y1="5.64" x2="19.78" y2="4.22"></line>
  `,
  light: `
    <circle cx="12" cy="12" r="5"></circle>
    <line x1="12" y1="1" x2="12" y2="3"></line>
    <line x1="12" y1="21" x2="12" y2="23"></line>
    <line x1="4.22" y1="4.22" x2="5.64" y2="5.64"></line>
    <line x1="18.36" y1="18.36" x2="19.78" y2="19.78"></line>
    <line x1="1" y1="12" x2="3" y2="12"></line>
    <line x1="21" y1="12" x2="23" y2="12"></line>
    <line x1="4.22" y1="19.78" x2="5.64" y2="18.36"></line>
    <line x1="18.36" y1="5.64" x2="19.78" y2="4.22"></line>
  `,
  dark: `
    <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"></path>
  `,
};

const state = structuredClone(DEFAULT_STATE);
let currentThemeMode = "auto";
let mediaThemeQuery = null;
let renderWorker = null;
let renderSession = null;
let renderButtonAnimation = 0;
let urlSyncTimer = 0;
let latestEncodedState = null;
let renderWorkerReady = false;
let audioContext = null;
let currentIRBuffer = null;
let currentAuralizationSource = null;
let currentAuralizationNodes = null;
let currentAuralizationPlaying = false;
let currentAuralizationStopTimer = 0;
let lastAuralizedWav = null;
let dragState = null;
let sceneRaycaster = null;
let scenePointer = null;
let sceneDragPlane = null;

const refs = {
  engineStatus: document.getElementById("engine-status"),
  themeToggle: document.getElementById("theme-toggle"),
  renderBadge: document.getElementById("render-badge"),
  resetScene: document.getElementById("reset-scene"),
  roomPreset: document.getElementById("room-preset"),
  materialPreset: document.getElementById("material-preset"),
  sourceDirectivity: document.getElementById("source-directivity"),
  sourceAzimuth: document.getElementById("source-azimuth"),
  sourceAzimuthValue: document.getElementById("source-azimuth-value"),
  sourceFocus: document.getElementById("source-focus"),
  sourceFocusValue: document.getElementById("source-focus-value"),
  sourceGain: document.getElementById("source-gain"),
  sourceGainValue: document.getElementById("source-gain-value"),
  sourceX: document.getElementById("source-x"),
  sourceY: document.getElementById("source-y"),
  sourceZ: document.getElementById("source-z"),
  receiverX: document.getElementById("receiver-x"),
  receiverY: document.getElementById("receiver-y"),
  receiverZ: document.getElementById("receiver-z"),
  roomWidth: document.getElementById("room-width"),
  roomDepth: document.getElementById("room-depth"),
  roomHeight: document.getElementById("room-height"),
  wallWest: document.getElementById("wall-west"),
  wallEast: document.getElementById("wall-east"),
  wallSouth: document.getElementById("wall-south"),
  wallNorth: document.getElementById("wall-north"),
  wallFloor: document.getElementById("wall-floor"),
  wallCeiling: document.getElementById("wall-ceiling"),
  materialSummary: document.getElementById("material-summary"),
  renderRays: document.getElementById("render-rays"),
  renderRaysValue: document.getElementById("render-rays-value"),
  renderOrder: document.getElementById("render-order"),
  renderOrderValue: document.getElementById("render-order-value"),
  renderDuration: document.getElementById("render-duration"),
  renderDurationValue: document.getElementById("render-duration-value"),
  renderCrossover: document.getElementById("render-crossover"),
  renderCrossoverValue: document.getElementById("render-crossover-value"),
  renderWindow: document.getElementById("render-window"),
  sceneReflections: document.getElementById("scene-reflections"),
  renderModeButtons: Array.from(
    document.querySelectorAll("#render-mode-switch .mode-button"),
  ),
  renderScene: document.getElementById("render-scene"),
  renderButtonTitle: document.querySelector("#render-scene .button-title"),
  renderButtonSubtitle: document.querySelector("#render-scene .button-subtitle"),
  downloadWav: document.getElementById("download-wav"),
  audioStatus: document.getElementById("audio-status"),
  drySource: document.getElementById("dry-source"),
  wetMix: document.getElementById("wet-mix"),
  wetMixValue: document.getElementById("wet-mix-value"),
  audioGain: document.getElementById("audio-gain"),
  audioGainValue: document.getElementById("audio-gain-value"),
  playAuralization: document.getElementById("play-auralization"),
  stopAuralization: document.getElementById("stop-auralization"),
  exportAuralization: document.getElementById("export-auralization"),
  metricFirstArrival: document.getElementById("metric-first-arrival"),
  metricPeak: document.getElementById("metric-peak"),
  metricEvents: document.getElementById("metric-events"),
  metricRenderTime: document.getElementById("metric-render-time"),
  waveformCanvas: document.getElementById("waveform-canvas"),
  irViewButtons: Array.from(
    document.querySelectorAll("#ir-view-switch .mode-button"),
  ),
  sceneCanvas: document.getElementById("scene-canvas"),
  audioPlayer: document.getElementById("audio-player"),
  renderLog: document.getElementById("render-log"),
};

let sceneView = null;
let lastRender = null;
let lastAudioURL = null;
let workerReadyResolve = null;
let workerReadyReject = null;
let currentRenderId = 0;

init();

async function init() {
  initTheme();
  populatePresetSelects();
  populateMaterialSelects();
  loadStateFromUrl();
  bindEvents();
  syncFormFromState();
  syncAuralizationControls();
  updateMaterialSummary();
  drawWaveform(refs.waveformCanvas, null, {
    irView: state.irView,
    renderMode: state.render.mode,
    durationSeconds: state.render.durationSeconds,
    crossoverTimeSeconds: state.render.crossoverTimeSeconds,
  });
  sceneView = createSceneView(refs.sceneCanvas);
  bindSceneInteractions(refs.sceneCanvas);
  applySceneTheme();
  updateSceneView();
  updateRenderButton("Render room", "Ready", 0);
  await initWasm();
  scheduleUrlSync();
}

async function initWasm() {
  try {
    setEngineStatus("Loading WASM", "loading");
    const readyPromise = waitForWorkerReady();
    renderWorker = createRenderWorker();
    await readyPromise;
    setEngineStatus("WASM ready", "ready");
    setRenderBadge("Waiting for render", "ready");
  } catch (error) {
    setEngineStatus("WASM failed", "error");
    setRenderBadge("Engine error", "error");
    refs.renderLog.textContent = `${error}`;
  }
}

function createRenderWorker() {
  const worker = new Worker("worker.js");
  worker.addEventListener("message", handleWorkerMessage);
  worker.addEventListener("error", (event) => {
    if (workerReadyReject) {
      workerReadyReject(event.error ?? new Error(event.message));
      workerReadyReject = null;
      workerReadyResolve = null;
    }
    setEngineStatus("WASM failed", "error");
    refs.renderLog.textContent = `${event.message}`;
  });
  worker.postMessage({ type: "init" });
  return worker;
}

function waitForWorkerReady() {
  if (renderWorkerReady) {
    return Promise.resolve();
  }

  return new Promise((resolve, reject) => {
    workerReadyResolve = resolve;
    workerReadyReject = reject;
  });
}

function handleWorkerMessage(event) {
  const { type, requestId, result, stage, percent, message } = event.data ?? {};

  if (type === "ready") {
    renderWorkerReady = true;
    if (workerReadyResolve) {
      workerReadyResolve();
      workerReadyResolve = null;
      workerReadyReject = null;
    }
    return;
  }

  if (type === "progress") {
    if (!renderSession || requestId !== renderSession.requestId) {
      return;
    }

    updateRenderSessionProgress({
      stage: stage ?? "working",
      percent: Number.isFinite(percent) ? percent : renderSession.progress,
      message: message ?? stage ?? "Working",
    });
    return;
  }

  if (type === "cancelled") {
    if (!renderSession || requestId !== renderSession.requestId) {
      return;
    }

    finalizeRenderSession("Render canceled", "ready");
    refs.renderLog.textContent = "Render canceled.";
    setRenderBadge("Render canceled", "ready");
    return;
  }

  if (type === "error") {
    if (renderSession && requestId === renderSession.requestId) {
      finalizeRenderSession("Render failed", "error");
    }
    setRenderBadge("Render failed", "error");
    refs.renderLog.textContent = `${message ?? result ?? "Render failed"}`;
    return;
  }

  if (type === "result") {
    if (!renderSession || requestId !== renderSession.requestId) {
      return;
    }

    if (renderSession.cancelRequested) {
      finalizeRenderSession("Render canceled", "ready");
      setRenderBadge("Render canceled", "ready");
      return;
    }

    const samples = result?.samples ? new Float32Array(result.samples) : new Float32Array();
    const wavBytes = result?.wavBytes ? new Uint8Array(result.wavBytes) : new Uint8Array();
    lastRender = {
      ...result,
      samples,
      wavBytes,
    };

    setRenderBadge("Render complete", "ready");
    updateMetrics(lastRender);
    updateAudio(lastRender.wavBytes);
    updateRenderLog(lastRender);
    drawWaveform(refs.waveformCanvas, samples, {
      irView: state.irView,
      renderMode: state.render.mode,
      durationSeconds: state.render.durationSeconds,
      crossoverTimeSeconds: state.render.crossoverTimeSeconds,
    });
    refs.downloadWav.disabled = false;
    finalizeRenderSession("Render room", "ready");
    setRenderBadge("Render complete", "ready");
    scheduleUrlSync();
  }
}

function bindEvents() {
  refs.themeToggle?.addEventListener("click", cycleTheme);

  refs.resetScene.addEventListener("click", () => {
    if (isRenderActive()) {
      requestRenderCancel();
    }
    copyState(DEFAULT_STATE, state);
    syncFormFromState();
    updateMaterialSummary();
    updateSceneView();
    drawWaveform(refs.waveformCanvas, lastRender?.samples ?? null, {
      irView: state.irView,
      renderMode: state.render.mode,
      durationSeconds: state.render.durationSeconds,
      crossoverTimeSeconds: state.render.crossoverTimeSeconds,
    });
    setRenderBadge("Waiting for render", "ready");
    scheduleUrlSync();
  });

  refs.roomPreset.addEventListener("change", () => {
    applyRoomPreset(refs.roomPreset.value);
  });

  refs.materialPreset.addEventListener("change", () => {
    applyMaterialPreset(refs.materialPreset.value);
  });

  refs.sourceDirectivity.addEventListener("change", () => {
    state.source.directivity = refs.sourceDirectivity.value;
    state.roomPreset = "custom";
    syncDirectivityAvailability();
    updateSceneView();
    syncPresetSelects();
    scheduleUrlSync();
  });

  bindNumber(refs.roomWidth, (value) => {
    state.room.width = clamp(value, 2.5, 16);
    state.room.kind = "shoebox";
    state.room.mesh = null;
    state.roomPreset = "custom";
    normalizeSpatialState();
    syncPresetSelects();
  });
  bindNumber(refs.roomDepth, (value) => {
    state.room.depth = clamp(value, 2.5, 16);
    state.room.kind = "shoebox";
    state.room.mesh = null;
    state.roomPreset = "custom";
    normalizeSpatialState();
    syncPresetSelects();
  });
  bindNumber(refs.roomHeight, (value) => {
    state.room.height = clamp(value, 2.2, 7);
    state.room.kind = "shoebox";
    state.room.mesh = null;
    state.roomPreset = "custom";
    normalizeSpatialState();
    syncPresetSelects();
  });

  bindNumber(refs.sourceX, (value) => {
    state.source.x = clamp(
      value,
      POSITION_MARGIN,
      state.room.width - POSITION_MARGIN,
    );
    state.roomPreset = "custom";
    syncPresetSelects();
  });
  bindNumber(refs.sourceY, (value) => {
    state.source.y = clamp(
      value,
      POSITION_MARGIN,
      state.room.depth - POSITION_MARGIN,
    );
    state.roomPreset = "custom";
    syncPresetSelects();
  });
  bindNumber(refs.sourceZ, (value) => {
    state.source.z = clamp(
      value,
      POSITION_MARGIN,
      state.room.height - POSITION_MARGIN,
    );
    state.roomPreset = "custom";
    syncPresetSelects();
  });
  bindNumber(refs.receiverX, (value) => {
    state.receiver.x = clamp(
      value,
      POSITION_MARGIN,
      state.room.width - POSITION_MARGIN,
    );
    state.roomPreset = "custom";
    syncPresetSelects();
  });
  bindNumber(refs.receiverY, (value) => {
    state.receiver.y = clamp(
      value,
      POSITION_MARGIN,
      state.room.depth - POSITION_MARGIN,
    );
    state.roomPreset = "custom";
    syncPresetSelects();
  });
  bindNumber(refs.receiverZ, (value) => {
    state.receiver.z = clamp(
      value,
      POSITION_MARGIN,
      state.room.height - POSITION_MARGIN,
    );
    state.roomPreset = "custom";
    syncPresetSelects();
  });

  bindRange(refs.sourceAzimuth, refs.sourceAzimuthValue, (value) => {
    state.source.azimuthDegrees = value;
    state.roomPreset = "custom";
    scheduleUrlSync();
    return `${Math.round(value)}°`;
  });
  bindRange(refs.sourceFocus, refs.sourceFocusValue, (value) => {
    state.source.cardioidOrder = value;
    state.roomPreset = "custom";
    scheduleUrlSync();
    return value.toFixed(2);
  });
  bindRange(refs.sourceGain, refs.sourceGainValue, (value) => {
    state.source.gainDb = value;
    state.roomPreset = "custom";
    scheduleUrlSync();
    return `${value.toFixed(1)} dB`;
  });
  bindRange(refs.renderRays, refs.renderRaysValue, (value) => {
    state.render.numRays = roundToStep(value, 128);
    refs.renderRays.value = String(state.render.numRays);
    scheduleUrlSync();
    return `${state.render.numRays.toLocaleString()} rays`;
  });
  bindRange(refs.renderOrder, refs.renderOrderValue, (value) => {
    state.render.maxOrder = Math.round(value);
    scheduleUrlSync();
    return `order ${state.render.maxOrder}`;
  });
  bindRange(refs.renderDuration, refs.renderDurationValue, (value) => {
    state.render.durationSeconds = value;
    state.render.crossoverTimeSeconds = clamp(
      state.render.crossoverTimeSeconds,
      0.03,
      state.render.durationSeconds * 0.85,
    );
    syncFormFromState();
    scheduleUrlSync();
    return `${value.toFixed(2)} s`;
  });
  bindRange(refs.renderCrossover, refs.renderCrossoverValue, (value) => {
    state.render.crossoverTimeSeconds = clamp(
      value,
      0.03,
      state.render.durationSeconds * 0.85,
    );
    scheduleUrlSync();
    return `${state.render.crossoverTimeSeconds.toFixed(2)} s`;
  });

  refs.renderWindow.addEventListener("change", () => {
    state.render.crossoverWindow = refs.renderWindow.value;
    scheduleUrlSync();
  });

  refs.sceneReflections.addEventListener("input", () => {
    const value = Number(refs.sceneReflections.value);
    if (Number.isFinite(value)) {
      state.reflections = clampInt(Math.round(value), 0, 6);
    }
    refs.sceneReflections.value = String(state.reflections);
    updateSceneView();
    scheduleUrlSync();
  });

  [
    [refs.wallWest, "west"],
    [refs.wallEast, "east"],
    [refs.wallSouth, "south"],
    [refs.wallNorth, "north"],
    [refs.wallFloor, "floor"],
    [refs.wallCeiling, "ceiling"],
  ].forEach(([element, key]) => {
    element.addEventListener("change", () => {
      state.materials[key] = element.value;
      state.materialPreset = "custom";
      updateMaterialSummary();
      updateSceneView();
      syncPresetSelects();
      scheduleUrlSync();
    });
  });

  refs.renderModeButtons.forEach((button) => {
    button.addEventListener("click", () => {
      state.render.mode = button.dataset.mode;
      syncModeButtons();
      drawWaveform(refs.waveformCanvas, lastRender?.samples ?? null, {
        irView: state.irView,
        renderMode: state.render.mode,
        durationSeconds: state.render.durationSeconds,
        crossoverTimeSeconds: state.render.crossoverTimeSeconds,
      });
      scheduleUrlSync();
    });
  });

  refs.irViewButtons.forEach((button) => {
    button.addEventListener("click", () => {
      state.irView = button.dataset.view;
      syncIrViewButtons();
      drawWaveform(refs.waveformCanvas, lastRender?.samples ?? null, {
        irView: state.irView,
        renderMode: state.render.mode,
        durationSeconds: state.render.durationSeconds,
        crossoverTimeSeconds: state.render.crossoverTimeSeconds,
      });
      scheduleUrlSync();
    });
  });

  refs.renderScene.addEventListener("click", onRenderButtonClick);
  refs.downloadWav.addEventListener("click", () => {
    if (!lastRender?.wavBytes) {
      return;
    }
    downloadBytes(lastRender.wavBytes, "algo-acoustics-demo.wav", "audio/wav");
  });

  refs.drySource.addEventListener("change", () => {
    scheduleUrlSync();
  });

  refs.wetMix.addEventListener("input", () => {
    syncAuralizationControls();
  });

  refs.audioGain.addEventListener("input", () => {
    syncAuralizationControls();
  });

  refs.playAuralization.addEventListener("click", () => {
    if (currentAuralizationPlaying) {
      stopAuralizationPlayback();
    } else {
      void startAuralizationPlayback();
    }
  });

  refs.stopAuralization.addEventListener("click", stopAuralizationPlayback);

  refs.exportAuralization.addEventListener("click", () => {
    void exportAuralizedWav();
  });

  window.addEventListener("resize", () => {
    sceneView?.resize();
    drawWaveform(refs.waveformCanvas, lastRender?.samples ?? null, {
      irView: state.irView,
      renderMode: state.render.mode,
      durationSeconds: state.render.durationSeconds,
      crossoverTimeSeconds: state.render.crossoverTimeSeconds,
    });
  });
}

function populateMaterialSelects() {
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

function syncFormFromState() {
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

function syncDirectivityAvailability() {
  const isCardioid = state.source.directivity === "cardioid";
  refs.sourceAzimuth.disabled = !isCardioid;
  refs.sourceFocus.disabled = !isCardioid;
}

function syncRoomModeAvailability() {
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

function syncModeButtons() {
  refs.renderModeButtons.forEach((button) => {
    button.classList.toggle(
      "is-active",
      button.dataset.mode === state.render.mode,
    );
  });
}

function syncIrViewButtons() {
  refs.irViewButtons.forEach((button) => {
    button.classList.toggle("is-active", button.dataset.view === state.irView);
  });
}

function syncPositionConstraints() {
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

function normalizeSpatialState() {
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
  syncFormFromState();
  updateSceneView();
  scheduleUrlSync();
}

function onRenderButtonClick() {
  if (isRenderActive()) {
    requestRenderCancel();
    return;
  }

  startRender();
}

function startRender() {
  if (!renderWorker) {
    setRenderBadge("Engine unavailable", "error");
    refs.renderLog.textContent = "WASM worker is not ready.";
    return;
  }

  if (renderSession) {
    return;
  }

  const requestId = ++currentRenderId;
  const request = buildRequest();
  renderSession = {
    requestId,
    startedAt: performance.now(),
    progress: 0,
    stage: "starting",
    cancelRequested: false,
  };

  refs.renderScene.disabled = false;
  refs.downloadWav.disabled = true;
  setRenderBadge("Rendering…", "busy");
  refs.renderLog.textContent = "Rendering hybrid impulse response in the browser…";
  updateRenderButton("Cancel", "Starting render…", 0);
  scheduleRenderButtonAnimation();
  renderWorker.postMessage({
    type: "render",
    requestId,
    payload: request,
  });
}

function requestRenderCancel() {
  if (!renderSession || renderSession.cancelRequested) {
    return;
  }

  renderSession.cancelRequested = true;
  setRenderBadge("Cancel requested", "busy");
  updateRenderButton("Cancel", "Cancel requested", renderSession.progress);
  renderWorker?.postMessage({ type: "cancel", requestId: renderSession.requestId });
}

function isRenderActive() {
  return renderSession !== null;
}

function updateRenderSessionProgress({ stage, percent, message }) {
  if (!renderSession) {
    return;
  }

  renderSession.stage = stage;
  renderSession.progress = clamp(percent, 0, 99);
  updateRenderButton("Cancel", message, renderSession.progress);
  setRenderBadge(`Rendering… ${Math.round(renderSession.progress)}%`, "busy");
}

function finalizeRenderSession(title, badgeState) {
  if (renderButtonAnimation) {
    cancelAnimationFrame(renderButtonAnimation);
    renderButtonAnimation = 0;
  }

  renderSession = null;
  updateRenderButton(title, "Ready", 0);
  refs.renderScene.disabled = false;
  setRenderBadge(title, badgeState);
}

function scheduleRenderButtonAnimation() {
  if (renderButtonAnimation) {
    return;
  }

  const tick = () => {
    if (!renderSession) {
      renderButtonAnimation = 0;
      return;
    }

    const elapsed = performance.now() - renderSession.startedAt;
    const drift = Math.min(8, Math.floor(elapsed / 650));
    const shownProgress = clamp(renderSession.progress + drift, 0, 99);
    updateRenderButton(
      renderSession.cancelRequested ? "Canceling…" : "Cancel",
      renderSession.stage,
      shownProgress,
    );
    renderButtonAnimation = requestAnimationFrame(tick);
  };

  renderButtonAnimation = requestAnimationFrame(tick);
}

function updateRenderButton(title, subtitle, progress) {
  refs.renderScene.style.setProperty("--render-progress", `${progress}%`);
  if (refs.renderButtonTitle) {
    refs.renderButtonTitle.textContent = title;
  } else {
    refs.renderScene.textContent = title;
  }
  if (refs.renderButtonSubtitle) {
    refs.renderButtonSubtitle.textContent = subtitle;
  }
}

function updateMetrics(result) {
  refs.metricFirstArrival.textContent = `${result.firstArrivalMs.toFixed(1)} ms`;
  refs.metricPeak.textContent = result.peakAmplitude.toFixed(3);
  refs.metricEvents.textContent = result.earlyEventCount.toLocaleString();
  refs.metricRenderTime.textContent = `${Math.round(result.renderMs)} ms`;
}

function updateRenderLog(result) {
  refs.renderLog.textContent = [
    `Mode: ${state.render.mode}`,
    `Room: ${state.room.width.toFixed(1)} × ${state.room.depth.toFixed(1)} × ${state.room.height.toFixed(1)} m`,
    `Room preset: ${state.roomPreset}`,
    `Material preset: ${state.materialPreset}`,
    `Source: ${formatVec(state.source.x, state.source.y, state.source.z)} | ${state.source.directivity}`,
    `Receiver: ${formatVec(state.receiver.x, state.receiver.y, state.receiver.z)}`,
    `Rays: ${state.render.numRays.toLocaleString()} | Max order: ${state.render.maxOrder}`,
    `Duration: ${state.render.durationSeconds.toFixed(2)} s | Crossover: ${state.render.crossoverTimeSeconds.toFixed(2)} s`,
    `Peak amplitude: ${result.peakAmplitude.toFixed(4)} | RMS: ${result.rmsAmplitude.toFixed(4)}`,
  ].join("\n");
}

function updateAudio(wavBytes) {
  if (lastAudioURL) {
    URL.revokeObjectURL(lastAudioURL);
  }
  const blob = new Blob([wavBytes], { type: "audio/wav" });
  lastAudioURL = URL.createObjectURL(blob);
  refs.audioPlayer.src = lastAudioURL;
  lastAuralizedWav = wavBytes.slice ? wavBytes.slice() : new Uint8Array(wavBytes);
  currentIRBuffer = null;
  updateAuralizationStatus("IR ready", "ready");
  syncAuralizationControls();
}

function updateAuralizationStatus(label, stateName) {
  refs.audioStatus.textContent = label;
  refs.audioStatus.className = `status-pill is-${stateName}`;
}

function syncAuralizationControls() {
  const wetMix = clamp(Number(refs.wetMix.value || 0.72), 0, 1);
  const gainDb = clamp(Number(refs.audioGain.value || -0.5), -12, 6);
  refs.wetMix.value = String(wetMix);
  refs.wetMixValue.textContent = `${Math.round(wetMix * 100)}%`;
  refs.audioGain.value = String(gainDb);
  refs.audioGainValue.textContent = `${gainDb.toFixed(1)} dB`;
  refs.playAuralization.disabled = !lastAuralizedWav || currentAuralizationPlaying;
  refs.stopAuralization.disabled = !currentAuralizationPlaying;
  refs.exportAuralization.disabled = !lastAuralizedWav;
  if (!lastAuralizedWav) {
    updateAuralizationStatus("Waiting for IR", "loading");
  } else if (currentAuralizationPlaying) {
    updateAuralizationStatus("Playing", "busy");
  } else {
    updateAuralizationStatus("Ready", "ready");
  }
}

async function ensureAudioContext() {
  if (!audioContext || audioContext.state === "closed") {
    audioContext = new AudioContext({ sampleRate: 48000 });
  }

  if (audioContext.state === "suspended") {
    await audioContext.resume();
  }

  return audioContext;
}

async function ensureIrAudioBuffer() {
  if (currentIRBuffer) {
    return currentIRBuffer;
  }

  if (!lastAuralizedWav) {
    throw new Error("No rendered IR available");
  }

  const context = await ensureAudioContext();
  const arrayBuffer = lastAuralizedWav.buffer.slice(
    lastAuralizedWav.byteOffset,
    lastAuralizedWav.byteOffset + lastAuralizedWav.byteLength,
  );
  currentIRBuffer = await context.decodeAudioData(arrayBuffer);
  return currentIRBuffer;
}

function buildDrySourceBuffer(context, preset) {
  return audioModule.buildDrySourceBuffer(context, preset);
}

async function startAuralizationPlayback() {
  if (!lastAuralizedWav) {
    updateAuralizationStatus("Waiting for IR", "loading");
    return;
  }

  const context = await ensureAudioContext();
  const irBuffer = await ensureIrAudioBuffer();
  stopAuralizationPlayback();

  const drySource = context.createBufferSource();
  drySource.buffer = buildDrySourceBuffer(context, refs.drySource.value);
  drySource.loop = false;

  const convolver = context.createConvolver();
  convolver.buffer = irBuffer;

  const dryGain = context.createGain();
  const wetGain = context.createGain();
  const outputGain = context.createGain();

  const wetMix = clamp(Number(refs.wetMix.value || 0.72), 0, 1);
  const gainDb = clamp(Number(refs.audioGain.value || -0.5), -12, 6);
  dryGain.gain.value = Math.sqrt(1 - wetMix);
  wetGain.gain.value = Math.sqrt(wetMix);
  outputGain.gain.value = dbToLinear(gainDb);

  drySource.connect(dryGain);
  drySource.connect(convolver);
  convolver.connect(wetGain);
  dryGain.connect(outputGain);
  wetGain.connect(outputGain);
  outputGain.connect(context.destination);

  currentAuralizationNodes = {
    drySource,
    convolver,
    dryGain,
    wetGain,
    outputGain,
  };
  currentAuralizationPlaying = true;
  syncAuralizationControls();
  updateAuralizationStatus("Playing", "busy");

  drySource.start();
  currentAuralizationStopTimer = window.setTimeout(() => {
    stopAuralizationPlayback();
  }, Math.ceil(drySource.buffer.duration * 1000) + 1200);

  drySource.onended = () => {
    if (currentAuralizationPlaying) {
      stopAuralizationPlayback();
    }
  };
}

function stopAuralizationPlayback() {
  if (currentAuralizationStopTimer) {
    clearTimeout(currentAuralizationStopTimer);
    currentAuralizationStopTimer = 0;
  }

  if (currentAuralizationNodes?.drySource) {
    try {
      currentAuralizationNodes.drySource.stop();
    } catch (error) {
      void error;
    }
  }

  if (currentAuralizationNodes) {
    for (const node of [
      currentAuralizationNodes.drySource,
      currentAuralizationNodes.convolver,
      currentAuralizationNodes.dryGain,
      currentAuralizationNodes.wetGain,
      currentAuralizationNodes.outputGain,
    ]) {
      try {
        node?.disconnect?.();
      } catch (error) {
        void error;
      }
    }
  }

  currentAuralizationNodes = null;
  currentAuralizationPlaying = false;
  syncAuralizationControls();
}

async function exportAuralizedWav() {
  if (!lastAuralizedWav) {
    return;
  }

  const rendered = await renderAuralizationOffline();
  const wavBytes = encodeAudioBufferToWav(rendered);
  downloadBytes(wavBytes, "algo-acoustics-auralization.wav", "audio/wav");
}

async function renderAuralizationOffline() {
  const baseContext = await ensureAudioContext();
  const irBuffer = await ensureIrAudioBuffer();
  const dryBuffer = buildDrySourceBuffer(baseContext, refs.drySource.value);
  const offline = new OfflineAudioContext(
    1,
    Math.ceil((dryBuffer.duration + irBuffer.duration + 1.2) * baseContext.sampleRate),
    baseContext.sampleRate,
  );

  const drySource = offline.createBufferSource();
  drySource.buffer = dryBuffer;
  const convolver = offline.createConvolver();
  convolver.buffer = irBuffer;
  const dryGain = offline.createGain();
  const wetGain = offline.createGain();
  const outputGain = offline.createGain();
  const wetMix = clamp(Number(refs.wetMix.value || 0.72), 0, 1);
  const gainDb = clamp(Number(refs.audioGain.value || -0.5), -12, 6);
  dryGain.gain.value = Math.sqrt(1 - wetMix);
  wetGain.gain.value = Math.sqrt(wetMix);
  outputGain.gain.value = dbToLinear(gainDb);

  drySource.connect(dryGain);
  drySource.connect(convolver);
  convolver.connect(wetGain);
  dryGain.connect(outputGain);
  wetGain.connect(outputGain);
  outputGain.connect(offline.destination);
  drySource.start();

  return offline.startRendering();
}

function encodeAudioBufferToWav(audioBuffer) {
  return audioModule.encodeAudioBufferToWav(audioBuffer);
}

function populatePresetSelects() {
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

function updateMaterialSummary() {
  refs.materialSummary.innerHTML = Object.entries(state.materials)
    .map(([wall, materialKey]) => {
      const material = MATERIALS[materialKey];
      const absorptionAverage = average(material.absorption);
      return `
        <div class="material-chip">
          <span class="material-swatch" style="background:${material.color}"></span>
          <strong>${capitalize(wall)}</strong>
          <span>${material.label}</span>
          <span>avg α ${absorptionAverage.toFixed(2)}</span>
        </div>
      `;
    })
    .join("");
}

function syncPresetSelects() {
  refs.roomPreset.value = state.roomPreset in ROOM_PRESETS ? state.roomPreset : "custom";
  refs.materialPreset.value = state.materialPreset in MATERIAL_PRESETS ? state.materialPreset : "custom";
}

function applyRoomPreset(presetName, options = {}) {
  const preset = ROOM_PRESETS[presetName] ?? ROOM_PRESETS.custom;
  state.roomPreset = presetName in ROOM_PRESETS ? presetName : "custom";

  state.room.kind = preset.kind ?? "shoebox";
  state.room.mesh = preset.mesh ? structuredClone(preset.mesh) : null;

  if (preset.room) {
    state.room.width = preset.room.width;
    state.room.depth = preset.room.depth;
    state.room.height = preset.room.height;
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
  drawWaveform(refs.waveformCanvas, lastRender?.samples ?? null, {
    irView: state.irView,
    renderMode: state.render.mode,
    durationSeconds: state.render.durationSeconds,
    crossoverTimeSeconds: state.render.crossoverTimeSeconds,
  });
  if (!options.skipUrlSync) {
    scheduleUrlSync();
  }
}

function applyMaterialPreset(presetName, options = {}) {
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

function loadStateFromUrl() {
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

function applySerializedState(input) {
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
    if (typeof input.reflections === "number" && Number.isFinite(input.reflections)) {
      state.reflections = input.reflections;
    }

    if (typeof input.irView === "string" && input.irView) {
      state.irView = input.irView;
    }
  }

  normalizeSceneState();
  syncFormFromState();
  updateMaterialSummary();
  updateSceneView();
  drawWaveform(refs.waveformCanvas, lastRender?.samples ?? null, {
    irView: state.irView,
    renderMode: state.render.mode,
    durationSeconds: state.render.durationSeconds,
    crossoverTimeSeconds: state.render.crossoverTimeSeconds,
  });
}

function assignRoomState(room) {
  if (!room || typeof room !== "object") {
    return;
  }

  if (room.kind === "mesh" || room.kind === "shoebox") {
    state.room.kind = room.kind;
  }

  if (typeof room.width === "number" && room.width > 0) {
    state.room.width = room.width;
  }
  if (typeof room.depth === "number" && room.depth > 0) {
    state.room.depth = room.depth;
  }
  if (typeof room.height === "number" && room.height > 0) {
    state.room.height = room.height;
  }

  if (room.mesh && typeof room.mesh === "object") {
    state.room.mesh = structuredClone(room.mesh);
  }
}

function assignMaterialState(materials) {
  if (!materials || typeof materials !== "object") {
    return;
  }

  for (const key of ["west", "east", "south", "north", "floor", "ceiling"]) {
    if (typeof materials[key] === "string" && materials[key] in MATERIALS) {
      state.materials[key] = materials[key];
    }
  }
}

function assignSourceState(source) {
  if (!source || typeof source !== "object") {
    return;
  }

  for (const key of ["x", "y", "z", "gainDb", "azimuthDegrees", "cardioidOrder"]) {
    if (typeof source[key] === "number" && Number.isFinite(source[key])) {
      state.source[key] = source[key];
    }
  }
  if (typeof source.directivity === "string") {
    state.source.directivity = source.directivity;
  }
}

function assignReceiverState(receiver) {
  if (!receiver || typeof receiver !== "object") {
    return;
  }

  for (const key of ["x", "y", "z"]) {
    if (typeof receiver[key] === "number" && Number.isFinite(receiver[key])) {
      state.receiver[key] = receiver[key];
    }
  }
}

function assignRenderState(render) {
  if (!render || typeof render !== "object") {
    return;
  }

  for (const key of ["maxOrder", "numRays"]) {
    if (typeof render[key] === "number" && Number.isFinite(render[key])) {
      state.render[key] = render[key];
    }
  }
  for (const key of ["durationSeconds", "crossoverTimeSeconds", "crossoverWindowAlpha"]) {
    if (typeof render[key] === "number" && Number.isFinite(render[key])) {
      state.render[key] = render[key];
    }
  }
  if (typeof render.mode === "string") {
    state.render.mode = render.mode;
  }
  if (typeof render.crossoverWindow === "string") {
    state.render.crossoverWindow = render.crossoverWindow;
  }
}

function normalizeSceneState() {
  state.room.width = clamp(state.room.width, 2.5, 16);
  state.room.depth = clamp(state.room.depth, 2.5, 16);
  state.room.height = clamp(state.room.height, 2.2, 7);

  state.source.directivity = normalizeDirectivity(state.source.directivity, DEFAULT_STATE.source.directivity);
  state.source.gainDb = clamp(state.source.gainDb, -24, 12);
  state.source.azimuthDegrees = clamp(state.source.azimuthDegrees, -180, 180);
  state.source.cardioidOrder = clamp(state.source.cardioidOrder, 0.25, 2.5);

  state.render.mode = ["early", "late", "hybrid"].includes(state.render.mode)
    ? state.render.mode
    : DEFAULT_STATE.render.mode;
  state.render.maxOrder = clampInt(Math.round(state.render.maxOrder), 1, 12);
  state.render.numRays = clampInt(Math.round(state.render.numRays), 128, 16384);
  state.render.durationSeconds = clamp(state.render.durationSeconds, 0.25, 3);
  state.render.crossoverTimeSeconds = clamp(
    state.render.crossoverTimeSeconds,
    0.03,
    state.render.durationSeconds * 0.85,
  );
  state.render.crossoverWindow = normalizeCrossoverWindow(
    state.render.crossoverWindow,
    DEFAULT_STATE.render.crossoverWindow,
  );

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

  state.roomPreset = state.roomPreset in ROOM_PRESETS ? state.roomPreset : "custom";
  state.materialPreset = state.materialPreset in MATERIAL_PRESETS ? state.materialPreset : "custom";
  state.reflections = clampInt(Math.round(state.reflections), 0, 6);

  if (state.room.kind === "mesh" && !state.room.mesh) {
    state.room.kind = "shoebox";
  }
  if (state.room.kind !== "mesh") {
    state.room.mesh = null;
  }
}

function normalizeCrossoverWindow(name, fallback) {
  const trimmed = String(name ?? "").trim();
  if (
    trimmed === "hann" ||
    trimmed === "hamming" ||
    trimmed === "blackman" ||
    trimmed === "blackman-harris" ||
    trimmed === "flattop"
  ) {
    return trimmed;
  }

  return fallback;
}

function encodeStateForUrl() {
  const payload = {
    roomPreset: state.roomPreset,
    materialPreset: state.materialPreset,
    room: state.room,
    materials: state.materials,
    source: state.source,
    receiver: state.receiver,
    render: state.render,
    irView: state.irView,
    reflections: state.reflections,
  };
  return base64UrlEncode(JSON.stringify(payload));
}

function decodeStateFromUrl(encoded) {
  const raw = base64UrlDecode(encoded);
  return JSON.parse(raw);
}

function scheduleUrlSync() {
  if (urlSyncTimer) {
    clearTimeout(urlSyncTimer);
  }

  urlSyncTimer = window.setTimeout(syncUrlState, 120);
}

function syncUrlState() {
  urlSyncTimer = 0;
  const encoded = encodeStateForUrl();
  if (latestEncodedState === encoded) {
    return;
  }

  latestEncodedState = encoded;
  const url = new URL(window.location.href);
  url.searchParams.set("scene", encoded);
  window.history.replaceState(null, "", `${url.pathname}?${url.searchParams.toString()}${url.hash}`);
}

function base64UrlEncode(value) {
  const bytes = new TextEncoder().encode(value);
  let binary = "";
  bytes.forEach((byte) => {
    binary += String.fromCharCode(byte);
  });
  return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/g, "");
}

function base64UrlDecode(value) {
  let normalized = value.replace(/-/g, "+").replace(/_/g, "/");
  while (normalized.length % 4 !== 0) {
    normalized += "=";
  }
  const binary = atob(normalized);
  const bytes = Uint8Array.from(binary, (char) => char.charCodeAt(0));
  return new TextDecoder().decode(bytes);
}

function buildRequest() {
  return {
    roomPreset: state.roomPreset,
    materialPreset: state.materialPreset,
    room: state.room,
    materials: state.materials,
    source: state.source,
    receiver: state.receiver,
    render: state.render,
    reflections: state.reflections,
  };
}

function getSystemTheme() {
  return window.matchMedia("(prefers-color-scheme: dark)").matches
    ? "dark"
    : "light";
}

function applyTheme(mode) {
  const theme = mode === "auto" ? getSystemTheme() : mode;
  if (theme === "dark") {
    document.documentElement.setAttribute("data-theme", "dark");
  } else {
    document.documentElement.removeAttribute("data-theme");
  }

  updateThemeToggle();
  applySceneTheme();
  updateSceneView();
  drawWaveform(refs.waveformCanvas, lastRender?.samples ?? null, {
    irView: state.irView,
    renderMode: state.render.mode,
    durationSeconds: state.render.durationSeconds,
    crossoverTimeSeconds: state.render.crossoverTimeSeconds,
  });
}

function updateThemeToggle() {
  if (!refs.themeToggle) {
    return;
  }

  const label = refs.themeToggle.querySelector(".theme-toggle-label");
  const icon = refs.themeToggle.querySelector(".theme-toggle-icon");
  const labels = { auto: "Auto", light: "Light", dark: "Dark" };
  const text = labels[currentThemeMode] ?? "Auto";

  if (label) {
    label.textContent = text;
  }
  if (icon) {
    icon.innerHTML = THEME_ICONS[currentThemeMode] ?? THEME_ICONS.auto;
  }

  refs.themeToggle.setAttribute("aria-label", `Theme: ${text}`);
  refs.themeToggle.title = `Theme: ${text}`;
}

function cycleTheme() {
  const currentIndex = THEME_MODES.indexOf(currentThemeMode);
  const nextIndex = (currentIndex + 1) % THEME_MODES.length;
  currentThemeMode = THEME_MODES[nextIndex];
  localStorage.setItem(THEME_KEY, currentThemeMode);
  applyTheme(currentThemeMode);
}

function initTheme() {
  const savedTheme = localStorage.getItem(THEME_KEY);
  if (savedTheme && THEME_MODES.includes(savedTheme)) {
    currentThemeMode = savedTheme;
  }

  mediaThemeQuery = window.matchMedia("(prefers-color-scheme: dark)");
  mediaThemeQuery.addEventListener("change", () => {
    if (currentThemeMode === "auto") {
      applyTheme("auto");
    }
  });

  applyTheme(currentThemeMode);
}

function scenePalette() {
  return sceneModule.scenePalette();
}

function waveformPalette(context, width) {
  return audioModule.waveformPalette(context, width);
}

function applySceneTheme() {
  sceneModule.applySceneTheme();
}

function createSceneView(canvas) {
  sceneModule.setSceneContext(state);
  const view = sceneModule.createSceneView(canvas);
  sceneModule.setSceneView(view);
  sceneModule.setSceneContext(state, view);
  return view;
}

function bindSceneInteractions(canvas) {
  sceneModule.setSceneChangeHandler(() => {
    syncFormFromState();
    updateSceneView();
    scheduleUrlSync();
  });
  sceneModule.bindSceneInteractions(canvas);
}

function updateSceneView() {
  sceneModule.updateSceneView();
}

function drawWaveform(canvas, samples = null, state = {}) {
  audioModule.drawWaveform(canvas, samples, state);
}

function bindRange(input, output, formatter) {
  const handler = () => {
    const value = Number(input.value);
    output.textContent = formatter(value);
    syncPresetSelects();
    updateSceneView();
    scheduleUrlSync();
  };
  input.addEventListener("input", handler);
}

function bindNumber(input, onChange) {
  input.addEventListener("input", () => {
    onChange(Number(input.value));
    syncFormFromState();
    updateMaterialSummary();
    updateSceneView();
    scheduleUrlSync();
  });
}

function setEngineStatus(label, stateName) {
  refs.engineStatus.textContent = label;
  refs.engineStatus.className = `status-pill is-${stateName}`;
}

function setRenderBadge(label, stateName) {
  refs.renderBadge.textContent = label;
  refs.renderBadge.className = `status-pill is-${stateName}`;
}
