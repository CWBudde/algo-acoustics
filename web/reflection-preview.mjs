export const MAX_REFLECTION_ORDER = 2;

export function normalizeReflectionOrder(value) {
  if (!Number.isFinite(value)) {
    return 0;
  }

  const rounded = Math.round(value);
  return Math.min(Math.max(rounded, 0), MAX_REFLECTION_ORDER);
}

export function getReflectionPreviewConfig(reflections) {
  const order = normalizeReflectionOrder(reflections);
  return {
    showDirectPath: true,
    reflectionOrders: Array.from({ length: order }, (_, index) => index + 1),
  };
}
