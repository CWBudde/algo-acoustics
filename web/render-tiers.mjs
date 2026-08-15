// Progressive render tiers (PLAN.md 19.7).
//
// The WASM render is synchronous and holds its worker until it finishes, so
// without intermediate reporting the page would show nothing for as long as the
// render takes — which at the top of the demo's envelope is many seconds. Go
// pushes tiers out mid-render and the worker forwards them here.
//
// These helpers only interpret a tier message. Applying one to the DOM is app.js
// work; keeping the interpretation separate is what makes it testable without a
// browser.

/**
 * Reads the statistical tier, which reports estimates derived from room geometry
 * and absorption with no simulation behind them.
 *
 * Returns null when the tier carries nothing usable — a mesh room has no shoebox
 * volume for the Sabine and Eyring estimators to work from, so the demo shows no
 * estimate rather than a fabricated one.
 */
export function readStatisticalTier(payload) {
  const statistics = payload?.statistics;
  if (!statistics || !Number.isFinite(statistics.sabineRt60Secs)) {
    return null;
  }

  if (statistics.sabineRt60Secs <= 0) {
    return null;
  }

  return {
    sabineRt60Secs: statistics.sabineRt60Secs,
    eyringRt60Secs: Number.isFinite(statistics.eyringRt60Secs)
      ? statistics.eyringRt60Secs
      : statistics.sabineRt60Secs,
    c80Db: Number.isFinite(statistics.c80Db) ? statistics.c80Db : 0,
    d50: Number.isFinite(statistics.d50) ? statistics.d50 : 0,
  };
}

/**
 * Reads the preview tier: a coarse but complete impulse response.
 *
 * The duration comes back with the samples because the preview truncates long
 * responses, and drawing its waveform against the slider's duration would put
 * the decay on the wrong part of the time axis.
 */
export function readPreviewTier(payload) {
  if (!payload?.samples) {
    return null;
  }

  const samples =
    payload.samples instanceof Float32Array
      ? payload.samples
      : new Float32Array(payload.samples);

  if (samples.length === 0) {
    return null;
  }

  return {
    samples,
    sampleRate: payload.sampleRate ?? 0,
    durationSeconds: payload.durationSeconds ?? samples.length / 48000,
    numRays: payload.numRays ?? 0,
    maxOrder: payload.maxOrder ?? 0,
    earlyEventCount: payload.earlyEventCount ?? 0,
    elapsedMs: payload.elapsedMs ?? 0,
  };
}

/**
 * Formats the estimate for the metric chip.
 */
export function formatEstimatedRt60(statistics) {
  return `${statistics.sabineRt60Secs.toFixed(2)} s`;
}

/**
 * Formats the tooltip that explains where the estimate came from, so the number
 * is not mistaken for a measurement of the rendered response.
 */
export function describeEstimatedRt60(statistics) {
  return (
    `Sabine ${statistics.sabineRt60Secs.toFixed(2)} s, ` +
    `Eyring ${statistics.eyringRt60Secs.toFixed(2)} s ` +
    `(estimated from room geometry and absorption)`
  );
}

/**
 * Formats the render-log line that accompanies a preview.
 */
export function describePreviewTier(preview) {
  return (
    `Preview ready after ${Math.round(preview.elapsedMs)} ms ` +
    `(${preview.numRays.toLocaleString()} rays, order ${preview.maxOrder}, ` +
    `${preview.durationSeconds.toFixed(2)} s). Refining…`
  );
}

/**
 * Reports whether a result was cut short by the render timeout, so the page can
 * say so rather than presenting a coarse response as the finished article.
 *
 * The warning text is produced by the Go side; matching on its prefix keeps the
 * two ends coupled through one agreed string instead of a parallel flag.
 */
export function findTimeoutWarning(warnings) {
  return (
    (warnings ?? []).find(
      (warning) =>
        typeof warning === "string" && warning.startsWith("render timeout:"),
    ) ?? null
  );
}
