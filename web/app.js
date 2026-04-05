import * as THREE from "three";
import { OrbitControls } from "three/addons/controls/OrbitControls.js";

const MATERIALS = {
  defaultMaterial: {
    label: "Default Material",
    color: "#7385a3",
    absorption: [0.12, 0.12, 0.18, 0.24, 0.28, 0.32],
  },
  smoothConcrete: {
    label: "Smooth Concrete",
    color: "#a0a9b8",
    absorption: [0.02, 0.02, 0.03, 0.04, 0.05, 0.07],
  },
  plywoodPanels: {
    label: "Plywood Panels",
    color: "#c88c52",
    absorption: [0.16, 0.14, 0.12, 0.1, 0.09, 0.08],
  },
  glassWindow: {
    label: "Glass Window",
    color: "#6fb6d6",
    absorption: [0.18, 0.08, 0.05, 0.03, 0.02, 0.02],
  },
  pileCarpet: {
    label: "Pile Carpet",
    color: "#d05b55",
    absorption: [0.08, 0.14, 0.32, 0.58, 0.7, 0.72],
  },
  thinCarpet: {
    label: "Thin Carpet",
    color: "#f0a55f",
    absorption: [0.03, 0.05, 0.14, 0.24, 0.36, 0.39],
  },
  heavyCurtain: {
    label: "Heavy Curtain",
    color: "#6d8c72",
    absorption: [0.06, 0.1, 0.22, 0.48, 0.66, 0.7],
  },
  perforatedWood: {
    label: "Perforated Wood",
    color: "#9e6b43",
    absorption: [0.14, 0.22, 0.36, 0.42, 0.39, 0.3],
  },
};

const ROOM_PRESETS = {
  custom: {
    label: "Custom",
    kind: "shoebox",
  },
  shoebox: makeShoeboxPreset(
    "Shoebox",
    { width: 6.4, depth: 4.8, height: 2.9 },
    {
      x: 1.4,
      y: 1.9,
      z: 1.25,
      gainDb: 0,
      directivity: "omni",
      azimuthDegrees: 18,
      cardioidOrder: 1.15,
    },
    { x: 4.85, y: 2.9, z: 1.2 },
    {
      materialPreset: "concertHall",
      renderMode: "hybrid",
    },
  ),
  classroom: makeShoeboxPreset(
    "Classroom",
    { width: 8.8, depth: 6.4, height: 3.1 },
    {
      x: 1.8,
      y: 1.9,
      z: 1.35,
      gainDb: 0,
      directivity: "omni",
      azimuthDegrees: 0,
      cardioidOrder: 1.1,
    },
    { x: 6.7, y: 3.7, z: 1.35 },
    {
      materialPreset: "studio",
      renderMode: "hybrid",
    },
  ),
  lectureHall: makeShoeboxPreset(
    "Lecture Hall",
    { width: 14.2, depth: 9.4, height: 5.1 },
    {
      x: 2.1,
      y: 3.0,
      z: 1.55,
      gainDb: -1,
      directivity: "omni",
      azimuthDegrees: 0,
      cardioidOrder: 1.1,
    },
    { x: 10.6, y: 4.7, z: 1.45 },
    {
      materialPreset: "concertHall",
      renderMode: "hybrid",
    },
  ),
  rehearsalRoom: makeShoeboxPreset(
    "Rehearsal Room",
    { width: 8.2, depth: 5.8, height: 3.0 },
    {
      x: 2.4,
      y: 2.2,
      z: 1.32,
      gainDb: -1,
      directivity: "cardioid",
      azimuthDegrees: 20,
      cardioidOrder: 1.2,
    },
    { x: 6.1, y: 3.4, z: 1.28 },
    {
      materialPreset: "studio",
      renderMode: "hybrid",
    },
  ),
  library: makeShoeboxPreset(
    "Library",
    { width: 11.2, depth: 7.0, height: 3.7 },
    {
      x: 2.3,
      y: 2.0,
      z: 1.28,
      gainDb: -2,
      directivity: "omni",
      azimuthDegrees: 0,
      cardioidOrder: 1.1,
    },
    { x: 8.1, y: 4.2, z: 1.22 },
    {
      materials: {
        west: "perforatedWood",
        east: "perforatedWood",
        south: "thinCarpet",
        north: "thinCarpet",
        floor: "pileCarpet",
        ceiling: "heavyCurtain",
      },
      renderMode: "hybrid",
    },
  ),
  chapel: makeShoeboxPreset(
    "Chapel",
    { width: 12.4, depth: 6.8, height: 7.2 },
    {
      x: 3.0,
      y: 2.0,
      z: 1.55,
      gainDb: -1,
      directivity: "cardioid",
      azimuthDegrees: 8,
      cardioidOrder: 1.4,
    },
    { x: 8.9, y: 3.5, z: 1.4 },
    {
      materialPreset: "concertHall",
      renderMode: "hybrid",
    },
  ),
  podcastBooth: makeShoeboxPreset(
    "Podcast Booth",
    { width: 3.8, depth: 2.8, height: 2.4 },
    {
      x: 1.1,
      y: 1.2,
      z: 1.2,
      gainDb: 0,
      directivity: "cardioid",
      azimuthDegrees: 15,
      cardioidOrder: 1.6,
    },
    { x: 2.9, y: 1.6, z: 1.18 },
    {
      materialPreset: "studio",
      renderMode: "early",
    },
  ),
  loft: makeMeshPreset(
    "Loft Atrium",
    { width: 8.6, depth: 6.4, height: 3.2 },
    makeTriangularPrismMesh(8.6, 6.4, 3.2),
    {
      x: 2.1,
      y: 1.8,
      z: 1.35,
      gainDb: -1,
      directivity: "cardioid",
      azimuthDegrees: 18,
      cardioidOrder: 1.35,
    },
    { x: 5.8, y: 3.7, z: 1.25 },
    {
      materialPreset: "concertHall",
      renderMode: "late",
    },
  ),
  wedgeHall: makeMeshPreset(
    "Wedge Hall",
    { width: 12.0, depth: 7.2, height: 5.4 },
    makeSlopedRoofMesh(12.0, 7.2, 3.0, 5.4),
    {
      x: 2.7,
      y: 2.2,
      z: 1.48,
      gainDb: -1,
      directivity: "omni",
      azimuthDegrees: 0,
      cardioidOrder: 1.1,
    },
    { x: 8.8, y: 4.1, z: 1.4 },
    {
      materialPreset: "concertHall",
      renderMode: "late",
    },
  ),
  cornerGallery: makeMeshPreset(
    "Corner Gallery",
    { width: 9.4, depth: 6.0, height: 4.1 },
    makeSlopedRoofMesh(9.4, 6.0, 2.8, 4.1),
    {
      x: 1.9,
      y: 1.7,
      z: 1.34,
      gainDb: -2,
      directivity: "cardioid",
      azimuthDegrees: 12,
      cardioidOrder: 1.2,
    },
    { x: 6.8, y: 3.3, z: 1.28 },
    {
      materials: {
        west: "smoothConcrete",
        east: "glassWindow",
        south: "perforatedWood",
        north: "perforatedWood",
        floor: "thinCarpet",
        ceiling: "heavyCurtain",
      },
      renderMode: "late",
    },
  ),
};

const ROOM_PRESET_GROUPS = [
  {
    label: "Compact Rooms",
    presets: ["shoebox", "podcastBooth", "rehearsalRoom"],
  },
  {
    label: "Medium Rooms",
    presets: ["classroom", "library"],
  },
  {
    label: "Large Rooms",
    presets: ["lectureHall", "chapel"],
  },
  {
    label: "Non-Rectangular",
    presets: ["loft", "wedgeHall", "cornerGallery"],
  },
];

const MATERIAL_PRESETS = {
  custom: {
    label: "Custom",
  },
  concertHall: {
    label: "Concert Hall",
    materials: {
      west: "perforatedWood",
      east: "perforatedWood",
      south: "heavyCurtain",
      north: "heavyCurtain",
      floor: "plywoodPanels",
      ceiling: "heavyCurtain",
    },
  },
  studio: {
    label: "Studio",
    materials: {
      west: "thinCarpet",
      east: "thinCarpet",
      south: "plywoodPanels",
      north: "defaultMaterial",
      floor: "pileCarpet",
      ceiling: "heavyCurtain",
    },
  },
  bathroom: {
    label: "Bathroom",
    materials: {
      west: "glassWindow",
      east: "glassWindow",
      south: "smoothConcrete",
      north: "smoothConcrete",
      floor: "smoothConcrete",
      ceiling: "smoothConcrete",
    },
  },
};

function makeShoeboxPreset(label, room, source, receiver, options = {}) {
  return {
    label,
    kind: "shoebox",
    room,
    source,
    receiver,
    ...options,
  };
}

function makeMeshPreset(label, room, mesh, source, receiver, options = {}) {
  return {
    label,
    kind: "mesh",
    room,
    mesh,
    source,
    receiver,
    ...options,
  };
}

function makeTriangularPrismMesh(width, depth, height) {
  return {
    triangles: [
      { v0: { x: 0, y: 0, z: 0 }, v1: { x: width / 2, y: depth, z: 0 }, v2: { x: width, y: 0, z: 0 } },
      { v0: { x: 0, y: 0, z: height }, v1: { x: width, y: 0, z: height }, v2: { x: width / 2, y: depth, z: height } },
      { v0: { x: 0, y: 0, z: 0 }, v1: { x: width, y: 0, z: 0 }, v2: { x: width, y: 0, z: height } },
      { v0: { x: 0, y: 0, z: 0 }, v1: { x: width, y: 0, z: height }, v2: { x: 0, y: 0, z: height } },
      { v0: { x: width, y: 0, z: 0 }, v1: { x: width / 2, y: depth, z: 0 }, v2: { x: width / 2, y: depth, z: height } },
      { v0: { x: width, y: 0, z: 0 }, v1: { x: width / 2, y: depth, z: height }, v2: { x: width, y: 0, z: height } },
      { v0: { x: width / 2, y: depth, z: 0 }, v1: { x: 0, y: 0, z: 0 }, v2: { x: 0, y: 0, z: height } },
      { v0: { x: width / 2, y: depth, z: 0 }, v1: { x: 0, y: 0, z: height }, v2: { x: width / 2, y: depth, z: height } },
    ],
  };
}

function makeSlopedRoofMesh(width, depth, lowHeight, highHeight) {
  return {
    triangles: [
      { v0: { x: 0, y: 0, z: 0 }, v1: { x: width, y: 0, z: 0 }, v2: { x: width, y: depth, z: 0 } },
      { v0: { x: 0, y: 0, z: 0 }, v1: { x: width, y: depth, z: 0 }, v2: { x: 0, y: depth, z: 0 } },
      { v0: { x: 0, y: 0, z: lowHeight }, v1: { x: width, y: 0, z: lowHeight }, v2: { x: width, y: depth, z: highHeight } },
      { v0: { x: 0, y: 0, z: lowHeight }, v1: { x: width, y: depth, z: highHeight }, v2: { x: 0, y: depth, z: highHeight } },
      { v0: { x: 0, y: 0, z: 0 }, v1: { x: width, y: 0, z: 0 }, v2: { x: width, y: 0, z: lowHeight } },
      { v0: { x: 0, y: 0, z: 0 }, v1: { x: width, y: 0, z: lowHeight }, v2: { x: 0, y: 0, z: lowHeight } },
      { v0: { x: width, y: depth, z: 0 }, v1: { x: width, y: 0, z: 0 }, v2: { x: width, y: 0, z: lowHeight } },
      { v0: { x: width, y: depth, z: 0 }, v1: { x: width, y: 0, z: lowHeight }, v2: { x: width, y: depth, z: highHeight } },
      { v0: { x: 0, y: depth, z: 0 }, v1: { x: 0, y: 0, z: 0 }, v2: { x: 0, y: 0, z: lowHeight } },
      { v0: { x: 0, y: depth, z: 0 }, v1: { x: 0, y: 0, z: lowHeight }, v2: { x: 0, y: depth, z: highHeight } },
      { v0: { x: 0, y: depth, z: 0 }, v1: { x: width, y: depth, z: 0 }, v2: { x: width, y: depth, z: highHeight } },
      { v0: { x: 0, y: depth, z: 0 }, v1: { x: width, y: depth, z: highHeight }, v2: { x: 0, y: depth, z: highHeight } },
    ],
  };
}

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
  view: {
    spatialVisible: true,
    reflections: 2,
  },
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
  sceneVisibility: document.getElementById("scene-visibility"),
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
  sceneEmpty: document.getElementById("scene-empty"),
  viewportCard: document.querySelector(".viewport-card"),
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
  drawWaveform();
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
    drawWaveform(samples);
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
    drawWaveform(lastRender?.samples ?? null);
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

  refs.sceneVisibility.addEventListener("change", () => {
    state.view.spatialVisible = refs.sceneVisibility.checked;
    syncSpatialViewVisibility();
    scheduleUrlSync();
  });

  refs.sceneReflections.addEventListener("input", () => {
    const value = Number(refs.sceneReflections.value);
    if (Number.isFinite(value)) {
      state.view.reflections = clampInt(Math.round(value), 0, 6);
    }
    refs.sceneReflections.value = String(state.view.reflections);
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
      drawWaveform(lastRender?.samples ?? null);
      scheduleUrlSync();
    });
  });

  refs.irViewButtons.forEach((button) => {
    button.addEventListener("click", () => {
      state.irView = button.dataset.view;
      syncIrViewButtons();
      drawWaveform(lastRender?.samples ?? null);
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
    drawWaveform(lastRender?.samples ?? null);
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
  refs.sceneVisibility.checked = state.view.spatialVisible;
  refs.sceneReflections.value = String(state.view.reflections);

  syncModeButtons();
  syncDirectivityAvailability();
  syncSpatialViewVisibility();
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

function syncSpatialViewVisibility() {
  const enabled = Boolean(state.view.spatialVisible);
  if (refs.viewportCard) {
    refs.viewportCard.classList.toggle("is-disabled", !enabled);
  }
  if (refs.sceneEmpty) {
    refs.sceneEmpty.hidden = enabled;
  }
  if (sceneView) {
    sceneView.scene.visible = enabled;
    sceneView.roomGroup.visible = enabled;
    sceneView.pathGroup.visible = enabled;
    sceneView.controls.enabled = enabled && !dragState;
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
  const sampleRate = context.sampleRate;
  const durationSeconds = preset === "music" ? 3.5 : preset === "speech" ? 2.4 : 1.4;
  const buffer = context.createBuffer(1, Math.ceil(durationSeconds * sampleRate), sampleRate);
  const data = buffer.getChannelData(0);

  if (preset === "clap") {
    for (let index = 0; index < data.length; index += 1) {
      const t = index / sampleRate;
      const burst = Math.exp(-t * 16);
      const noise = Math.sin(index * 12.9898) * 43758.5453;
      const n = (noise - Math.floor(noise)) * 2 - 1;
      data[index] = burst * n * (t < 0.12 ? 1 : 0.15);
    }
    return buffer;
  }

  if (preset === "music") {
    const notes = [261.63, 329.63, 392, 523.25];
    for (let index = 0; index < data.length; index += 1) {
      const t = index / sampleRate;
      const note = notes[Math.floor(t * 1.5) % notes.length];
      const env = Math.exp(-t * 0.8) * (0.92 + 0.08 * Math.sin(t * 2 * Math.PI * 0.5));
      data[index] =
        env *
        (0.42 * Math.sin(2 * Math.PI * note * t) +
          0.18 * Math.sin(2 * Math.PI * note * 2 * t) +
          0.08 * Math.sin(2 * Math.PI * note * 3 * t));
    }
    return buffer;
  }

  // speech-like: voiced-ish pulses with moving formant-ish envelope.
  const phonemes = [
    { freq: 140, v1: 730, v2: 1090, amp: 1.0, dur: 0.32 },
    { freq: 155, v1: 530, v2: 1840, amp: 0.92, dur: 0.36 },
    { freq: 170, v1: 300, v2: 2240, amp: 0.86, dur: 0.34 },
    { freq: 150, v1: 600, v2: 870, amp: 0.88, dur: 0.36 },
  ];

  let cursor = 0;
  for (const phoneme of phonemes) {
    const start = cursor;
    const end = Math.min(data.length, start + Math.floor(phoneme.dur * sampleRate));
    for (let index = start; index < end; index += 1) {
      const t = (index - start) / sampleRate;
      const env = Math.exp(-t * 5.5);
      const base = Math.sin(2 * Math.PI * phoneme.freq * (index / sampleRate));
      const formant =
        0.55 * Math.sin(2 * Math.PI * phoneme.v1 * (index / sampleRate)) +
        0.35 * Math.sin(2 * Math.PI * phoneme.v2 * (index / sampleRate)) +
        0.1 * Math.sin(2 * Math.PI * phoneme.v2 * 1.5 * (index / sampleRate));
      data[index] += phoneme.amp * env * (0.45 * base + 0.55 * formant);
    }
    cursor = end + Math.floor(0.05 * sampleRate);
  }

  return buffer;
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
  const channels = audioBuffer.numberOfChannels;
  const sampleRate = audioBuffer.sampleRate;
  const frameCount = audioBuffer.length;
  const bytesPerSample = 2;
  const blockAlign = channels * bytesPerSample;
  const byteRate = sampleRate * blockAlign;
  const dataSize = frameCount * blockAlign;
  const buffer = new ArrayBuffer(44 + dataSize);
  const view = new DataView(buffer);
  let offset = 0;

  const writeString = (text) => {
    for (let index = 0; index < text.length; index += 1) {
      view.setUint8(offset + index, text.charCodeAt(index));
    }
    offset += text.length;
  };

  writeString("RIFF");
  view.setUint32(offset, 36 + dataSize, true);
  offset += 4;
  writeString("WAVE");
  writeString("fmt ");
  view.setUint32(offset, 16, true);
  offset += 4;
  view.setUint16(offset, 1, true);
  offset += 2;
  view.setUint16(offset, channels, true);
  offset += 2;
  view.setUint32(offset, sampleRate, true);
  offset += 4;
  view.setUint32(offset, byteRate, true);
  offset += 4;
  view.setUint16(offset, blockAlign, true);
  offset += 2;
  view.setUint16(offset, 16, true);
  offset += 2;
  writeString("data");
  view.setUint32(offset, dataSize, true);
  offset += 4;

  const channelData = [];
  for (let channel = 0; channel < channels; channel += 1) {
    channelData.push(audioBuffer.getChannelData(channel));
  }

  for (let frame = 0; frame < frameCount; frame += 1) {
    for (let channel = 0; channel < channels; channel += 1) {
      const sample = clamp(channelData[channel][frame], -1, 1);
      view.setInt16(offset, Math.round(sample * 0x7fff), true);
      offset += 2;
    }
  }

  return new Uint8Array(buffer);
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
  drawWaveform(lastRender?.samples ?? null);
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
    assignViewState(input.view);

    if (typeof input.irView === "string" && input.irView) {
      state.irView = input.irView;
    }
  }

  normalizeSceneState();
  syncFormFromState();
  updateMaterialSummary();
  updateSceneView();
  drawWaveform(lastRender?.samples ?? null);
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

function assignViewState(view) {
  if (!view || typeof view !== "object") {
    return;
  }

  if (typeof view.spatialVisible === "boolean") {
    state.view.spatialVisible = view.spatialVisible;
  }

  if (typeof view.reflections === "number" && Number.isFinite(view.reflections)) {
    state.view.reflections = view.reflections;
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
  state.view.spatialVisible = Boolean(state.view.spatialVisible);
  state.view.reflections = clampInt(Math.round(state.view.reflections), 0, 6);

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
    view: state.view,
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
    view: state.view,
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
  drawWaveform(lastRender?.samples ?? null);
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

function cssVar(name) {
  return getComputedStyle(document.documentElement)
    .getPropertyValue(name)
    .trim();
}

function cssNumber(name, fallback) {
  const value = Number.parseFloat(cssVar(name));
  return Number.isFinite(value) ? value : fallback;
}

function scenePalette() {
  return {
    gridMajor: cssVar("--scene-grid-major") || "#2f3d4a",
    gridMinor: cssVar("--scene-grid-minor") || "#1e2934",
    edge: cssVar("--scene-edge") || "rgba(243, 245, 248, 0.5)",
    ray: cssVar("--scene-ray") || "rgba(248, 250, 252, 0.18)",
    path: cssVar("--scene-path") || "rgba(242, 239, 231, 0.85)",
    key: cssVar("--scene-key") || "#fff2dc",
    fill: cssVar("--scene-fill") || "#7dd3c7",
    ambient: cssNumber("--scene-ambient", 0.8),
    keyIntensity: cssNumber("--scene-key-intensity", 1.3),
    fillIntensity: cssNumber("--scene-fill-intensity", 0.55),
  };
}

function waveformPalette(context, width) {
  const gradient = context.createLinearGradient(0, 0, width, 0);
  gradient.addColorStop(0, cssVar("--accent-2") || "#0f9d92");
  gradient.addColorStop(1, cssVar("--accent") || "#ff6b4a");
  return {
    background: cssVar("--wave-fill") || "#0d141b",
    grid: cssVar("--wave-grid") || "rgba(255,255,255,0.08)",
    empty: cssVar("--wave-empty") || "rgba(255,255,255,0.68)",
    divider: cssVar("--wave-divider") || "rgba(255,255,255,0.28)",
    trace: gradient,
  };
}

function applySceneTheme() {
  if (!sceneView) {
    return;
  }

  const palette = scenePalette();
  sceneView.ambientLight.intensity = palette.ambient;
  sceneView.keyLight.color.set(palette.key);
  sceneView.keyLight.intensity = palette.keyIntensity;
  sceneView.fillLight.color.set(palette.fill);
  sceneView.fillLight.intensity = palette.fillIntensity;

  const mat = sceneView.grid.material;
  if (Array.isArray(mat)) {
    mat[0].color.set(palette.gridMajor);
    mat[1].color.set(palette.gridMinor);
  } else {
    mat.color.set(palette.gridMajor);
  }
}

function createSceneView(canvas) {
  const renderer = new THREE.WebGLRenderer({
    canvas,
    antialias: true,
    alpha: true,
  });
  renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));
  const clock = new THREE.Clock();
  const animatedPathMaterials = [];

  const scene = new THREE.Scene();
  const camera = new THREE.PerspectiveCamera(36, 1, 0.1, 100);
  camera.position.set(8.5, 6.5, 8.5);

  const controls = new OrbitControls(camera, canvas);
  controls.enableDamping = true;
  controls.autoRotate = true;
  controls.autoRotateSpeed = 0.4;
  controls.target.set(
    state.room.width / 2,
    state.room.height / 2,
    state.room.depth / 2,
  );

  const ambientLight = new THREE.AmbientLight(0xffffff, 0.8);
  scene.add(ambientLight);
  const keyLight = new THREE.DirectionalLight(0xfff2dc, 1.3);
  keyLight.position.set(8, 10, 6);
  scene.add(keyLight);
  const fillLight = new THREE.DirectionalLight(0x7dd3c7, 0.55);
  fillLight.position.set(-6, 4, -8);
  scene.add(fillLight);
  const grid = new THREE.GridHelper(30, 24, 0x2f3d4a, 0x1e2934);
  scene.add(grid);

  const roomGroup = new THREE.Group();
  scene.add(roomGroup);
  const pathGroup = new THREE.Group();
  scene.add(pathGroup);

  function resize() {
    const width = canvas.clientWidth;
    const height = canvas.clientHeight;
    if (!width || !height) {
      return;
    }
    renderer.setSize(width, height, false);
    camera.aspect = width / height;
    camera.updateProjectionMatrix();
  }

  function animate() {
    resize();
    const delta = clock.getDelta();
    for (const material of animatedPathMaterials) {
      if (material && typeof material.dashOffset === "number") {
        material.dashOffset -= delta * 0.8;
      }
    }
    controls.target.set(
      state.room.width / 2,
      state.room.height / 2,
      state.room.depth / 2,
    );
    controls.update();
    renderer.render(scene, camera);
    requestAnimationFrame(animate);
  }
  animate();

  return {
    scene,
    camera,
    controls,
    renderer,
    roomGroup,
    pathGroup,
    roomSurfaces: [],
    animatedPathMaterials,
    ambientLight,
    keyLight,
    fillLight,
    grid,
    resize,
    sourceMarker: null,
    receiverMarker: null,
    interactiveObjects: [],
  };
}

function bindSceneInteractions(canvas) {
  sceneRaycaster = new THREE.Raycaster();
  scenePointer = new THREE.Vector2();
  sceneDragPlane = new THREE.Plane(new THREE.Vector3(0, 1, 0), 0);

  canvas.style.touchAction = "none";
  canvas.addEventListener("pointerdown", onScenePointerDown);
  canvas.addEventListener("pointermove", onScenePointerMove);
  canvas.addEventListener("pointerup", onScenePointerUp);
  canvas.addEventListener("pointercancel", onScenePointerUp);
  canvas.addEventListener("lostpointercapture", onScenePointerUp);
}

function onScenePointerDown(event) {
  if (!sceneView) {
    return;
  }

  const target = pickDragTarget(event);
  if (!target) {
    return;
  }

  event.preventDefault();

  dragState = {
    target,
    pointerId: event.pointerId,
    planeY: target === "source" ? state.source.z : state.receiver.z,
  };

  sceneView.controls.enabled = false;
  refs.sceneCanvas.setPointerCapture(event.pointerId);
  refs.sceneCanvas.classList.add("is-dragging");
}

function onScenePointerMove(event) {
  if (!dragState || !sceneView) {
    return;
  }

  if (event.pointerId !== dragState.pointerId) {
    return;
  }

  event.preventDefault();

  const point = projectScenePointerToPlane(event, dragState.planeY);
  if (!point) {
    return;
  }

  if (dragState.target === "source") {
    state.source.x = clamp(point.x, POSITION_MARGIN, state.room.width - POSITION_MARGIN);
    state.source.y = clamp(point.z, POSITION_MARGIN, state.room.depth - POSITION_MARGIN);
  } else if (dragState.target === "receiver") {
    state.receiver.x = clamp(point.x, POSITION_MARGIN, state.room.width - POSITION_MARGIN);
    state.receiver.y = clamp(point.z, POSITION_MARGIN, state.room.depth - POSITION_MARGIN);
  }

  syncFormFromState();
  updateSceneView();
  scheduleUrlSync();
}

function onScenePointerUp(event) {
  if (!dragState || event.pointerId !== dragState.pointerId) {
    return;
  }

  try {
    refs.sceneCanvas.releasePointerCapture(event.pointerId);
  } catch (error) {
    void error;
  }

  dragState = null;
  if (sceneView) {
    sceneView.controls.enabled = true;
  }
  refs.sceneCanvas.classList.remove("is-dragging");
}

function pickDragTarget(event) {
  if (!sceneView?.interactiveObjects?.length) {
    return null;
  }

  const rect = refs.sceneCanvas.getBoundingClientRect();
  scenePointer.x = ((event.clientX - rect.left) / rect.width) * 2 - 1;
  scenePointer.y = -(((event.clientY - rect.top) / rect.height) * 2 - 1);
  sceneRaycaster.setFromCamera(scenePointer, sceneView.camera);
  const hits = sceneRaycaster.intersectObjects(sceneView.interactiveObjects, false);
  if (!hits.length) {
    return null;
  }

  return hits[0].object.userData.dragTarget ?? null;
}

function projectScenePointerToPlane(event, planeY) {
  const rect = refs.sceneCanvas.getBoundingClientRect();
  scenePointer.x = ((event.clientX - rect.left) / rect.width) * 2 - 1;
  scenePointer.y = -(((event.clientY - rect.top) / rect.height) * 2 - 1);
  sceneRaycaster.setFromCamera(scenePointer, sceneView.camera);
  const plane = sceneDragPlane ?? new THREE.Plane(new THREE.Vector3(0, 1, 0), -planeY);
  plane.constant = -planeY;
  const point = new THREE.Vector3();
  const hit = sceneRaycaster.ray.intersectPlane(plane, point);
  return hit ? point : null;
}

function updateSceneView() {
  if (!sceneView) {
    return;
  }

  const { roomGroup, pathGroup } = sceneView;
  roomGroup.clear();
  pathGroup.clear();
  sceneView.roomSurfaces = [];
  sceneView.animatedPathMaterials.length = 0;

  const width = state.room.width;
  const depth = state.room.depth;
  const height = state.room.height;
  const palette = scenePalette();

  if (state.room.kind === "mesh" && state.room.mesh?.triangles?.length) {
    const meshPreview = createMeshRoomPreview(state.room.mesh, palette);
    roomGroup.add(meshPreview.mesh);
    roomGroup.add(meshPreview.edges);
    sceneView.roomSurfaces = [meshPreview.mesh];
  } else {
    const walls = [
      createWallMesh("west", depth, height, state.materials.west, (mesh) => {
        mesh.rotation.y = Math.PI / 2;
        mesh.position.set(0, height / 2, depth / 2);
      }),
      createWallMesh("east", depth, height, state.materials.east, (mesh) => {
        mesh.rotation.y = -Math.PI / 2;
        mesh.position.set(width, height / 2, depth / 2);
      }),
      createWallMesh("south", width, height, state.materials.south, (mesh) => {
        mesh.position.set(width / 2, height / 2, 0);
      }),
      createWallMesh("north", width, height, state.materials.north, (mesh) => {
        mesh.rotation.y = Math.PI;
        mesh.position.set(width / 2, height / 2, depth);
      }),
      createWallMesh("floor", width, depth, state.materials.floor, (mesh) => {
        mesh.rotation.x = -Math.PI / 2;
        mesh.position.set(width / 2, 0, depth / 2);
      }),
      createWallMesh("ceiling", width, depth, state.materials.ceiling, (mesh) => {
        mesh.rotation.x = Math.PI / 2;
        mesh.position.set(width / 2, height, depth / 2);
      }),
    ];

    walls.forEach((wall) => roomGroup.add(wall));
    sceneView.roomSurfaces = walls;

    const edgeBox = new THREE.LineSegments(
      new THREE.EdgesGeometry(new THREE.BoxGeometry(width, height, depth)),
      new THREE.LineBasicMaterial({
        color: palette.edge,
        transparent: true,
        opacity: 1,
      }),
    );
    edgeBox.position.set(width / 2, height / 2, depth / 2);
    roomGroup.add(edgeBox);
  }

  const sourceMarker = createMarkerSphere(state.source, 0xff6b4a, 0.12, "source");
  const receiverMarker = createMarkerSphere(state.receiver, 0x0f9d92, 0.14, "receiver");
  const directPath = createDirectPathLine(palette);
  pathGroup.add(directPath);
  sceneView.animatedPathMaterials.push(directPath.material);

  const rayPaths = createRayPathOverlay(palette);
  rayPaths.forEach((pathLine) => {
    pathGroup.add(pathLine);
    sceneView.animatedPathMaterials.push(pathLine.material);
  });

  roomGroup.add(sourceMarker);
  roomGroup.add(receiverMarker);

  if (state.source.directivity === "cardioid") {
    roomGroup.add(createDirectivityCone(palette));
  }

  sceneView.sourceMarker = sourceMarker;
  sceneView.receiverMarker = receiverMarker;
  sceneView.interactiveObjects = [sourceMarker, receiverMarker];
  sceneView.directPathLine = directPath;
  syncSpatialViewVisibility();
}

function createWallMesh(name, width, height, materialKey, configure) {
  const material = MATERIALS[materialKey];
  const averageAbsorption = average(material.absorption);
  const mesh = new THREE.Mesh(
    new THREE.PlaneGeometry(width, height),
    new THREE.MeshPhysicalMaterial({
      color: material.color,
      transparent: true,
      opacity: 0.16 + averageAbsorption * 0.26,
      roughness: 0.42,
      metalness: 0.05,
      side: THREE.DoubleSide,
      transmission: name === "east" ? 0.15 : 0,
    }),
  );
  configure(mesh);
  return mesh;
}

function createMeshRoomPreview(meshSpec, palette) {
  const geometry = new THREE.BufferGeometry();
  const positions = [];
  const triangles = meshSpec?.triangles ?? [];

  for (const tri of triangles) {
    for (const vertex of [tri.v0, tri.v1, tri.v2]) {
      positions.push(vertex.x, vertex.z, vertex.y);
    }
  }

  geometry.setAttribute("position", new THREE.Float32BufferAttribute(positions, 3));
  geometry.computeVertexNormals();

  const material = new THREE.MeshPhysicalMaterial({
    color: MATERIALS.perforatedWood.color,
    transparent: true,
    opacity: 0.22,
    roughness: 0.5,
    metalness: 0.02,
    side: THREE.DoubleSide,
  });

  const mesh = new THREE.Mesh(geometry, material);
  const edges = new THREE.LineSegments(
    new THREE.EdgesGeometry(geometry),
    new THREE.LineBasicMaterial({
      color: palette.edge,
      transparent: true,
      opacity: 1,
    }),
  );

  return { mesh, edges };
}

function createMarkerSphere(position, color, radius, target) {
  const marker = new THREE.Mesh(
    new THREE.SphereGeometry(radius, 28, 28),
    new THREE.MeshStandardMaterial({
      color,
      emissive: color,
      emissiveIntensity: 0.35,
    }),
  );
  marker.position.set(position.x, position.z, position.y);
  marker.userData.dragTarget = target;
  marker.userData.dragHeight = position.z;
  return marker;
}

function createDirectPathLine(palette) {
  const geometry = new THREE.BufferGeometry().setFromPoints([
    new THREE.Vector3(state.source.x, state.source.z, state.source.y),
    new THREE.Vector3(state.receiver.x, state.receiver.z, state.receiver.y),
  ]);
  const line = new THREE.Line(
    geometry,
    new THREE.LineDashedMaterial({
      color: palette.path,
      dashSize: 0.22,
      gapSize: 0.16,
      opacity: 1,
      transparent: true,
    }),
  );
  line.computeLineDistances();
  return line;
}

function createRayPathOverlay(palette) {
  if (!state.view.spatialVisible) {
    return [];
  }

  const reflections = clampInt(Math.round(state.view.reflections), 0, 6);
  if (reflections <= 0 || !sceneView?.roomSurfaces?.length) {
    return [];
  }

  const path = traceProbeRayPath(sceneView.roomSurfaces, reflections);
  if (path.length < 2) {
    return [];
  }

  return [makePathLine(path, palette.ray, 0.75)];
}

function makePathLine(points, color, opacity) {
  const geometry = new THREE.BufferGeometry().setFromPoints(points);
  const material = new THREE.LineDashedMaterial({
    color,
    dashSize: 0.18,
    gapSize: 0.12,
    opacity,
    transparent: true,
  });
  const line = new THREE.Line(geometry, material);
  line.computeLineDistances();
  return line;
}

function traceProbeRayPath(roomSurfaces, maxBounces) {
  const points = [new THREE.Vector3(state.source.x, state.source.z, state.source.y)];
  if (!roomSurfaces?.length) {
    return points;
  }

  if (maxBounces <= 0) {
    return points;
  }

  const raycaster = new THREE.Raycaster();
  let origin = points[0].clone();
  const target = new THREE.Vector3(state.receiver.x, state.receiver.z, state.receiver.y);
  let direction = target.clone().sub(origin);
  if (direction.lengthSq() === 0) {
    direction.set(1, 0, 0);
  }
  direction.normalize();

  for (let bounce = 0; bounce < maxBounces; bounce += 1) {
    raycaster.set(origin, direction);
    const hits = raycaster.intersectObjects(roomSurfaces, false);
    const hit = hits.find((candidate) => candidate.distance > 1e-4);
    if (!hit) {
      break;
    }

    points.push(hit.point.clone());
    const normal = getWorldFaceNormal(hit);
    if (!normal) {
      break;
    }

    direction = reflectDirection(direction, normal).normalize();
    origin = hit.point.clone().add(direction.clone().multiplyScalar(1e-3));
  }

  return points;
}

function getWorldFaceNormal(hit) {
  if (!hit?.face) {
    return null;
  }

  const normal = hit.face.normal.clone();
  normal.transformDirection(hit.object.matrixWorld);
  return normal.normalize();
}

function reflectDirection(direction, normal) {
  const n = normal.clone().normalize();
  return direction.clone().sub(n.multiplyScalar(2 * direction.dot(n)));
}

function createDirectivityCone(palette) {
  const cone = new THREE.Mesh(
    new THREE.ConeGeometry(0.16, 0.42, 24, 1, true),
    new THREE.MeshStandardMaterial({
      color: palette.path,
      transparent: true,
      opacity: 0.32,
    }),
  );
  const azimuthRadians = (state.source.azimuthDegrees * Math.PI) / 180;
  const direction = new THREE.Vector3(
    Math.cos(azimuthRadians),
    0,
    Math.sin(azimuthRadians),
  );
  cone.position.set(state.source.x, state.source.z, state.source.y);
  cone.quaternion.setFromUnitVectors(
    new THREE.Vector3(0, 1, 0),
    direction.normalize(),
  );
  cone.position.add(direction.multiplyScalar(0.24));
  return cone;
}

function drawWaveform(samples = null) {
  const canvas = refs.waveformCanvas;
  const context = canvas.getContext("2d");
  const width = canvas.clientWidth || canvas.width || 640;
  const height = canvas.clientHeight || canvas.height || 280;
  const palette = waveformPalette(context, width);
  canvas.width = width * Math.min(window.devicePixelRatio || 1, 2);
  canvas.height = height * Math.min(window.devicePixelRatio || 1, 2);
  context.setTransform(
    canvas.width / width,
    0,
    0,
    canvas.height / height,
    0,
    0,
  );

  context.clearRect(0, 0, width, height);
  context.fillStyle = palette.background;
  context.fillRect(0, 0, width, height);

  drawWaveGrid(context, width, height, palette);

  if (!samples || samples.length === 0) {
    context.fillStyle = palette.empty;
    context.font = '600 16px "Manrope"';
    context.fillText(
      "Render a scene to inspect the impulse response.",
      22,
      height / 2,
    );
    return;
  }

  const downsampled = downsampleForCanvas(samples, width);
  // Use a robust peak estimate so strong early reflections do not visually flatten the late tail.
  const maxAmplitude = robustPeakAmplitude(downsampled);

  const pad = 12;

  if (state.irView === "dB") {
    const dynRange = 60;

    // dB view: positive-only, 0 dB at top, -60 dB at bottom
    context.strokeStyle = palette.trace;
    context.lineWidth = 1.5;
    context.beginPath();
    downsampled.forEach((sample, index) => {
      const x = (index / Math.max(1, downsampled.length - 1)) * width;
      const abs = Math.abs(sample);
      const dB = abs > 0 ? 20 * Math.log10(abs / maxAmplitude) : -dynRange;
      const clampedDB = Math.max(dB, -dynRange);
      const normalized = 1 + clampedDB / dynRange; // 1 = 0dB (top), 0 = -60dB (bottom)
      const y = pad + (1 - normalized) * (height - 2 * pad);
      if (index === 0) {
        context.moveTo(x, y);
      } else {
        context.lineTo(x, y);
      }
    });
    context.stroke();
  } else {
    const mid = height / 2;

    // zero-line
    context.strokeStyle = palette.grid;
    context.lineWidth = 1.5;
    context.beginPath();
    context.moveTo(0, mid);
    context.lineTo(width, mid);
    context.stroke();

    // linear waveform
    context.strokeStyle = palette.trace;
    context.lineWidth = 1.5;
    context.beginPath();
    downsampled.forEach((sample, index) => {
      const x = (index / Math.max(1, downsampled.length - 1)) * width;
      const y = mid - (sample / maxAmplitude) * (mid - pad);
      if (index === 0) {
        context.moveTo(x, y);
      } else {
        context.lineTo(x, y);
      }
    });
    context.stroke();
  }

  if (state.render.mode === "hybrid") {
    const x =
      (state.render.crossoverTimeSeconds / state.render.durationSeconds) *
      width;
    context.strokeStyle = palette.divider;
    context.setLineDash([6, 6]);
    context.beginPath();
    context.moveTo(x, 18);
    context.lineTo(x, height - 18);
    context.stroke();
    context.setLineDash([]);
  }
}

function drawWaveGrid(context, width, height, palette) {
  context.strokeStyle = palette.grid;
  context.lineWidth = 1;
  for (let index = 1; index < 5; index += 1) {
    const x = (index / 5) * width;
    context.beginPath();
    context.moveTo(x, 0);
    context.lineTo(x, height);
    context.stroke();
  }
  for (let index = 1; index < 4; index += 1) {
    const y = (index / 4) * height;
    context.beginPath();
    context.moveTo(0, y);
    context.lineTo(width, y);
    context.stroke();
  }
}

function downsampleForCanvas(samples, targetWidth) {
  const blockSize = Math.max(
    1,
    Math.floor(samples.length / Math.max(1, targetWidth)),
  );
  const output = [];
  for (let index = 0; index < samples.length; index += blockSize) {
    let peak = 0;
    for (
      let inner = index;
      inner < Math.min(index + blockSize, samples.length);
      inner += 1
    ) {
      if (Math.abs(samples[inner]) > Math.abs(peak)) {
        peak = samples[inner];
      }
    }
    output.push(peak);
  }
  return output;
}

function robustPeakAmplitude(samples) {
  if (!samples || samples.length === 0) {
    return 1e-6;
  }

  const magnitudes = samples
    .map((value) => Math.abs(value))
    .sort((left, right) => left - right);

  const index = Math.floor((magnitudes.length - 1) * 0.995);
  const percentile = magnitudes[Math.max(0, index)] || 0;
  const absoluteMax = magnitudes[magnitudes.length - 1] || 0;

  // Keep enough headroom to avoid over-clipping while still revealing lower-level tails.
  return Math.max(percentile, absoluteMax * 0.2, 1e-6);
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

function formatVec(x, y, z) {
  return `${x.toFixed(2)}, ${y.toFixed(2)}, ${z.toFixed(2)} m`;
}

function capitalize(value) {
  return value.charAt(0).toUpperCase() + value.slice(1);
}

function average(values) {
  return (
    values.reduce((sum, value) => sum + value, 0) / Math.max(1, values.length)
  );
}

function clamp(value, minValue, maxValue) {
  return Math.min(Math.max(value, minValue), maxValue);
}

function dbToLinear(valueDb) {
  return Math.pow(10, valueDb / 20);
}

function roundToStep(value, step) {
  return Math.round(value / step) * step;
}

function copyState(source, target) {
  target.room = structuredClone(source.room);
  target.materials = structuredClone(source.materials);
  target.source = structuredClone(source.source);
  target.receiver = structuredClone(source.receiver);
  target.render = structuredClone(source.render);
  target.view = structuredClone(source.view);
}

function downloadBytes(bytes, filename, mimeType) {
  const url = URL.createObjectURL(new Blob([bytes], { type: mimeType }));
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  link.click();
  setTimeout(() => URL.revokeObjectURL(url), 0);
}
