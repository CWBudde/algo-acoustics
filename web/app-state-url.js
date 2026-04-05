import { requireAppStateContext } from "./app-state-context.js";

let urlSyncTimer = 0;
let latestEncodedState = null;

function base64UrlEncode(value) {
  return btoa(unescape(encodeURIComponent(value)))
    .replaceAll("+", "-")
    .replaceAll("/", "_")
    .replaceAll("=", "");
}

function base64UrlDecode(value) {
  const padded = value.replaceAll("-", "+").replaceAll("_", "/");
  const normalized = padded + "=".repeat((4 - (padded.length % 4)) % 4);
  return decodeURIComponent(escape(atob(normalized)));
}

export function encodeStateForUrl() {
  const { state } = requireAppStateContext();
  return base64UrlEncode(
    JSON.stringify({
      roomPreset: state.roomPreset,
      materialPreset: state.materialPreset,
      room: state.room,
      materials: state.materials,
      source: state.source,
      receiver: state.receiver,
      render: state.render,
      irView: state.irView,
      reflections: state.reflections,
    }),
  );
}

export function decodeStateFromUrl(encoded) {
  return JSON.parse(base64UrlDecode(encoded));
}

export function scheduleUrlSync() {
  const { window } = requireAppStateContext();
  if (urlSyncTimer) {
    window.clearTimeout(urlSyncTimer);
  }
  urlSyncTimer = window.setTimeout(syncUrlState, 120);
}

export function syncUrlState() {
  const { window } = requireAppStateContext();
  const encoded = encodeStateForUrl();
  if (encoded === latestEncodedState) {
    return;
  }

  latestEncodedState = encoded;
  const url = new URL(window.location.href);
  url.searchParams.set("scene", encoded);
  window.history.replaceState(null, "", `${url.pathname}?${url.searchParams.toString()}${url.hash}`);
}

export function buildRequest() {
  const { state } = requireAppStateContext();
  return {
    room: state.room,
    materials: state.materials,
    source: state.source,
    receiver: state.receiver,
    render: state.render,
    irView: state.irView,
    reflections: state.reflections,
  };
}

export function getLatestEncodedState() {
  return latestEncodedState;
}

export function setLatestEncodedState(value) {
  latestEncodedState = value;
}
