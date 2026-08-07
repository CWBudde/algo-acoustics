const COLOR_STOPS = [
  { position: 0, color: [0.09, 0.16, 0.42] },
  { position: 0.32, color: [0.0, 0.69, 0.78] },
  { position: 0.62, color: [0.96, 0.82, 0.2] },
  { position: 0.82, color: [0.96, 0.36, 0.12] },
  { position: 1, color: [0.68, 0.03, 0.16] },
];

export function normalizeSPLHeatmap(input) {
  if (!input || !Array.isArray(input.samples) || !input.samples.length) {
    return null;
  }

  const samples = input.samples
    .filter(isValidSample)
    .map((sample) => ({
      surfaceId: sample.surfaceId,
      x: Number(sample.x),
      y: Number(sample.y),
      z: Number(sample.z),
      levelDb: Number(sample.levelDb),
    }));
  if (!samples.length) {
    return null;
  }

  const sampleMinimum = Math.min(...samples.map((sample) => sample.levelDb));
  const sampleMaximum = Math.max(...samples.map((sample) => sample.levelDb));
  const minimumDb = Number.isFinite(input.minimumDb)
    ? Number(input.minimumDb)
    : sampleMinimum;
  const maximumDb = Number.isFinite(input.maximumDb)
    ? Number(input.maximumDb)
    : sampleMaximum;

  return {
    minimumDb: Math.min(minimumDb, maximumDb),
    maximumDb: Math.max(minimumDb, maximumDb),
    samples,
  };
}

export function samplesForSurface(heatmap, surfaceId) {
  return heatmap?.samples?.filter((sample) => sample.surfaceId === surfaceId) ?? [];
}

export function interpolateSPL(samples, point) {
  if (!samples?.length) {
    return null;
  }

  const nearest = samples
    .map((sample) => ({
      sample,
      distanceSquared:
        (sample.x - point.x) ** 2 +
        (sample.y - point.y) ** 2 +
        (sample.z - point.z) ** 2,
    }))
    .sort((left, right) => left.distanceSquared - right.distanceSquared)
    .slice(0, 4);

  if (nearest[0].distanceSquared < 1e-12) {
    return nearest[0].sample.levelDb;
  }

  let weightedLevel = 0;
  let totalWeight = 0;
  for (const entry of nearest) {
    const weight = 1 / Math.max(entry.distanceSquared, 1e-12);
    weightedLevel += entry.sample.levelDb * weight;
    totalWeight += weight;
  }

  return weightedLevel / totalWeight;
}

export function splHeatmapColor(levelDb, minimumDb, maximumDb) {
  const span = maximumDb - minimumDb;
  const normalized = span > 0 ? clamp((levelDb - minimumDb) / span, 0, 1) : 1;

  for (let index = 1; index < COLOR_STOPS.length; index += 1) {
    const upper = COLOR_STOPS[index];
    if (normalized > upper.position) {
      continue;
    }

    const lower = COLOR_STOPS[index - 1];
    const mix =
      (normalized - lower.position) / (upper.position - lower.position);
    return lower.color.map(
      (channel, channelIndex) =>
        channel + (upper.color[channelIndex] - channel) * mix,
    );
  }

  return [...COLOR_STOPS.at(-1).color];
}

function isValidSample(sample) {
  return (
    sample &&
    typeof sample.surfaceId === "string" &&
    sample.surfaceId.length > 0 &&
    [sample.x, sample.y, sample.z, sample.levelDb].every((value) =>
      Number.isFinite(Number(value)),
    )
  );
}

function clamp(value, minimum, maximum) {
  return Math.min(maximum, Math.max(minimum, value));
}
