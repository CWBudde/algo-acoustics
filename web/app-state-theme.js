import { requireAppStateContext } from "./app-state-context.js";

const THEME_MODES = ["auto", "light", "dark"];
const THEME_KEY = "algo-acoustics-demo-theme";
let currentThemeMode = "auto";
let mediaThemeQuery = null;

export function getSystemTheme() {
  const { window } = requireAppStateContext();
  return window.matchMedia?.("(prefers-color-scheme: dark)")?.matches ? "dark" : "light";
}

export function applyTheme(mode) {
  const { window, refs, sceneModule, drawWaveform, state, getLastRender } = requireAppStateContext();
  currentThemeMode = mode;
  const theme = mode === "auto" ? getSystemTheme() : mode;
  if (theme === "dark") {
    window.document.documentElement.setAttribute("data-theme", "dark");
  } else {
    window.document.documentElement.removeAttribute("data-theme");
  }

  updateThemeToggle();
  sceneModule.applySceneTheme();
  if (refs.waveformCanvas) {
    drawWaveform(refs.waveformCanvas, getLastRender?.()?.samples ?? null, {
      irView: state.irView,
      renderMode: state.render.mode,
      durationSeconds: state.render.durationSeconds,
      crossoverTimeSeconds: state.render.crossoverTimeSeconds,
    });
  }
}

export function updateThemeToggle() {
  const { refs } = requireAppStateContext();
  if (!refs.themeToggle) {
    return;
  }
  refs.themeToggle.setAttribute("data-theme-mode", currentThemeMode);
  refs.themeToggle.innerHTML = THEME_ICONS[currentThemeMode] || THEME_ICONS.auto;
}

export function cycleTheme() {
  const index = THEME_MODES.indexOf(currentThemeMode);
  const nextMode = THEME_MODES[(index + 1) % THEME_MODES.length];
  requireAppStateContext().window.localStorage.setItem(THEME_KEY, nextMode);
  applyTheme(nextMode);
}

export function initTheme() {
  const { window } = requireAppStateContext();
  const savedTheme = window.localStorage.getItem(THEME_KEY);
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

export const THEME_ICONS = {
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
