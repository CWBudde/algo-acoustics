import * as presets from "./app-presets.js";
import * as appState from "./app-state.js";
import * as sceneModule from "./app-scene.js";
import * as audioModule from "./app-audio.js";
import { AuralizationEngine } from "./auralization-engine.mjs";
import {
  DRY_AUDIO_SOURCES,
  DryAudioLoader,
} from "./audio-samples.mjs";
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
import { RenderWorkerController } from "./render-worker-controller.mjs";

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
let renderWorkerController = null;
let renderSession = null;
let renderButtonAnimation = 0;
let urlSyncTimer = 0;
let latestEncodedState = null;
let audioContext = null;
let currentIRBuffer = null;
let currentIRDecode = null;
let currentIRVersion = 0;
let auralizationEngine = null;
let auralizationStarting = false;
let currentAuralizationPlaying = false;
let auralizationStatusOverride = null;
let audioTransitionTimer = 0;
let lastAuralizedWav = null;
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
  updateMaterialSummary,
  syncPresetSelects,
  syncFormFromState,
  syncDirectivityAvailability,
  syncRoomModeAvailability,
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
let lastAudioURL = null;
let currentRenderId = 0;

window.algoAcousticsDemoReady = false;
window.algoAcousticsDemoLastRender = null;

init();

async function init() {
  initTheme();
  populatePresetSelects();
  populateMaterialSelects();
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
  };
  window.dispatchEvent(
    new CustomEvent("algo-acoustics-demo-render", {
      detail: window.algoAcousticsDemoLastRender,
    }),
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
    lastRender = {
      ...result,
      samples,
      wavBytes,
    };
    publishRenderResult(requestId, lastRender);

    // Sync the actual mode used back into state (mesh rooms are forced to "late").
    if (lastRender.mode && lastRender.mode !== state.render.mode) {
      state.render.mode = lastRender.mode;
      syncModeButtons();
    }
    setRenderBadge("Render complete", "ready");
    updateMetrics(lastRender);
    updateAudio(lastRender.wavBytes);
    updateRenderLog(lastRender);
    redrawWaveform(samples);
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

function updateMetrics(result) {
  refs.metricFirstArrival.textContent = `${result.firstArrivalMs.toFixed(1)} ms`;
  refs.metricPeak.textContent = result.peakAmplitude.toFixed(3);
  refs.metricEvents.textContent = result.earlyEventCount.toLocaleString();
  refs.metricRenderTime.textContent = `${Math.round(result.renderMs)} ms`;
}

function updateRenderLog(result) {
  refs.renderLog.textContent = [
    `Mode: ${result.mode ?? state.render.mode}`,
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
  lastAuralizedWav = wavBytes.slice
    ? wavBytes.slice()
    : new Uint8Array(wavBytes);
  currentIRVersion += 1;
  currentIRBuffer = null;
  currentIRDecode = null;
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

async function installLatestImpulseResponse(requestedVersion) {
  auralizationStatusOverride = { label: "Updating IR", state: "busy" };
  syncAuralizationControls();
  try {
    const irBuffer = await ensureIrAudioBuffer();
    if (requestedVersion !== currentIRVersion) {
      return;
    }
    const transitioned = auralizationEngine?.setImpulseResponse(irBuffer);
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
    const [irBuffer, dryBuffer] = await Promise.all([
      ensureIrAudioBuffer(),
      dryAudioLoader.load(context, refs.drySource.value),
    ]);
    auralizationEngine.setImpulseResponse(irBuffer);
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
  const [irBuffer, dryBuffer] = await Promise.all([
    ensureIrAudioBuffer(),
    dryAudioLoader.load(baseContext, refs.drySource.value),
  ]);
  const offline = new OfflineAudioContext(
    1,
    Math.ceil(
      (dryBuffer.duration + irBuffer.duration + 1.2) * baseContext.sampleRate,
    ),
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
  sceneModule.updateSceneView();
}

function drawWaveform(canvas, samples = null, state = {}) {
  audioModule.drawWaveform(canvas, samples, state);
}

function redrawWaveform(samples = lastRender?.samples ?? null) {
  drawWaveform(refs.waveformCanvas, samples, {
    irView: state.irView,
    renderMode: state.render.mode,
    durationSeconds: state.render.durationSeconds,
    crossoverTimeSeconds: state.render.crossoverTimeSeconds,
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
