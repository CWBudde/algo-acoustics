import { clamp, cssVar } from "./app-utils.js";

export function waveformPalette(context, width) {
  const gradient = context.createLinearGradient(0, 0, width, 0);
  gradient.addColorStop(0, cssVar("--accent-2") || "#0f9d92");
  gradient.addColorStop(1, cssVar("--accent") || "#ff6b4a");
  return {
    background: cssVar("--wave-fill") || "#0d141b",
    grid: cssVar("--wave-grid") || "#f8fafc",
    empty: cssVar("--wave-empty") || "#ffffff",
    divider: cssVar("--wave-divider") || "#ffffff",
    trace: gradient,
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
  const maxAmplitude = robustPeakAmplitude(downsampled);
  const pad = 12;

  if (state.irView === "dB") {
    const dynRange = 60;

    context.strokeStyle = palette.trace;
    context.lineWidth = 1.5;
    context.beginPath();
    downsampled.forEach((sample, index) => {
      const x = (index / Math.max(1, downsampled.length - 1)) * width;
      const abs = Math.abs(sample);
      const dB = abs > 0 ? 20 * Math.log10(abs / maxAmplitude) : -dynRange;
      const clampedDB = Math.max(dB, -dynRange);
      const normalized = 1 + clampedDB / dynRange;
      const y = pad + (1 - normalized) * (height - 2 * pad);
      if (index === 0) {
        context.moveTo(x, y);
      } else {
        context.lineTo(x, y);
      }
    });
    context.stroke();
  } else {
    const mid = height / 2;

    context.strokeStyle = palette.grid;
    context.lineWidth = 1.5;
    context.beginPath();
    context.moveTo(0, mid);
    context.lineTo(width, mid);
    context.stroke();

    context.strokeStyle = palette.trace;
    context.lineWidth = 1.5;
    context.beginPath();
    downsampled.forEach((sample, index) => {
      const x = (index / Math.max(1, downsampled.length - 1)) * width;
      const y = mid - (sample / maxAmplitude) * (mid - pad);
      if (index === 0) {
        context.moveTo(x, y);
      } else {
        context.lineTo(x, y);
      }
    });
    context.stroke();
  }

  if (state.renderMode === "hybrid") {
    const x = (state.crossoverTimeSeconds / state.durationSeconds) * width;
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
