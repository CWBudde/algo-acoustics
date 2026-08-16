import * as presets from "./app-presets.js";
import * as appState from "./app-state.js";
import * as sceneModule from "./app-scene.js";
import * as audioModule from "./app-audio.js";
import {
  AuralizationEngine,
  portalCrossfadeWeight,
} from "./auralization-engine.mjs";
import { DRY_AUDIO_SOURCES, DryAudioLoader } from "./audio-samples.mjs";
import {
  clamp,
  clampInt,
  formatVec,
  copyState,
  capitalize,
  roundToStep,
  downloadBytes,
  dbToLinear,
} from "./app-utils.js";
import { normalizeReflectionOrder } from "./reflection-preview.mjs";
import { applyRangeBounds, rangeBoundsFromLimits } from "./demo-limits.mjs";
import {
  describeEstimatedRt60,
  describePreviewTier,
  findTimeoutWarning,
  formatEstimatedRt60,
  readPreviewTier,
  readStatisticalTier,
} from "./render-tiers.mjs";
import { RenderWorkerController } from "./render-worker-controller.mjs";

const MATERIALS = presets.MATERIALS;
const PORTAL_MATERIALS = presets.PORTAL_MATERIALS;
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
  portal: {
    enabled: false,
    aperture: 0,
    rootOrder: 2,
    material: "woodenDoor",
    receiverRoom: { width: 6.4, depth: 4.8, height: 2.9 },
    opening: { width: 1.2, height: 2.1, bottom: 0 },
  },
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
let renderWorkerController = null;
let renderSession = null;
let renderButtonAnimation = 0;
let urlSyncTimer = 0;
let latestEncodedState = null;
let audioContext = null;
let currentIRBuffer = null;
let currentIRDecode = null;
let currentPortalBuffers = null;
let currentPortalDecode = null;
let currentIRVersion = 0;
let auralizationEngine = null;
let auralizationStarting = false;
let currentAuralizationPlaying = false;
let auralizationStatusOverride = null;
let audioTransitionTimer = 0;
let lastAuralizedWav = null;
let lastPortalResponseWavs = null;
const dryAudioLoader = new DryAudioLoader();
let waveformZoom = 1;
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
  portalEnabled: document.getElementById("portal-enabled"),
  portalAperture: document.getElementById("portal-aperture"),
  portalApertureValue: document.getElementById("portal-aperture-value"),
  portalMaterial: document.getElementById("portal-material"),
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
  splHeatmap: document.getElementById("spl-heatmap"),
  splHeatmapLegend: document.getElementById("spl-heatmap-legend"),
  splHeatmapMin: document.getElementById("spl-heatmap-min"),
  splHeatmapMax: document.getElementById("spl-heatmap-max"),
  renderModeButtons: Array.from(
    document.querySelectorAll("#render-mode-switch .mode-button"),
  ),
  renderScene: document.getElementById("render-scene"),
  renderButtonTitle: document.querySelector("#render-scene .button-title"),
  renderButtonSubtitle: document.querySelector(
    "#render-scene .button-subtitle",
  ),
  downloadWav: document.getElementById("download-wav"),
  audioStatus: document.getElementById("audio-status"),
  drySource: document.getElementById("dry-source"),
  wetMix: document.getElementById("wet-mix"),
  wetMixValue: document.getElementById("wet-mix-value"),
  audioGain: document.getElementById("audio-gain"),
  audioGainValue: document.getElementById("audio-gain-value"),
  playAuralization: document.getElementById("play-auralization"),
  exportAuralization: document.getElementById("export-auralization"),
  metricEstimatedRt60: document.getElementById("metric-estimated-rt60"),
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

const {
  populatePresetSelects,
  populatePortalMaterialSelect,
  updateMaterialSummary,
  syncPresetSelects,
  syncFormFromState,
  syncDirectivityAvailability,
  syncRoomModeAvailability,
  syncPortalAvailability,
  syncModeButtons,
  syncIrViewButtons,
  syncPositionConstraints,
  normalizeSpatialState,
  applyRoomPreset,
  applyMaterialPreset,
  loadStateFromUrl,
  applySerializedState,
  assignRoomState,
  assignMaterialState,
  assignSourceState,
  assignReceiverState,
  assignPortalState,
  assignRenderState,
  normalizeSceneState,
  normalizeCrossoverWindow,
  encodeStateForUrl,
  decodeStateFromUrl,
  scheduleUrlSync,
  syncUrlState,
  base64UrlEncode,
  base64UrlDecode,
  buildRequest,
  getSystemTheme,
  applyTheme,
  updateThemeToggle,
  cycleTheme,
  initTheme,
} = appState;

appState.setAppStateContext({
  window,
  document,
  state,
  refs,
  POSITION_MARGIN,
  DEFAULT_STATE,
  THEME_KEY,
  THEME_MODES,
  THEME_ICONS,
  MATERIALS,
  PORTAL_MATERIALS,
  ROOM_PRESETS,
  ROOM_PRESET_GROUPS,
  MATERIAL_PRESETS,
  sceneModule,
  updateSceneView,
  getLastRender: () => lastRender,
  getLatestEncodedState: () => latestEncodedState,
  setLatestEncodedState: (value) => {
    latestEncodedState = value;
  },
  drawWaveform,
});

let sceneView = null;
let lastRender = null;
// The most recent progressive preview. It is held only so the waveform survives
// a redraw between the preview arriving and the full render replacing it.
let previewTier = null;
let lastAudioURL = null;
let currentRenderId = 0;

window.algoAcousticsDemoReady = false;
window.algoAcousticsDemoLastRender = null;

init();

async function init() {
  initTheme();
  populatePresetSelects();
  populateMaterialSelects();
  populatePortalMaterialSelect();
  populateDrySources();
  loadStateFromUrl();
  bindEvents();
  syncFormFromState();
  syncAuralizationControls();
  updateMaterialSummary();
  redrawWaveform(null);
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
    setPageReady(false);
    setEngineStatus("Loading WASM", "loading");
    renderWorkerController = createRenderWorkerController();
    await renderWorkerController.start();
    applyEngineLimits(renderWorkerController.limits);
    setEngineStatus("WASM ready", "ready");
    setRenderBadge("Waiting for render", "ready");
    setPageReady(true);
  } catch (error) {
    setPageReady(false);
    setEngineStatus("WASM failed", "error");
    setRenderBadge("Engine error", "error");
    refs.renderLog.textContent = `${error}`;
  }
}

// applyEngineLimits sizes the render sliders from the envelope the WASM module
// enforces, so the controls and the engine cannot disagree about what the demo
// accepts. See docs/web-demo-limits.md.
function applyEngineLimits(limits) {
  const clamped = applyRangeBounds(document, rangeBoundsFromLimits(limits));
  if (clamped.length === 0) {
    return;
  }

  // A stored or shared URL can carry a setting the current build no longer
  // accepts; pull the state back to what the slider now shows.
  state.render.numRays = Number(refs.renderRays.value);
  state.render.maxOrder = Number(refs.renderOrder.value);
  state.render.durationSeconds = Number(refs.renderDuration.value);
  syncFormFromState();
}

function createRenderWorkerController() {
  return new RenderWorkerController({
    createWorker: () => new Worker("worker.js"),
    onMessage: handleWorkerMessage,
    onError: (error) => {
      setPageReady(false);
      if (renderSession) {
        finalizeRenderSession("Render failed", "error");
      }
      setRenderBadge("Engine error", "error");
      setEngineStatus("WASM failed", "error");
      refs.renderLog.textContent = `${error}`;
    },
  });
}

function setPageReady(ready) {
  window.algoAcousticsDemoReady = ready;
  if (ready) {
    window.dispatchEvent(new Event("algo-acoustics-demo-ready"));
  }
}

function publishRenderResult(requestId, result) {
  window.algoAcousticsDemoLastRender = {
    requestId,
    mode: result.mode,
    sampleCount: result.samples?.length ?? 0,
    wavByteLength: result.wavBytes?.length ?? 0,
    memory: result.memory ?? null,
    warnings: result.warnings ?? [],
  };
  reportRenderMemory(result);
  window.dispatchEvent(
    new CustomEvent("algo-acoustics-demo-render", {
      detail: window.algoAcousticsDemoLastRender,
    }),
  );
}

// A Go/WASM heap only grows, so the peak a worker reaches is the footprint the
// tab keeps. Log it after every render to make drift visible without adding
// chrome to the page.
function reportRenderMemory(result) {
  for (const warning of result?.warnings ?? []) {
    console.warn(`[algo-acoustics] ${warning}`);
  }

  const memory = result?.memory;
  if (!memory) {
    return;
  }

  const mib = (bytes) => (Number(bytes ?? 0) / (1024 * 1024)).toFixed(1);
  console.info(
    `[algo-acoustics] memory: peak ${mib(memory.peakSysBytes)} MiB of a ` +
      `${mib(memory.budgetBytes)} MiB budget ` +
      `(live ${mib(memory.heapBytes)} MiB, projected ${mib(memory.estimateBytes)} MiB)`,
  );
}

function handleWorkerMessage(event) {
  const { type, requestId, result, stage, percent, message } = event.data ?? {};

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

  if (type === "tier") {
    if (!renderSession || requestId !== renderSession.requestId) {
      return;
    }

    applyRenderTier(event.data);
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

    const samples = result?.samples
      ? new Float32Array(result.samples)
      : new Float32Array();
    const wavBytes = result?.wavBytes
      ? new Uint8Array(result.wavBytes)
      : new Uint8Array();
    // Phase 21 WASM contract: endpoint responses arrive as encoded WAVs at
    // portalResponses.closedWavBytes and portalResponses.openWavBytes. The
    // top-level wavBytes remains the aperture-specific preview/download IR.
    const portalResponses = normalizePortalResponseWavs(
      result?.portalResponses,
    );
    lastRender = {
      ...result,
      samples,
      wavBytes,
      portalResponses,
    };
    // The finished render supersedes any preview that was on screen.
    previewTier = null;
    publishRenderResult(requestId, lastRender);

    // Sync the actual mode used back into state (mesh rooms are forced to "late").
    if (lastRender.mode && lastRender.mode !== state.render.mode) {
      state.render.mode = lastRender.mode;
      syncModeButtons();
    }
    // A render the timeout cut short is still a usable response, but it is a
    // coarser one than was asked for, so it must not be reported as complete.
    // The warning itself is already appended to the render log.
    const timedOut = findTimeoutWarning(lastRender.warnings);
    const badge = timedOut ? "Partial render" : "Render complete";

    setRenderBadge(badge, "ready");
    updateMetrics(lastRender);
    installSPLHeatmap(lastRender.splHeatmap);
    updateAudio(lastRender.wavBytes, lastRender.portalResponses);
    updateRenderLog(lastRender);
    redrawWaveform(samples, {
      durationSeconds:
        lastRender.durationSeconds ?? state.render.durationSeconds,
    });
    refs.downloadWav.disabled = false;
    finalizeRenderSession("Render room", "ready");
    setRenderBadge(badge, "ready");
    scheduleUrlSync();
  }
}

function bindEvents() {
  refs.themeToggle?.addEventListener("click", cycleTheme);

  refs.splHeatmap?.addEventListener("click", () => {
    const enabled = refs.splHeatmap.getAttribute("aria-pressed") !== "true";
    setSPLHeatmapVisibility(enabled);
    sceneModule.updateSceneView();
  });

  refs.resetScene.addEventListener("click", () => {
    if (isRenderActive()) {
      requestRenderCancel();
    }
    copyState(DEFAULT_STATE, state);
    syncFormFromState();
    updateMaterialSummary();
    updateSceneView();
    redrawWaveform();
    setRenderBadge("Waiting for render", "ready");
    scheduleUrlSync();
  });

  refs.roomPreset.addEventListener("change", () => {
    applyRoomPreset(refs.roomPreset.value);
  });

  refs.materialPreset.addEventListener("change", () => {
    applyMaterialPreset(refs.materialPreset.value);
  });

  refs.portalEnabled.addEventListener("change", () => {
    state.portal.enabled = refs.portalEnabled.checked;
    normalizeSpatialState();
    syncFormFromState();
    updateSceneView();
    scheduleUrlSync();
  });

  bindRange(refs.portalAperture, refs.portalApertureValue, (value) => {
    state.portal.aperture = clamp(value, 0, 1);
    auralizationEngine?.setPortalAperture(
      state.portal.aperture,
      state.portal.rootOrder,
    );
    return `${Math.round(state.portal.aperture * 100)}%`;
  });

  refs.portalMaterial.addEventListener("change", () => {
    state.portal.material = refs.portalMaterial.value;
    scheduleUrlSync();
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
    const receiverRoom = state.portal.enabled
      ? state.portal.receiverRoom
      : state.room;
    state.receiver.x = clamp(
      value,
      POSITION_MARGIN,
      receiverRoom.width - POSITION_MARGIN,
    );
    state.roomPreset = "custom";
    syncPresetSelects();
  });
  bindNumber(refs.receiverY, (value) => {
    const receiverRoom = state.portal.enabled
      ? state.portal.receiverRoom
      : state.room;
    state.receiver.y = clamp(
      value,
      POSITION_MARGIN,
      receiverRoom.depth - POSITION_MARGIN,
    );
    state.roomPreset = "custom";
    syncPresetSelects();
  });
  bindNumber(refs.receiverZ, (value) => {
    const receiverRoom = state.portal.enabled
      ? state.portal.receiverRoom
      : state.room;
    state.receiver.z = clamp(
      value,
      POSITION_MARGIN,
      receiverRoom.height - POSITION_MARGIN,
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
      state.reflections = normalizeReflectionOrder(value);
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
      redrawWaveform();
      scheduleUrlSync();
    });
  });

  refs.irViewButtons.forEach((button) => {
    button.addEventListener("click", () => {
      state.irView = button.dataset.view;
      syncIrViewButtons();
      redrawWaveform();
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
    if (audioContext) {
      void dryAudioLoader
        .load(audioContext, refs.drySource.value)
        .catch((error) => {
          auralizationStatusOverride = {
            label: `Sample failed: ${error.message ?? error}`,
            state: "error",
          };
          syncAuralizationControls();
        });
    }
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

  refs.exportAuralization.addEventListener("click", () => {
    void exportAuralizedWav();
  });

  refs.waveformCanvas.addEventListener(
    "wheel",
    (event) => {
      if (!event.shiftKey || !lastRender?.samples?.length) {
        return;
      }

      event.preventDefault();
      const factor = event.deltaY < 0 ? 1.12 : 1 / 1.12;
      waveformZoom = clamp(waveformZoom * factor, 0.5, 8);
      redrawWaveform();
    },
    { passive: false },
  );

  window.addEventListener("resize", () => {
    sceneView?.resize();
    redrawWaveform();
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

function populateDrySources() {
  refs.drySource.innerHTML = Object.entries(DRY_AUDIO_SOURCES)
    .map(
      ([value, source]) => `<option value="${value}">${source.label}</option>`,
    )
    .join("");
}

function onRenderButtonClick() {
  if (isRenderActive()) {
    requestRenderCancel();
    return;
  }

  startRender();
}

function startRender() {
  if (!renderWorkerController?.ready) {
    setRenderBadge("Engine unavailable", "error");
    refs.renderLog.textContent = "WASM worker is not ready.";
    return;
  }

  if (renderSession) {
    return;
  }

  const requestId = ++currentRenderId;
  const request = buildRequest();
  previewTier = null;
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
  refs.renderLog.textContent =
    "Rendering hybrid impulse response in the browser…";
  updateRenderButton("Cancel", "Starting render…", 0);
  scheduleRenderButtonAnimation();
  renderWorkerController.postMessage({
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
  void restartWorkerAfterCancel(renderSession.requestId);
}

async function restartWorkerAfterCancel(requestId) {
  setPageReady(false);
  setEngineStatus("Restarting WASM", "loading");
  refs.renderScene.disabled = true;

  try {
    await renderWorkerController.restart();
  } catch (error) {
    setPageReady(false);
    setEngineStatus("WASM failed", "error");
    if (renderSession?.requestId === requestId) {
      finalizeRenderSession("Render failed", "error");
    }
    setRenderBadge("Engine error", "error");
    refs.renderLog.textContent = `${error}`;
    return;
  }

  setEngineStatus("WASM ready", "ready");
  setPageReady(true);
  if (renderSession?.requestId === requestId) {
    finalizeRenderSession("Render canceled", "ready");
    refs.renderLog.textContent = "Render canceled.";
  }
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

// applyRenderTier shows a progressive tier while the render is still running
// (see docs/web-demo-limits.md). Tiers are informational: they never touch
// lastRender, the download, or the auralization, all of which stay bound to the
// render that finishes. What they do is stop the page looking frozen — the
// statistical tier
// puts a decay time on screen within milliseconds, and the preview tier puts a
// real waveform there before the accurate render is a fraction done.
function applyRenderTier({ tier, payload }) {
  if (tier === "statistical") {
    const statistics = readStatisticalTier(payload);
    if (statistics) {
      refs.metricEstimatedRt60.textContent = formatEstimatedRt60(statistics);
      refs.metricEstimatedRt60.title = describeEstimatedRt60(statistics);
    }
    return;
  }

  if (tier !== "preview") {
    return;
  }

  const preview = readPreviewTier(payload);
  if (!preview) {
    return;
  }

  previewTier = preview;
  redrawWaveform(preview.samples, {
    durationSeconds: preview.durationSeconds,
    crossoverTimeSeconds: Math.min(
      state.render.crossoverTimeSeconds,
      preview.durationSeconds,
    ),
  });
  refs.renderLog.textContent = describePreviewTier(preview);
}

function updateMetrics(result) {
  refs.metricFirstArrival.textContent = `${result.firstArrivalMs.toFixed(1)} ms`;
  refs.metricPeak.textContent = result.peakAmplitude.toFixed(3);
  refs.metricEvents.textContent = result.earlyEventCount.toLocaleString();
  refs.metricRenderTime.textContent = `${Math.round(result.renderMs)} ms`;
}

function updateRenderLog(result) {
  const lines = [
    `Mode: ${result.mode ?? state.render.mode}`,
    `Room: ${state.room.width.toFixed(1)} × ${state.room.depth.toFixed(1)} × ${state.room.height.toFixed(1)} m`,
    `Room preset: ${state.roomPreset}`,
    `Material preset: ${state.materialPreset}`,
    `Source: ${formatVec(state.source.x, state.source.y, state.source.z)} | ${state.source.directivity}`,
    `Receiver: ${formatVec(state.receiver.x, state.receiver.y, state.receiver.z)}`,
    `Rays: ${state.render.numRays.toLocaleString()} | Max order: ${state.render.maxOrder}`,
    `Duration: ${state.render.durationSeconds.toFixed(2)} s | Crossover: ${state.render.crossoverTimeSeconds.toFixed(2)} s`,
    `Peak amplitude: ${result.peakAmplitude.toFixed(4)} | RMS: ${result.rmsAmplitude.toFixed(4)}`,
  ];
  if (state.portal.enabled) {
    lines.splice(
      2,
      0,
      `Portal: ${PORTAL_MATERIALS[state.portal.material].label} | Aperture: ${Math.round(state.portal.aperture * 100)}%`,
    );
  }
  // Settings the memory budget reduced are reported here, because the sliders
  // still show what was asked for rather than what was rendered.
  for (const warning of result.warnings ?? []) {
    lines.push(warning);
  }
  refs.renderLog.textContent = lines.join("\n");
}

function updateAudio(wavBytes, portalResponses = null) {
  if (lastAudioURL) {
    URL.revokeObjectURL(lastAudioURL);
  }
  const blob = new Blob([wavBytes], { type: "audio/wav" });
  lastAudioURL = URL.createObjectURL(blob);
  refs.audioPlayer.src = lastAudioURL;
  lastAuralizedWav = wavBytes.slice
    ? wavBytes.slice()
    : new Uint8Array(wavBytes);
  lastPortalResponseWavs = portalResponses;
  currentIRVersion += 1;
  currentIRBuffer = null;
  currentIRDecode = null;
  currentPortalBuffers = null;
  currentPortalDecode = null;
  auralizationStatusOverride = null;
  syncAuralizationControls();

  if (auralizationEngine?.isPlaying) {
    void installLatestImpulseResponse(currentIRVersion);
  }
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
  refs.playAuralization.textContent = auralizationStarting
    ? "Loading…"
    : currentAuralizationPlaying
      ? "Stop"
      : "Play";
  refs.playAuralization.classList.toggle(
    "is-playing",
    currentAuralizationPlaying,
  );
  refs.playAuralization.setAttribute(
    "aria-pressed",
    String(currentAuralizationPlaying),
  );
  refs.playAuralization.disabled =
    auralizationStarting || (!lastAuralizedWav && !currentAuralizationPlaying);
  refs.exportAuralization.disabled = !lastAuralizedWav;
  if (auralizationEngine?.isPlaying) {
    auralizationEngine.setMix(wetMix, dbToLinear(gainDb));
  }

  if (auralizationStatusOverride) {
    updateAuralizationStatus(
      auralizationStatusOverride.label,
      auralizationStatusOverride.state,
    );
  } else if (!lastAuralizedWav) {
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
    currentIRBuffer = null;
    currentIRDecode = null;
    currentPortalBuffers = null;
    currentPortalDecode = null;
    auralizationEngine = new AuralizationEngine({
      context: audioContext,
      onPlaybackEnded: () => {
        currentAuralizationPlaying = false;
        auralizationStatusOverride = null;
        syncAuralizationControls();
      },
    });
  }

  if (audioContext.state === "suspended") {
    await audioContext.resume();
  }

  return audioContext;
}

async function ensureIrAudioBuffer() {
  if (!lastAuralizedWav) {
    throw new Error("No rendered IR available");
  }

  const context = await ensureAudioContext();
  while (!currentIRBuffer) {
    const version = currentIRVersion;
    if (!currentIRDecode || currentIRDecode.version !== version) {
      const bytes = lastAuralizedWav;
      const arrayBuffer = bytes.buffer.slice(
        bytes.byteOffset,
        bytes.byteOffset + bytes.byteLength,
      );
      currentIRDecode = {
        version,
        promise: context.decodeAudioData(arrayBuffer),
      };
    }

    try {
      const decoded = await currentIRDecode.promise;
      if (version === currentIRVersion) {
        currentIRBuffer = decoded;
      }
    } catch (error) {
      if (version === currentIRVersion) {
        currentIRDecode = null;
        throw error;
      }
    }
  }
  return currentIRBuffer;
}

async function ensurePortalAudioBuffers() {
  if (!state.portal.enabled || !lastPortalResponseWavs) {
    return null;
  }

  const context = await ensureAudioContext();
  while (!currentPortalBuffers) {
    const version = currentIRVersion;
    if (!currentPortalDecode || currentPortalDecode.version !== version) {
      currentPortalDecode = {
        version,
        promise: Promise.all([
          context.decodeAudioData(
            copyWavArrayBuffer(lastPortalResponseWavs.closedWavBytes),
          ),
          context.decodeAudioData(
            copyWavArrayBuffer(lastPortalResponseWavs.openWavBytes),
          ),
        ]),
      };
    }

    try {
      const [closed, open] = await currentPortalDecode.promise;
      if (version === currentIRVersion) {
        currentPortalBuffers = { closed, open };
      }
    } catch (error) {
      if (version === currentIRVersion) {
        currentPortalDecode = null;
        throw error;
      }
    }
  }

  return currentPortalBuffers;
}

async function installLatestImpulseResponse(requestedVersion) {
  auralizationStatusOverride = { label: "Updating IR", state: "busy" };
  syncAuralizationControls();
  try {
    const [irBuffer, portalBuffers] = await Promise.all([
      ensureIrAudioBuffer(),
      ensurePortalAudioBuffers(),
    ]);
    if (requestedVersion !== currentIRVersion) {
      return;
    }
    const transitioned = portalBuffers
      ? auralizationEngine?.setPortalResponses(
          portalBuffers.closed,
          portalBuffers.open,
          {
            aperture: state.portal.aperture,
            rootOrder: state.portal.rootOrder,
          },
        )
      : auralizationEngine?.setImpulseResponse(irBuffer);
    if (transitioned) {
      if (audioTransitionTimer) {
        window.clearTimeout(audioTransitionTimer);
      }
      audioTransitionTimer = window.setTimeout(() => {
        audioTransitionTimer = 0;
        auralizationStatusOverride = null;
        syncAuralizationControls();
      }, 120);
      return;
    }
    auralizationStatusOverride = null;
  } catch (error) {
    auralizationStatusOverride = {
      label: `IR update failed: ${error.message ?? error}`,
      state: "error",
    };
  }
  syncAuralizationControls();
}

async function startAuralizationPlayback() {
  if (!lastAuralizedWav) {
    updateAuralizationStatus("Waiting for IR", "loading");
    return;
  }

  auralizationStarting = true;
  auralizationStatusOverride = { label: "Loading audio", state: "busy" };
  syncAuralizationControls();
  try {
    const context = await ensureAudioContext();
    const [irBuffer, portalBuffers, dryBuffer] = await Promise.all([
      ensureIrAudioBuffer(),
      ensurePortalAudioBuffers(),
      dryAudioLoader.load(context, refs.drySource.value),
    ]);
    if (portalBuffers) {
      auralizationEngine.setPortalResponses(
        portalBuffers.closed,
        portalBuffers.open,
        {
          aperture: state.portal.aperture,
          rootOrder: state.portal.rootOrder,
        },
      );
    } else {
      auralizationEngine.setImpulseResponse(irBuffer);
    }
    auralizationEngine.play(dryBuffer, {
      wetMix: clamp(Number(refs.wetMix.value || 0.72), 0, 1),
      gainLinear: dbToLinear(
        clamp(Number(refs.audioGain.value || -0.5), -12, 6),
      ),
    });
    currentAuralizationPlaying = true;
    auralizationStatusOverride = null;
  } catch (error) {
    auralizationEngine?.stop(false);
    currentAuralizationPlaying = false;
    auralizationStatusOverride = {
      label: `Audio failed: ${error.message ?? error}`,
      state: "error",
    };
  } finally {
    auralizationStarting = false;
    syncAuralizationControls();
  }
}

function stopAuralizationPlayback() {
  if (audioTransitionTimer) {
    window.clearTimeout(audioTransitionTimer);
    audioTransitionTimer = 0;
  }
  auralizationEngine?.stop(false);
  currentAuralizationPlaying = false;
  auralizationStatusOverride = null;
  syncAuralizationControls();
}

async function exportAuralizedWav() {
  if (!lastAuralizedWav) {
    return;
  }

  auralizationStatusOverride = { label: "Exporting audio", state: "busy" };
  syncAuralizationControls();
  try {
    const rendered = await renderAuralizationOffline();
    const wavBytes = encodeAudioBufferToWav(rendered);
    downloadBytes(wavBytes, "algo-acoustics-auralization.wav", "audio/wav");
    auralizationStatusOverride = null;
  } catch (error) {
    auralizationStatusOverride = {
      label: `Export failed: ${error.message ?? error}`,
      state: "error",
    };
  }
  syncAuralizationControls();
}

async function renderAuralizationOffline() {
  const baseContext = await ensureAudioContext();
  const [irBuffer, portalBuffers, dryBuffer] = await Promise.all([
    ensureIrAudioBuffer(),
    ensurePortalAudioBuffers(),
    dryAudioLoader.load(baseContext, refs.drySource.value),
  ]);
  const impulseResponses = portalBuffers
    ? [portalBuffers.closed, portalBuffers.open]
    : [irBuffer];
  const maximumIRDuration = Math.max(
    ...impulseResponses.map((response) => response.duration),
  );
  const offline = new OfflineAudioContext(
    1,
    Math.ceil(
      (dryBuffer.duration + maximumIRDuration + 1.2) * baseContext.sampleRate,
    ),
    baseContext.sampleRate,
  );

  const drySource = offline.createBufferSource();
  drySource.buffer = dryBuffer;
  const dryGain = offline.createGain();
  const wetGain = offline.createGain();
  const outputGain = offline.createGain();
  const wetMix = clamp(Number(refs.wetMix.value || 0.72), 0, 1);
  const gainDb = clamp(Number(refs.audioGain.value || -0.5), -12, 6);
  dryGain.gain.value = Math.sqrt(1 - wetMix);
  wetGain.gain.value = Math.sqrt(wetMix);
  outputGain.gain.value = dbToLinear(gainDb);

  drySource.connect(dryGain);
  const portalWeight = portalBuffers
    ? portalCrossfadeWeight(state.portal.aperture, state.portal.rootOrder)
    : 1;
  impulseResponses.forEach((response, index) => {
    const convolver = offline.createConvolver();
    const responseGain = offline.createGain();
    convolver.buffer = response;
    responseGain.gain.value = portalBuffers
      ? index === 0
        ? 1 - portalWeight
        : portalWeight
      : 1;
    drySource.connect(convolver);
    convolver.connect(responseGain);
    responseGain.connect(wetGain);
  });
  dryGain.connect(outputGain);
  wetGain.connect(outputGain);
  outputGain.connect(offline.destination);
  drySource.start();

  return offline.startRendering();
}

function encodeAudioBufferToWav(audioBuffer) {
  return audioModule.encodeAudioBufferToWav(audioBuffer);
}

function normalizePortalResponseWavs(portalResponses) {
  if (!portalResponses?.closedWavBytes || !portalResponses?.openWavBytes) {
    return null;
  }

  return {
    closedWavBytes: new Uint8Array(portalResponses.closedWavBytes),
    openWavBytes: new Uint8Array(portalResponses.openWavBytes),
  };
}

function copyWavArrayBuffer(bytes) {
  return bytes.buffer.slice(
    bytes.byteOffset,
    bytes.byteOffset + bytes.byteLength,
  );
}

// State helpers moved to app-state.js.

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
  clearSPLHeatmap();
  sceneModule.updateSceneView();
}

function installSPLHeatmap(heatmap) {
  const hasSamples =
    Array.isArray(heatmap?.samples) && heatmap.samples.length > 0;
  sceneModule.setSPLHeatmap(hasSamples ? heatmap : null);
  refs.splHeatmap.disabled = !hasSamples;
  refs.splHeatmap.title = hasSamples
    ? "Toggle the simulated broadband surface SPL map"
    : "Render the room before enabling the broadband SPL map";

  if (!hasSamples) {
    setSPLHeatmapVisibility(false);
    return;
  }

  refs.splHeatmapMin.textContent = `${Math.round(heatmap.minimumDb)} dB`;
  refs.splHeatmapMax.textContent = `${Math.round(heatmap.maximumDb)} dB rel.`;
  setSPLHeatmapVisibility(true);
  sceneModule.updateSceneView();
}

function clearSPLHeatmap() {
  sceneModule.setSPLHeatmap(null);
  if (!refs.splHeatmap) {
    return;
  }
  refs.splHeatmap.disabled = true;
  refs.splHeatmap.title =
    "Render the room before enabling the broadband SPL map";
  setSPLHeatmapVisibility(false);
}

function setSPLHeatmapVisibility(enabled) {
  const active = Boolean(enabled) && !refs.splHeatmap.disabled;
  refs.splHeatmap.setAttribute("aria-pressed", String(active));
  refs.splHeatmapLegend.hidden = !active;
  sceneModule.setSPLHeatmapEnabled(active);
}

function drawWaveform(canvas, samples = null, state = {}) {
  audioModule.drawWaveform(canvas, samples, state);
}

// A preview tier renders a shorter response than the request asked for, so the
// time axis has to come from the tier that produced the samples rather than from
// the sliders, or the waveform would be drawn against a scale it does not span.
function redrawWaveform(
  // While only a preview exists, an unrelated redraw (a zoom or a view switch)
  // must keep showing it rather than blanking the canvas.
  samples = lastRender?.samples ?? previewTier?.samples ?? null,
  {
    durationSeconds = lastRender
      ? state.render.durationSeconds
      : (previewTier?.durationSeconds ?? state.render.durationSeconds),
    crossoverTimeSeconds = state.render.crossoverTimeSeconds,
  } = {},
) {
  drawWaveform(refs.waveformCanvas, samples, {
    irView: state.irView,
    renderMode: state.render.mode,
    durationSeconds,
    crossoverTimeSeconds,
    waveformZoom,
  });
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
