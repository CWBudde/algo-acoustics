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
