// Demo limits; see docs/web-demo-limits.md.
//
// The request envelope is defined once, in Go (web/wasm/limits.go), and travels
// to the page on the worker's ready message. These helpers turn it into slider
// bounds so the controls cannot offer a setting the engine will silently clamp,
// and cannot drift out of sync with the enforced values as the HTML ages.

/**
 * Slider bounds derived from the published limits, keyed by input element id.
 *
 * Steps are chosen so the extremes are reachable: a range input whose span is
 * not a whole number of steps cannot be dragged to its maximum.
 */
export function rangeBoundsFromLimits(limits) {
  if (!limits) {
    return {};
  }

  const bounds = {};

  if (isPositive(limits.minNumRays) && isPositive(limits.maxNumRays)) {
    bounds["render-rays"] = {
      min: limits.minNumRays,
      max: limits.maxNumRays,
      step: limits.minNumRays,
    };
  }

  if (isPositive(limits.minMaxOrder) && isPositive(limits.maxMaxOrder)) {
    bounds["render-order"] = {
      min: limits.minMaxOrder,
      max: limits.maxMaxOrder,
      step: 1,
    };
  }

  if (
    isPositive(limits.minDurationSeconds) &&
    isPositive(limits.maxDurationSeconds)
  ) {
    bounds["render-duration"] = {
      min: limits.minDurationSeconds,
      max: limits.maxDurationSeconds,
      step: 0.05,
    };
  }

  return bounds;
}

/**
 * Applies the bounds to the page's range inputs.
 *
 * A value already outside the new bounds is pulled back inside, because the
 * engine would clamp it anyway and a slider showing an unreachable number is
 * worse than one showing the truth. Returns the ids whose value had to move.
 */
export function applyRangeBounds(document, bounds) {
  const clamped = [];

  for (const [id, bound] of Object.entries(bounds)) {
    const input = document.getElementById(id);
    if (!input) {
      continue;
    }

    input.min = String(bound.min);
    input.max = String(bound.max);
    input.step = String(bound.step);

    const value = Number(input.value);
    if (!Number.isFinite(value)) {
      continue;
    }

    const inRange = Math.min(Math.max(value, bound.min), bound.max);
    if (inRange !== value) {
      input.value = String(inRange);
      clamped.push(id);
    }
  }

  return clamped;
}

function isPositive(value) {
  return Number.isFinite(value) && value > 0;
}
