import { clamp, cssVar } from "./app-utils.js";

export function waveformPalette(context, width) {
  const gradient = context.createLinearGradient(0, 0, width, 0);
  gradient.addColorStop(0, cssVar("--accent-2") || "#0f9d92");
  gradient.addColorStop(1, cssVar("--accent") || "#ff6b4a");
  return {
    background: cssVar("--wave-fill") || "#0d141b",
    grid: cssVar("--wave-grid") || "#f8fafc",
    gridStrong: cssVar("--wave-divider") || "#ffffff",
    empty: cssVar("--wave-empty") || "#ffffff",
    divider: cssVar("--wave-divider") || "#ffffff",
    trace: gradient,
    traceSoft:
      "rgba(255, 255, 255, 0.08)",
    fill: `rgba(255, 255, 255, 0.05)`,
    glow: cssVar("--accent-soft") || "rgba(255, 107, 74, 0.14)",
    axis: "rgba(255, 255, 255, 0.62)",
    labelBg: "rgba(10, 17, 23, 0.72)",
    labelBorder: "rgba(255, 255, 255, 0.08)",
  };
}

export function buildDrySourceBuffer(context, preset) {
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

export function encodeAudioBufferToWav(audioBuffer) {
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

export function drawWaveform(canvas, samples = null, state = {}) {
  const context = canvas.getContext("2d");
  const width = canvas.clientWidth || canvas.width || 640;
  const height = canvas.clientHeight || canvas.height || 280;
  const palette = waveformPalette(context, width);
  const dpr = Math.min(window.devicePixelRatio || 1, 2);
  canvas.width = Math.round(width * dpr);
  canvas.height = Math.round(height * dpr);
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

  const layout = getWaveformLayout(width, height);

  if (!samples || samples.length === 0) {
    drawWaveGrid(context, layout, palette);
    drawAxisLabels(context, width, height, palette, state, null, state.irView === "dB" ? "dB" : "linear", layout);
    context.fillStyle = palette.empty;
    context.font = '700 16px "Manrope"';
    context.textBaseline = "middle";
    context.fillText(
      "Render a scene to inspect the impulse response.",
      layout.left + 10,
      layout.top + layout.height / 2 - 6,
    );
    context.font = '600 12px "Manrope"';
    context.fillStyle = "rgba(255, 255, 255, 0.62)";
    context.fillText(
      "The chart will show time on the x-axis and amplitude on the y-axis.",
      layout.left + 10,
      layout.top + layout.height / 2 + 16,
    );
    return;
  }

  const downsampled = downsampleForCanvas(samples, width);
  const maxAmplitude = robustPeakAmplitude(downsampled);
  drawWaveGrid(context, layout, palette);
  const view = state.irView === "dB" ? "dB" : "linear";
  const peakIndex = findPeakIndex(downsampled);
  const peakValue = downsampled[peakIndex] ?? 0;
  const peakX = (peakIndex / Math.max(1, downsampled.length - 1)) * width;

  if (view === "dB") {
    drawDbWaveform(context, layout, palette, downsampled, maxAmplitude);
  } else {
    drawLinearWaveform(context, layout, palette, downsampled, maxAmplitude);
  }

  drawPeakMarker(context, palette, peakX, peakValue, view, maxAmplitude, layout);

  if (state.renderMode === "hybrid") {
    const x = (state.crossoverTimeSeconds / state.durationSeconds) * width;
    drawDivider(context, palette, x, layout, state.crossoverTimeSeconds);
  }

  drawAxisLabels(context, width, height, palette, state, samples, view, layout);
}

function getWaveformLayout(width, height) {
  const left = 56;
  const right = 18;
  const top = 18;
  const bottom = 38;
  return {
    left,
    right,
    top,
    bottom,
    width: Math.max(1, width - left - right),
    height: Math.max(1, height - top - bottom),
  };
}

function drawWaveGrid(context, layout, palette) {
  const { left, right, top, bottom, width, height } = layout;
  const x0 = left;
  const x1 = left + width;
  const y0 = top;
  const y1 = top + height;

  context.save();
  context.strokeStyle = palette.grid;
  context.lineWidth = 1;
  context.setLineDash([]);
  for (let index = 1; index < 5; index += 1) {
    const x = x0 + (index / 5) * width;
    context.globalAlpha = index === 2 || index === 4 ? 0.26 : 0.14;
    context.beginPath();
    context.moveTo(x, y0);
    context.lineTo(x, y1);
    context.stroke();
  }
  for (let index = 1; index < 4; index += 1) {
    const y = y0 + (index / 4) * height;
    context.globalAlpha = index === 2 ? 0.24 : 0.12;
    context.beginPath();
    context.moveTo(x0, y);
    context.lineTo(x1, y);
    context.stroke();
  }
  context.globalAlpha = 0.24;
  context.strokeStyle = "rgba(255, 255, 255, 0.16)";
  context.strokeRect(x0, y0, width, height);
  context.globalAlpha = 1;
  context.restore();
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

  return Math.max(percentile, absoluteMax * 0.2, 1e-6);
}

function drawLinearWaveform(context, layout, palette, samples, maxAmplitude) {
  const baseline = Math.round(layout.top + layout.height * 0.62);
  const upper = Math.max(layout.top + 12, baseline - 1);
  const lower = Math.min(layout.top + layout.height - 2, baseline + 1);
  const scale = Math.max(1, Math.min(baseline - upper, lower - baseline));
  const { linePath, fillPath } = buildLinearPaths(
    samples,
    layout.left,
    layout.width,
    baseline,
    scale,
    maxAmplitude,
  );

  context.save();
  context.shadowColor = palette.glow;
  context.shadowBlur = 22;
  const fill = context.createLinearGradient(0, baseline - scale, 0, layout.top + layout.height);
  fill.addColorStop(0, "rgba(255, 255, 255, 0.16)");
  fill.addColorStop(0.6, "rgba(255, 255, 255, 0.05)");
  fill.addColorStop(1, "rgba(255, 255, 255, 0)");
  context.fillStyle = fill;
  context.fill(fillPath);

  context.shadowBlur = 0;
  context.lineWidth = 2.2;
  context.strokeStyle = palette.trace;
  context.stroke(linePath);
  context.restore();
}

function drawDbWaveform(context, layout, palette, samples, maxAmplitude) {
  const top = layout.top + 6;
  const bottom = layout.top + layout.height - 4;
  const dynRange = 60;
  const { linePath, fillPath } = buildDbPaths(
    samples,
    layout.left,
    layout.width,
    top,
    bottom,
    maxAmplitude,
    dynRange,
  );
  context.save();
  context.shadowColor = palette.glow;
  context.shadowBlur = 18;
  context.lineWidth = 2;
  context.strokeStyle = palette.trace;
  const fill = context.createLinearGradient(0, top, 0, bottom);
  fill.addColorStop(0, "rgba(255, 255, 255, 0.16)");
  fill.addColorStop(1, "rgba(255, 255, 255, 0)");
  context.fillStyle = fill;
  context.fill(fillPath);

  context.shadowBlur = 0;
  context.stroke(linePath);

  drawDbGuideLines(context, layout, palette, top, bottom);
  context.restore();
}

function drawDbGuideLines(context, layout, palette, top, bottom) {
  const levels = [0, -20, -40, -60];
  context.save();
  context.strokeStyle = palette.gridStrong;
  context.fillStyle = "rgba(255, 255, 255, 0.72)";
  context.font = '600 11px "Manrope"';
  context.textBaseline = "middle";
  levels.forEach((level, index) => {
    const y = top + (index / (levels.length - 1)) * (bottom - top);
    context.globalAlpha = index === 0 ? 0.28 : 0.14;
    context.beginPath();
    context.moveTo(layout.left, y);
    context.lineTo(layout.left + layout.width, y);
    context.stroke();
    context.globalAlpha = 1;
    drawChip(context, palette, `${level} dB`, layout.left - 8, y, "right");
  });
  context.restore();
}

function drawAxisLabels(context, width, height, palette, state, samples, view, layout) {
  const durationSeconds = state.durationSeconds || 1;
  const xAxisY = height - 12;
  const labels = [
    { x: layout.left, text: "0 s", align: "left" },
    { x: layout.left + layout.width * 0.25, text: formatSeconds(durationSeconds * 0.25), align: "center" },
    { x: layout.left + layout.width * 0.5, text: formatSeconds(durationSeconds * 0.5), align: "center" },
    { x: layout.left + layout.width * 0.75, text: formatSeconds(durationSeconds * 0.75), align: "center" },
    { x: layout.left + layout.width, text: formatSeconds(durationSeconds), align: "right" },
  ];

  context.save();
  context.font = '600 11px "Manrope"';
  context.textBaseline = "middle";
  labels.forEach((label) => {
    context.fillStyle = "rgba(255, 255, 255, 0.54)";
    context.textAlign = label.align;
    context.fillText(label.text, label.x, xAxisY);
  });

  if (view === "linear") {
    const baseline = Math.round(layout.top + layout.height * 0.62);
    context.fillStyle = "rgba(255, 255, 255, 0.58)";
    context.textAlign = "right";
    context.fillText("+1", layout.left - 8, layout.top + 20);
    context.fillText("0", layout.left - 8, baseline + 4);
    context.fillText("-1", layout.left - 8, layout.top + layout.height - 2);
  } else {
    const levels = [0, -20, -40, -60];
    context.fillStyle = "rgba(255, 255, 255, 0.5)";
    context.textAlign = "right";
    levels.forEach((level, index) => {
      const y = layout.top + 6 + (index / (levels.length - 1)) * (layout.height - 10);
      context.fillText(`${level}`, layout.left - 8, y);
    });
  }

  if (!samples || samples.length === 0) {
    context.restore();
    return;
  }

  context.fillStyle = "rgba(255, 255, 255, 0.5)";
  context.textAlign = "left";
  context.fillText("amplitude", 16, layout.top - 2);
  drawChip(context, palette, "time", layout.left + layout.width, height - 30, "right");
  context.restore();
}

function drawDivider(context, palette, x, layout, timeSeconds) {
  const clampedX = clamp(x, layout.left, layout.left + layout.width);
  context.save();
  context.strokeStyle = palette.divider;
  context.fillStyle = palette.labelBg;
  context.lineWidth = 1.5;
  context.setLineDash([6, 6]);
  context.beginPath();
  context.moveTo(clampedX, layout.top);
  context.lineTo(clampedX, layout.top + layout.height);
  context.stroke();
  context.setLineDash([]);
  drawChip(context, palette, `crossover ${formatSeconds(timeSeconds)}`, clampedX, layout.top - 2, "center");
  context.restore();
}

function drawPeakMarker(context, palette, peakX, peakValue, view, maxAmplitude, layout) {
  if (!Number.isFinite(peakX) || Math.abs(peakValue) < 1e-6) {
    return;
  }

  const baseline = Math.round(layout.top + layout.height * 0.62);
  const top = layout.top + 6;
  const bottom = layout.top + layout.height - 4;
  const dynRange = 60;
  const y =
    view === "dB"
      ? (() => {
          const abs = Math.abs(peakValue);
          const dB = abs > 0 ? 20 * Math.log10(abs / maxAmplitude) : -dynRange;
          const clampedDB = Math.max(dB, -dynRange);
          const normalized = 1 + clampedDB / dynRange;
          return bottom - normalized * (bottom - top);
        })()
      : baseline - (peakValue / maxAmplitude) * Math.max(1, baseline - top);

  context.save();
  context.fillStyle = palette.trace;
  context.shadowColor = palette.glow;
  context.shadowBlur = 12;
  context.beginPath();
  context.arc(peakX, y, 3.6, 0, Math.PI * 2);
  context.fill();
  context.shadowBlur = 0;
  drawChip(context, palette, "peak", peakX, Math.max(layout.top - 2, y - 14), "center");
  context.restore();
}

function buildLinearPaths(samples, left, width, baseline, scale, maxAmplitude) {
  const linePath = new Path2D();
  const fillPath = new Path2D();
  fillPath.moveTo(left, baseline);
  samples.forEach((sample, index) => {
    const x = (index / Math.max(1, samples.length - 1)) * width;
    const y = baseline - (sample / maxAmplitude) * scale;
    if (index === 0) {
      linePath.moveTo(left + x, y);
      fillPath.lineTo(left + x, y);
    } else {
      linePath.lineTo(left + x, y);
      fillPath.lineTo(left + x, y);
    }
  });
  fillPath.lineTo(left + width, baseline);
  fillPath.closePath();
  return { linePath, fillPath };
}

function buildDbPaths(samples, left, width, top, bottom, maxAmplitude, dynRange) {
  const linePath = new Path2D();
  const fillPath = new Path2D();
  fillPath.moveTo(left, bottom);
  samples.forEach((sample, index) => {
    const x = (index / Math.max(1, samples.length - 1)) * width;
    const abs = Math.abs(sample);
    const dB = abs > 0 ? 20 * Math.log10(abs / maxAmplitude) : -dynRange;
    const clampedDB = Math.max(dB, -dynRange);
    const normalized = 1 + clampedDB / dynRange;
    const y = bottom - normalized * (bottom - top);
    if (index === 0) {
      linePath.moveTo(left + x, y);
      fillPath.lineTo(left + x, y);
    } else {
      linePath.lineTo(left + x, y);
      fillPath.lineTo(left + x, y);
    }
  });
  fillPath.lineTo(left + width, bottom);
  fillPath.closePath();
  return { linePath, fillPath };
}

function drawChip(context, palette, text, x, y, align = "center") {
  context.save();
  context.font = '700 11px "Manrope"';
  context.textBaseline = "middle";
  context.textAlign = align;
  const paddingX = 8;
  const metrics = context.measureText(text);
  const textWidth = metrics.width;
  const boxWidth = textWidth + paddingX * 2;
  const boxHeight = 18;
  const left =
    align === "center" ? x - boxWidth / 2 : align === "right" ? x - boxWidth : x;
  const top = y - boxHeight / 2;
  context.fillStyle = palette.labelBg;
  context.strokeStyle = palette.labelBorder;
  context.lineWidth = 1;
  roundedRect(context, left, top, boxWidth, boxHeight, 999);
  context.fill();
  context.stroke();
  context.fillStyle = "rgba(255, 255, 255, 0.84)";
  context.fillText(text, align === "center" ? x : align === "right" ? x - paddingX : x + paddingX, y + 0.5);
  context.restore();
}

function roundedRect(context, x, y, width, height, radius) {
  const r = Math.min(radius, width / 2, height / 2);
  context.beginPath();
  context.moveTo(x + r, y);
  context.arcTo(x + width, y, x + width, y + height, r);
  context.arcTo(x + width, y + height, x, y + height, r);
  context.arcTo(x, y + height, x, y, r);
  context.arcTo(x, y, x + width, y, r);
  context.closePath();
}

function findPeakIndex(samples) {
  let peakIndex = 0;
  let peakMagnitude = -1;
  samples.forEach((sample, index) => {
    const magnitude = Math.abs(sample);
    if (magnitude > peakMagnitude) {
      peakMagnitude = magnitude;
      peakIndex = index;
    }
  });
  return peakIndex;
}

function formatSeconds(seconds) {
  if (!Number.isFinite(seconds)) {
    return "0.0 s";
  }
  if (seconds < 1) {
    return `${Math.round(seconds * 1000)} ms`;
  }
  return `${seconds.toFixed(seconds >= 10 ? 0 : 1)} s`;
}
