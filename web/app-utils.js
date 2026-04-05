export function cssVar(name) {
  return getComputedStyle(document.documentElement)
    .getPropertyValue(name)
    .trim();
}

export function cssNumber(name, fallback) {
  const value = Number.parseFloat(cssVar(name));
  return Number.isFinite(value) ? value : fallback;
}

export function formatVec(x, y, z) {
  return `${x.toFixed(2)}, ${y.toFixed(2)}, ${z.toFixed(2)}`;
}

export function average(values) {
  if (!values.length) {
    return 0;
  }
  return values.reduce((sum, value) => sum + value, 0) / values.length;
}

export function clamp(value, minValue, maxValue) {
  return Math.min(Math.max(value, minValue), maxValue);
}

export function clampInt(value, minValue, maxValue) {
  return Math.round(clamp(value, minValue, maxValue));
}

export function dbToLinear(valueDb) {
  return 10 ** (valueDb / 20);
}

export function isWithin(value, min, max) {
  return value >= min - 1e-9 && value <= max + 1e-9;
}

export function roundToStep(value, step) {
  return Math.round(value / step) * step;
}

export function capitalize(value) {
  return value.charAt(0).toUpperCase() + value.slice(1);
}

export function downloadBytes(bytes, filename, mimeType) {
  const url = URL.createObjectURL(new Blob([bytes], { type: mimeType }));
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  link.click();
  setTimeout(() => URL.revokeObjectURL(url), 0);
}

export function copyState(source, target) {
  target.room = structuredClone(source.room);
  target.materials = structuredClone(source.materials);
  target.source = structuredClone(source.source);
  target.receiver = structuredClone(source.receiver);
  target.render = structuredClone(source.render);
  target.reflections = source.reflections;
}
