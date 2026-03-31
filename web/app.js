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

const DEFAULT_STATE = {
  room: { width: 6.4, depth: 4.8, height: 2.9 },
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

const refs = {
  engineStatus: document.getElementById("engine-status"),
  themeToggle: document.getElementById("theme-toggle"),
  renderBadge: document.getElementById("render-badge"),
  resetScene: document.getElementById("reset-scene"),
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
  renderModeButtons: Array.from(document.querySelectorAll(".mode-button")),
  renderScene: document.getElementById("render-scene"),
  downloadWav: document.getElementById("download-wav"),
  metricFirstArrival: document.getElementById("metric-first-arrival"),
  metricPeak: document.getElementById("metric-peak"),
  metricEvents: document.getElementById("metric-events"),
  metricRenderTime: document.getElementById("metric-render-time"),
  waveformCanvas: document.getElementById("waveform-canvas"),
  sceneCanvas: document.getElementById("scene-canvas"),
  audioPlayer: document.getElementById("audio-player"),
  renderLog: document.getElementById("render-log"),
};

let wasmApi = null;
let sceneView = null;
let lastRender = null;
let lastAudioURL = null;

init();

async function init() {
  initTheme();
  populateMaterialSelects();
  bindEvents();
  syncFormFromState();
  updateMaterialSummary();
  drawWaveform();
  sceneView = createSceneView(refs.sceneCanvas);
  applySceneTheme();
  updateSceneView();
  await initWasm();
}

async function initWasm() {
  try {
    setEngineStatus("Loading WASM", "loading");
    const go = new Go();
    const result = await WebAssembly.instantiateStreaming(
      fetch("algo_acoustics_demo.wasm"),
      go.importObject,
    );
    go.run(result.instance);
    wasmApi = window.algoAcousticsDemo;
    if (!wasmApi?.renderScene) {
      throw new Error(
        "WASM runtime did not expose algoAcousticsDemo.renderScene",
      );
    }
    setEngineStatus("WASM ready", "ready");
    setRenderBadge("Waiting for render", "ready");
  } catch (error) {
    setEngineStatus("WASM failed", "error");
    setRenderBadge("Engine error", "error");
    refs.renderLog.textContent = `${error}`;
  }
}

function bindEvents() {
  refs.themeToggle?.addEventListener("click", cycleTheme);

  refs.resetScene.addEventListener("click", () => {
    copyState(DEFAULT_STATE, state);
    syncFormFromState();
    updateMaterialSummary();
    updateSceneView();
    drawWaveform(lastRender?.samples ?? null);
    setRenderBadge("Waiting for render", "ready");
  });

  refs.sourceDirectivity.addEventListener("change", () => {
    state.source.directivity = refs.sourceDirectivity.value;
    syncDirectivityAvailability();
    updateSceneView();
  });

  bindNumber(refs.roomWidth, (value) => {
    state.room.width = clamp(value, 2.5, 16);
    normalizeSpatialState();
  });
  bindNumber(refs.roomDepth, (value) => {
    state.room.depth = clamp(value, 2.5, 16);
    normalizeSpatialState();
  });
  bindNumber(refs.roomHeight, (value) => {
    state.room.height = clamp(value, 2.2, 7);
    normalizeSpatialState();
  });

  bindNumber(refs.sourceX, (value) => {
    state.source.x = clamp(
      value,
      POSITION_MARGIN,
      state.room.width - POSITION_MARGIN,
    );
  });
  bindNumber(refs.sourceY, (value) => {
    state.source.y = clamp(
      value,
      POSITION_MARGIN,
      state.room.depth - POSITION_MARGIN,
    );
  });
  bindNumber(refs.sourceZ, (value) => {
    state.source.z = clamp(
      value,
      POSITION_MARGIN,
      state.room.height - POSITION_MARGIN,
    );
  });
  bindNumber(refs.receiverX, (value) => {
    state.receiver.x = clamp(
      value,
      POSITION_MARGIN,
      state.room.width - POSITION_MARGIN,
    );
  });
  bindNumber(refs.receiverY, (value) => {
    state.receiver.y = clamp(
      value,
      POSITION_MARGIN,
      state.room.depth - POSITION_MARGIN,
    );
  });
  bindNumber(refs.receiverZ, (value) => {
    state.receiver.z = clamp(
      value,
      POSITION_MARGIN,
      state.room.height - POSITION_MARGIN,
    );
  });

  bindRange(refs.sourceAzimuth, refs.sourceAzimuthValue, (value) => {
    state.source.azimuthDegrees = value;
    return `${Math.round(value)}°`;
  });
  bindRange(refs.sourceFocus, refs.sourceFocusValue, (value) => {
    state.source.cardioidOrder = value;
    return value.toFixed(2);
  });
  bindRange(refs.sourceGain, refs.sourceGainValue, (value) => {
    state.source.gainDb = value;
    return `${value.toFixed(1)} dB`;
  });
  bindRange(refs.renderRays, refs.renderRaysValue, (value) => {
    state.render.numRays = roundToStep(value, 128);
    refs.renderRays.value = String(state.render.numRays);
    return `${state.render.numRays.toLocaleString()} rays`;
  });
  bindRange(refs.renderOrder, refs.renderOrderValue, (value) => {
    state.render.maxOrder = Math.round(value);
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
    return `${value.toFixed(2)} s`;
  });
  bindRange(refs.renderCrossover, refs.renderCrossoverValue, (value) => {
    state.render.crossoverTimeSeconds = clamp(
      value,
      0.03,
      state.render.durationSeconds * 0.85,
    );
    return `${state.render.crossoverTimeSeconds.toFixed(2)} s`;
  });

  refs.renderWindow.addEventListener("change", () => {
    state.render.crossoverWindow = refs.renderWindow.value;
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
      updateMaterialSummary();
      updateSceneView();
    });
  });

  refs.renderModeButtons.forEach((button) => {
    button.addEventListener("click", () => {
      state.render.mode = button.dataset.mode;
      syncModeButtons();
      drawWaveform(lastRender?.samples ?? null);
    });
  });

  refs.renderScene.addEventListener("click", runRender);
  refs.downloadWav.addEventListener("click", () => {
    if (!lastRender?.wavBytes) {
      return;
    }
    downloadBytes(lastRender.wavBytes, "algo-acoustics-demo.wav", "audio/wav");
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

  syncModeButtons();
  syncDirectivityAvailability();
}

function syncDirectivityAvailability() {
  const isCardioid = state.source.directivity === "cardioid";
  refs.sourceAzimuth.disabled = !isCardioid;
  refs.sourceFocus.disabled = !isCardioid;
}

function syncModeButtons() {
  refs.renderModeButtons.forEach((button) => {
    button.classList.toggle(
      "is-active",
      button.dataset.mode === state.render.mode,
    );
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
}

async function runRender() {
  if (!wasmApi?.renderScene) {
    setRenderBadge("Engine unavailable", "error");
    return;
  }

  refs.renderScene.disabled = true;
  setRenderBadge("Rendering…", "busy");
  refs.renderLog.textContent =
    "Rendering hybrid impulse response in the browser…";
  await nextPaint();

  try {
    const result = wasmApi.renderScene(buildRequest());
    if (result.error) {
      throw new Error(result.error);
    }

    const samples = new Float32Array(result.samples);
    const wavBytes = new Uint8Array(result.wavBytes);
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
  } catch (error) {
    setRenderBadge("Render failed", "error");
    refs.renderLog.textContent = `${error}`;
  } finally {
    refs.renderScene.disabled = false;
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

function buildRequest() {
  return {
    room: state.room,
    materials: state.materials,
    source: state.source,
    receiver: state.receiver,
    render: state.render,
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
    ambientLight,
    keyLight,
    fillLight,
    grid,
    resize,
  };
}

function updateSceneView() {
  if (!sceneView) {
    return;
  }

  const { roomGroup } = sceneView;
  roomGroup.clear();

  const width = state.room.width;
  const depth = state.room.depth;
  const height = state.room.height;
  const palette = scenePalette();

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

  roomGroup.add(createMarkerSphere(state.source, 0xff6b4a, 0.12));
  roomGroup.add(createMarkerSphere(state.receiver, 0x0f9d92, 0.14));
  roomGroup.add(createDirectPathLine(palette));
  roomGroup.add(createCornerFanLines(palette));

  if (state.source.directivity === "cardioid") {
    roomGroup.add(createDirectivityCone(palette));
  }
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

function createMarkerSphere(position, color, radius) {
  const marker = new THREE.Mesh(
    new THREE.SphereGeometry(radius, 28, 28),
    new THREE.MeshStandardMaterial({
      color,
      emissive: color,
      emissiveIntensity: 0.35,
    }),
  );
  marker.position.set(position.x, position.z, position.y);
  return marker;
}

function createDirectPathLine(palette) {
  const geometry = new THREE.BufferGeometry().setFromPoints([
    new THREE.Vector3(state.source.x, state.source.z, state.source.y),
    new THREE.Vector3(state.receiver.x, state.receiver.z, state.receiver.y),
  ]);
  return new THREE.Line(
    geometry,
    new THREE.LineDashedMaterial({
      color: palette.path,
      dashSize: 0.22,
      gapSize: 0.16,
      opacity: 1,
      transparent: true,
    }),
  );
}

function createCornerFanLines(palette) {
  const corners = [
    [0, 0, 0],
    [state.room.width, 0, 0],
    [0, 0, state.room.depth],
    [state.room.width, 0, state.room.depth],
    [0, state.room.height, 0],
    [state.room.width, state.room.height, 0],
    [0, state.room.height, state.room.depth],
    [state.room.width, state.room.height, state.room.depth],
  ];
  const points = [];
  corners.forEach(([x, y, z]) => {
    points.push(
      new THREE.Vector3(state.source.x, state.source.z, state.source.y),
    );
    points.push(new THREE.Vector3(x, y, z));
  });
  const geometry = new THREE.BufferGeometry().setFromPoints(points);
  return new THREE.LineSegments(
    geometry,
    new THREE.LineBasicMaterial({
      color: palette.ray,
      transparent: true,
      opacity: 1,
    }),
  );
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
  const maxAmplitude = Math.max(
    ...downsampled.map((value) => Math.abs(value)),
    1e-6,
  );

  context.strokeStyle = palette.trace;
  context.lineWidth = 2;
  context.beginPath();
  downsampled.forEach((sample, index) => {
    const x = (index / Math.max(1, downsampled.length - 1)) * width;
    const y = height / 2 - (sample / maxAmplitude) * height * 0.36;
    if (index === 0) {
      context.moveTo(x, y);
    } else {
      context.lineTo(x, y);
    }
  });
  context.stroke();

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

function bindRange(input, output, formatter) {
  const handler = () => {
    const value = Number(input.value);
    output.textContent = formatter(value);
    updateSceneView();
  };
  input.addEventListener("input", handler);
}

function bindNumber(input, onChange) {
  input.addEventListener("input", () => {
    onChange(Number(input.value));
    syncFormFromState();
    updateMaterialSummary();
    updateSceneView();
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

function roundToStep(value, step) {
  return Math.round(value / step) * step;
}

function copyState(source, target) {
  target.room = structuredClone(source.room);
  target.materials = structuredClone(source.materials);
  target.source = structuredClone(source.source);
  target.receiver = structuredClone(source.receiver);
  target.render = structuredClone(source.render);
}

function nextPaint() {
  return new Promise((resolve) => {
    requestAnimationFrame(() => requestAnimationFrame(resolve));
  });
}

function downloadBytes(bytes, filename, mimeType) {
  const url = URL.createObjectURL(new Blob([bytes], { type: mimeType }));
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  link.click();
  setTimeout(() => URL.revokeObjectURL(url), 0);
}
